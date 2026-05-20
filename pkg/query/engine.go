package query

import (
	"context"
	"sync"

	"github.com/ricardo/anthrogo/internal/session"
	"github.com/ricardo/anthrogo/pkg/compact"
	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/tool"
)

// HookSink is the subset of hooks.Manager that the query engine needs.
// All methods are nil-safe when the field is nil.
type HookSink interface {
	FirePostToolUse(ctx context.Context, toolName string, input, response map[string]any) string
	FireStop(ctx context.Context, reason string)
	FirePreCompact(ctx context.Context, trigger string)
}

type Config struct {
	Provider     provider.Provider
	Tools        *tool.Registry
	Permissions  *permissions.Context
	Model        string
	SystemPrompt string
	MaxTokens    int
	Temperature  float64
	Cwd          string
	MaxTurns     int // 0 = unlimited

	// Surface hooks; nil-safe.
	RequestPrompt   func(source string, req tool.PromptRequest) (tool.PromptResponse, error)
	AppendUIMessage func(msg string)
	RecordHook      func(session.Record)

	// Hooks is an optional hook sink for PostToolUse and Stop events.
	Hooks HookSink
}

// Engine owns one conversation. Each SubmitMessage starts a new turn within
// the same conversation; messages, usage, cwd persist across turns.
type Engine struct {
	mu       sync.Mutex
	cfg      Config
	messages []message.Message
	usage    message.Usage
	denials  []PermissionDenial
}

func NewEngine(cfg Config) *Engine {
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 4096
	}
	return &Engine{cfg: cfg}
}

func (e *Engine) Messages() []message.Message {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]message.Message, len(e.messages))
	copy(out, e.messages)
	return out
}

func (e *Engine) SetInitialMessages(msgs []message.Message) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if msgs == nil {
		e.messages = nil
		return
	}
	e.messages = append([]message.Message(nil), msgs...)
}

func (e *Engine) Usage() message.Usage {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.usage
}

// CompactOptions controls Engine.Compact behaviour.
type CompactOptions struct {
	KeepRecent int    // default 10
	Trigger    string // "manual" | "auto"
}

// Summary is the result of Engine.Compact.
type Summary struct {
	OriginalCount int
	NewCount      int
	OriginalBytes int
	NewBytes      int
	SummaryText   string
	Skipped       bool
	SkipReason    string
	Trigger       string
}

// Compact summarizes earlier conversation turns to reduce context size.
// It fires the PreCompact hook, calls compact.Run, then swaps the engine's
// message list and emits a compact record via RecordHook.
//
// The lock is held only to take a snapshot before the (slow) LLM call and to
// install the result after; the LLM call itself runs without the lock so
// concurrent reads of Messages() or SubmitMessage turns are not blocked for
// the entire summarisation duration.
func (e *Engine) Compact(ctx context.Context, opts CompactOptions) (Summary, error) {
	if opts.Trigger == "" {
		opts.Trigger = "manual"
	}
	if e.cfg.Hooks != nil {
		e.cfg.Hooks.FirePreCompact(ctx, opts.Trigger)
	}

	// Snapshot current messages under lock; release before the LLM call.
	e.mu.Lock()
	msgs := append([]message.Message(nil), e.messages...)
	e.mu.Unlock()

	in := compact.Input{
		Provider:   e.cfg.Provider,
		Model:      e.cfg.Model,
		Messages:   msgs,
		KeepRecent: opts.KeepRecent,
	}
	out, err := compact.Run(ctx, in)
	if err != nil {
		return Summary{}, err
	}
	s := Summary{
		OriginalCount: out.OriginalCount,
		NewCount:      out.NewCount,
		OriginalBytes: out.OriginalBytes,
		NewBytes:      out.NewBytes,
		SummaryText:   out.SummaryText,
		Skipped:       out.Skipped,
		SkipReason:    out.SkipReason,
		Trigger:       opts.Trigger,
	}
	if !out.Skipped {
		// Install compacted messages directly under lock to avoid a double-copy
		// that SetInitialMessages would introduce.
		e.mu.Lock()
		e.messages = out.NewMessages
		e.mu.Unlock()

		if e.cfg.RecordHook != nil {
			e.cfg.RecordHook(session.Record{
				Kind: session.KindCompact,
				Compact: &session.CompactRecord{
					OriginalCount: s.OriginalCount,
					NewCount:      s.NewCount,
					OriginalBytes: s.OriginalBytes,
					NewBytes:      s.NewBytes,
					Trigger:       s.Trigger,
				},
			})
		}
	}
	return s, nil
}
