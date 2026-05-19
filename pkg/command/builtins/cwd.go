package builtins

import (
	"context"

	"github.com/ricardo/anthrogo/pkg/command"
)

type Cwd struct{}

func (Cwd) Name() string        { return "/cwd" }
func (Cwd) Aliases() []string   { return nil }
func (Cwd) Description() string { return "Print current working directory" }
func (Cwd) Type() command.Type  { return command.TypeLocal }

func (Cwd) Run(_ context.Context, _ string, host command.Host) (command.Result, error) {
	return command.Result{Text: "cwd: " + host.Cwd()}, nil
}
