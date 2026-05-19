package builtins

import (
	"context"

	"github.com/ricardo/anthrogo/pkg/command"
)

type Memory struct{}

func (Memory) Name() string        { return "/memory" }
func (Memory) Aliases() []string   { return nil }
func (Memory) Description() string { return "Print resolved CLAUDE.md content" }
func (Memory) Type() command.Type  { return command.TypeLocal }

func (Memory) Run(_ context.Context, _ string, host command.Host) (command.Result, error) {
	md := host.ClaudeMd()
	if md == "" {
		return command.Result{Text: "(no CLAUDE.md found in cwd or parents)"}, nil
	}
	return command.Result{Text: md}, nil
}
