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
	"github.com/ricardo/anthrogo/pkg/tool"
)

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
}

// Run executes one prompt and writes the assistant's final text to Stdout.
// Tool denials in headless mode flow through naturally because Permissions
// is configured with ShouldAvoidPrompts=true at the CLI level (Task 26).
func Run(ctx context.Context, opts Options) error {
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}
	e := query.NewEngine(query.Config{
		Provider:     opts.Provider,
		Tools:        opts.Tools,
		Permissions:  opts.Permissions,
		Model:        opts.Model,
		SystemPrompt: opts.SystemPrompt,
		Cwd:          opts.Cwd,
		RecordHook:   opts.RecordHook,
	})
	if len(opts.InitialMessages) > 0 {
		e.SetInitialMessages(opts.InitialMessages)
	}
	ch := e.SubmitMessage(ctx, opts.Prompt)
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
