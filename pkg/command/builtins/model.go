package builtins

import (
	"context"
	"strings"

	"github.com/ricardo/anthrogo/pkg/command"
)

type Model struct {
	Available []string
}

func (Model) Name() string        { return "/model" }
func (Model) Aliases() []string   { return nil }
func (Model) Description() string { return "Switch active model (or list available)" }
func (Model) Type() command.Type  { return command.TypeLocal }

func (m Model) Run(_ context.Context, args string, host command.Host) (command.Result, error) {
	args = strings.TrimSpace(args)
	if args == "" {
		if len(m.Available) == 0 {
			return command.Result{Text: "available models:\n  claude-sonnet-4-6\n  claude-opus-4-7\n  claude-haiku-4-5-20251001"}, nil
		}
		return command.Result{Text: "available models:\n  " + strings.Join(m.Available, "\n  ")}, nil
	}
	host.AppendUIMessage("switching to model: " + args + " (effective on next turn)")
	return command.Result{Text: "model: " + args}, nil
}
