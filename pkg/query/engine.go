package query

import (
	"context"

	"github.com/ricardo/anthrogo/internal/session"
	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/tool"
)

// HookSink is the subset of hooks.Manager that the query engine needs.
// Both methods are nil-safe when the field is nil.
type HookSink interface {
	FirePostToolUse(ctx context.Context, toolName string, input, response map[string]any) string
	FireStop(ctx context.Context, reason string)
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
	out := make([]message.Message, len(e.messages))
	copy(out, e.messages)
	return out
}

func (e *Engine) SetInitialMessages(msgs []message.Message) {
	if msgs == nil {
		e.messages = nil
		return
	}
	e.messages = append([]message.Message(nil), msgs...)
}

func (e *Engine) Usage() message.Usage { return e.usage }
