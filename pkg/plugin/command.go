package plugin

import (
	"context"
	"strings"

	"github.com/ricardo/anthrogo/pkg/command"
)

// DynamicCommand implements command.Command from a manifest CommandSpec.
type DynamicCommand struct {
	spec CommandSpec
}

func (d DynamicCommand) Name() string { return d.spec.Name }
func (d DynamicCommand) Aliases() []string {
	if len(d.spec.Aliases) == 0 {
		return nil
	}
	return d.spec.Aliases
}
func (d DynamicCommand) Description() string { return d.spec.Description + " (plugin)" }
func (d DynamicCommand) Type() command.Type  { return command.Type(d.spec.Type) }

func (d DynamicCommand) Run(_ context.Context, args string, _ command.Host) (command.Result, error) {
	body := d.spec.Body
	if trimmed := strings.TrimSpace(args); trimmed != "" {
		body += "\n\n" + trimmed
	}
	switch command.Type(d.spec.Type) {
	case command.TypeLocalPrompt, command.TypeSubmit:
		return command.Result{SubmitText: body}, nil
	case command.TypeLocal:
		return command.Result{Text: body}, nil
	default:
		return command.Result{Text: "plugin command has unknown type: " + d.spec.Type}, nil
	}
}
