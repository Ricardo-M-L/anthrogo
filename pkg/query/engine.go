package query

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/ricardo/anthrogo/internal/session"
	"github.com/ricardo/anthrogo/pkg/compact"
	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/subagent"
	"github.com/ricardo/anthrogo/pkg/tool"
)

// HookSink is the subset of hooks.Manager that the query engine needs.
// All methods are nil-safe when the field is nil.
type HookSink interface {
	FirePostToolUse(ctx context.Context, toolName string, input, response map[string]any) string
	FireStop(ctx context.Context, reason string)
	FirePreCompact(ctx context.Context, trigger string)
	FireSubagentStop(ctx context.Context, reason string)
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

	// Hooks is an optional hook sink for PostToolUse, Stop, and SubagentStop events.
	Hooks HookSink

	// Subagent configuration.
	SubagentRegistry *subagent.Registry
	SubagentDepth    int // set by parent engine when constructing child; 0 at top level
	MaxSubagentDepth int // default 3 if zero
}

// Engine owns one conversation. Each SubmitMessage starts a new turn within
// the same conversation; messages, usage, cwd persist across turns.
type Engine struct {
	mu            sync.Mutex
	cfg           Config
	messages      []message.Message
	usage         message.Usage
	denials       []PermissionDenial
	subagentDepth int
}

func NewEngine(cfg Config) *Engine {
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 4096
	}
	return &Engine{cfg: cfg, subagentDepth: cfg.SubagentDepth}
}

// SubagentOptions carries the parameters for running a subagent.
type SubagentOptions struct {
	Type        string
	Description string
	Prompt      string
}

// RunSubagent spawns a child Engine for the named subagent type, runs one turn
// with opts.Prompt, drains the stream collecting the last assistant turn's text,
// fires the SubagentStop hook, and returns the result.
func (e *Engine) RunSubagent(ctx context.Context, opts SubagentOptions) (string, error) {
	// 1. Depth check + increment under lock.
	e.mu.Lock()
	maxDepth := e.cfg.MaxSubagentDepth
	if maxDepth <= 0 {
		maxDepth = 3
	}
	if e.subagentDepth >= maxDepth {
		e.mu.Unlock()
		return "", fmt.Errorf("subagent depth limit (%d) exceeded", maxDepth)
	}
	e.subagentDepth++
	currentDepth := e.subagentDepth
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.subagentDepth--
		e.mu.Unlock()
	}()

	// 2. Registry lookup.
	if e.cfg.SubagentRegistry == nil {
		return "", fmt.Errorf("subagent: no registry configured")
	}
	spec, ok := e.cfg.SubagentRegistry.Get(opts.Type)
	if !ok {
		return "", fmt.Errorf("subagent: unknown type %q", opts.Type)
	}

	// 3. Build filtered tool registry (or share parent's).
	var childTools *tool.Registry
	if len(spec.ToolAllowlist) > 0 {
		childTools = tool.NewRegistry()
		allow := make(map[string]bool, len(spec.ToolAllowlist))
		for _, n := range spec.ToolAllowlist {
			allow[n] = true
		}
		for _, t := range e.cfg.Tools.All() {
			if allow[t.Name()] {
				childTools.Register(t)
			}
		}
	} else {
		// Share parent's tool registry. Safe today because tool.Registry is
		// frozen at startup (cmd/anthrogo populates it once before Run). If a
		// future change makes the registry hot-mutable, defensive copy here.
		childTools = e.cfg.Tools
	}

	// 4. Build child Config.
	// Clone the permissions context so subagent Mode toggles don't affect parent.
	childPerms := e.cfg.Permissions.Clone()
	childCfg := Config{
		Provider:         e.cfg.Provider,
		Model:            e.cfg.Model,
		Tools:            childTools,
		Permissions:      childPerms,
		SystemPrompt:     e.cfg.SystemPrompt + spec.SystemPromptSuffix,
		Hooks:            e.cfg.Hooks,
		Cwd:              e.cfg.Cwd,
		RecordHook:       nil, // child messages do not pollute parent JSONL
		SubagentRegistry: e.cfg.SubagentRegistry,
		SubagentDepth:    currentDepth,
		MaxSubagentDepth: maxDepth,
		MaxTokens:        e.cfg.MaxTokens,
		Temperature:      e.cfg.Temperature,
		MaxTurns:         e.cfg.MaxTurns,
	}

	child := NewEngine(childCfg)

	// 5. Run one turn, drain the channel collecting the LAST assistant turn's text.
	// We reset the buffer on every KindAssistantStop so we end up with only the
	// final assistant turn (the child may cycle through tool_use sub-turns).
	ch := child.SubmitMessage(ctx, opts.Prompt)
	var buf strings.Builder
	for ev := range ch {
		switch ev.Kind {
		case KindAssistantDelta:
			buf.WriteString(ev.Text)
		case KindAssistantStop:
			// KindAssistantStop fires after every assistant message (including
			// intermediate ones before tool_use cycles). Reset to keep only the
			// final assistant message text.
			if ev.StopReason == "tool_use" {
				// intermediate turn — clear for next accumulation
				buf.Reset()
			}
			// if end_turn / other, keep (will be returned)
		case KindError:
			if e.cfg.Hooks != nil {
				e.cfg.Hooks.FireSubagentStop(ctx, "error")
			}
			return "", ev.Err
		}
	}

	// 6. Fire SubagentStop with success.
	if e.cfg.Hooks != nil {
		e.cfg.Hooks.FireSubagentStop(ctx, "end_turn")
	}

	return strings.TrimSpace(buf.String()), nil
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
