package builtins

import (
	"context"

	"github.com/ricardo/anthrogo/pkg/command"
)

type Compact struct{}

func (Compact) Name() string        { return "/compact" }
func (Compact) Aliases() []string   { return nil }
func (Compact) Description() string { return "Compact prior turns (deferred to M3)" }
func (Compact) Type() command.Type  { return command.TypeLocal }

func (Compact) Run(_ context.Context, _ string, host command.Host) (command.Result, error) {
	host.AppendUIMessage("compaction is deferred to M3 — no-op for now")
	return command.Result{Text: "compact: deferred to M3"}, nil
}
