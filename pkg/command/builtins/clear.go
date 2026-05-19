package builtins

import (
	"context"

	"github.com/ricardo/anthrogo/pkg/command"
)

type Clear struct{}

func (Clear) Name() string        { return "/clear" }
func (Clear) Aliases() []string   { return nil }
func (Clear) Description() string { return "Start a fresh session (writes a new JSONL file)" }
func (Clear) Type() command.Type  { return command.TypeLocal }

func (Clear) Run(_ context.Context, _ string, host command.Host) (command.Result, error) {
	if err := host.ResetSession(); err != nil {
		return command.Result{}, err
	}
	host.ReplaceMessages(nil)
	return command.Result{Text: "session cleared"}, nil
}
