package headless

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/ricardo/anthrogo/internal/session"
	"github.com/ricardo/anthrogo/pkg/command"
	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/pricing"
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
	Session         *session.Store
	Stdout          io.Writer
	Stderr          io.Writer
	Hooks           PromptHookSink
	Subagents       *subagent.Registry
	// OnEngineReady, if non-nil, is called with the engine before the first
	// SubmitMessage. Callers use this to wire deferred runners (e.g. Task tool).
	OnEngineReady func(*query.Engine)

	// AutoCompactThreshold and AutoCompactKeepRecent are forwarded to the engine.
	AutoCompactThreshold  int
	AutoCompactKeepRecent int

	// Pricing is the optional pricing table for cost tracking. nil = disabled.
	Pricing *pricing.Table

	// CostLimitUSD, when > 0, enables hard budget enforcement via IsOverBudget().
	CostLimitUSD float64
}

// RunExecRequest executes a command.ExecRequest in headless mode: the subprocess
// inherits stdio so an editor (or any interactive program) can paint the terminal
// directly. OnComplete is called after the process exits, and the returned status
// string (if any) is written to stdout.
//
// Callers that dispatch slash commands in headless mode should call this after
// checking result.ExecCmd != nil.
func RunExecRequest(req *command.ExecRequest, stdout io.Writer) {
	if req == nil || req.Cmd == nil {
		return
	}
	req.Cmd.Stdin = os.Stdin
	req.Cmd.Stdout = os.Stdout
	req.Cmd.Stderr = os.Stderr
	err := req.Cmd.Run()
	if req.OnComplete != nil {
		if msg := req.OnComplete(err); msg != "" {
			fmt.Fprintln(stdout, msg)
		}
	}
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
		Provider:              opts.Provider,
		Tools:                 opts.Tools,
		Permissions:           opts.Permissions,
		Model:                 opts.Model,
		SystemPrompt:          opts.SystemPrompt,
		Cwd:                   opts.Cwd,
		RecordHook:            opts.RecordHook,
		Session:               opts.Session,
		Hooks:                 opts.Hooks,
		SubagentRegistry:      opts.Subagents,
		AutoCompactThreshold:  opts.AutoCompactThreshold,
		AutoCompactKeepRecent: opts.AutoCompactKeepRecent,
		Pricing:               opts.Pricing,
		CostLimitUSD:          opts.CostLimitUSD,
	})
	if opts.OnEngineReady != nil {
		opts.OnEngineReady(e)
	}
	if len(opts.InitialMessages) > 0 {
		e.SetInitialMessages(opts.InitialMessages)
	}
	blocks, parseErr := message.ParseUserPrompt(prompt)
	if parseErr != nil {
		return fmt.Errorf("prompt parse: %w", parseErr)
	}
	if len(blocks) == 0 {
		blocks = []message.Block{{Type: message.BlockText, Text: prompt}}
	}
	ch := e.SubmitMessageBlocks(ctx, blocks)
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
