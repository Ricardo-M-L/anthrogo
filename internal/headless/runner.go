package headless

import (
	"context"
	"fmt"
	"io"

	"github.com/ricardo/anthrogo/internal/session"
	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/query"
	"github.com/ricardo/anthrogo/pkg/subagent"
	"github.com/ricardo/anthrogo/pkg/tool"
)

// PromptHookSink is the subset of hooks.Manager that headless.Run needs.
// Declared locally to keep import graph flat (no import of internal/hooks).
type PromptHookSink interface {
	FireUserPromptSubmit(ctx context.Context, prompt string) (context.Context, string, bool, string)
	FireSessionStart(ctx context.Context, kind string)
	FireSessionEnd(ctx context.Context, kind string)
	FireNotification(ctx context.Context, message, kind string)
	FirePostToolUse(ctx context.Context, toolName string, input, response map[string]any) string
	FireStop(ctx context.Context, reason string)
	FirePreCompact(ctx context.Context, trigger string)
	FireSubagentStop(ctx context.Context, reason string)
}

// Options bundles everything Run needs. The CLI assembles this from cobra flags
// + config.
type Options struct {
	Prompt          string
	Model           string
	SystemPrompt    string
	Cwd             string
	Provider        provider.Provider
	Tools           *tool.Registry
	Permissions     *permissions.Context
	InitialMessages []message.Message
	RecordHook      func(session.Record)
	Stdout          io.Writer
	Stderr          io.Writer
	Hooks           PromptHookSink
	Subagents       *subagent.Registry
	// OnEngineReady, if non-nil, is called with the engine before the first
	// SubmitMessage. Callers use this to wire deferred runners (e.g. Task tool).
	OnEngineReady func(*query.Engine)
}

// Run executes one prompt and writes the assistant's final text to Stdout.
// Tool denials in headless mode flow through naturally because Permissions
// is configured with ShouldAvoidPrompts=true at the CLI level (Task 26).
func Run(ctx context.Context, opts Options) error {
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}

	if opts.Hooks != nil {
		opts.Hooks.FireSessionStart(ctx, "new")
		defer opts.Hooks.FireSessionEnd(ctx, "user_quit")
	}

	// Run UserPromptSubmit hooks.
	prompt := opts.Prompt
	if opts.Hooks != nil {
		_, finalPrompt, abort, reason := opts.Hooks.FireUserPromptSubmit(ctx, prompt)
		if abort {
			return fmt.Errorf("prompt blocked by hook: %s", reason)
		}
		prompt = finalPrompt
	}

	e := query.NewEngine(query.Config{
		Provider:         opts.Provider,
		Tools:            opts.Tools,
		Permissions:      opts.Permissions,
		Model:            opts.Model,
		SystemPrompt:     opts.SystemPrompt,
		Cwd:              opts.Cwd,
		RecordHook:       opts.RecordHook,
		Hooks:            opts.Hooks,
		SubagentRegistry: opts.Subagents,
	})
	if opts.OnEngineReady != nil {
		opts.OnEngineReady(e)
	}
	if len(opts.InitialMessages) > 0 {
		e.SetInitialMessages(opts.InitialMessages)
	}
	ch := e.SubmitMessage(ctx, prompt)
	for ev := range ch {
		switch ev.Kind {
		case query.KindAssistantDelta:
			fmt.Fprint(opts.Stdout, ev.Text)
		case query.KindAssistantStop:
			fmt.Fprintln(opts.Stdout)
		case query.KindToolUseRequest:
			fmt.Fprintf(opts.Stderr, "[tool] %s %v\n", ev.ToolName, ev.ToolInput)
		case query.KindToolResult:
			if ev.IsError {
				fmt.Fprintf(opts.Stderr, "[tool error] %s: %s\n", ev.ToolName, ev.Text)
			}
		case query.KindError:
			return ev.Err
		case query.KindTurnComplete:
			return nil
		}
	}
	return nil
}
