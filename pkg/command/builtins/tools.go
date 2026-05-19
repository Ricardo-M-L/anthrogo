package builtins

import (
	"context"
	"fmt"
	"strings"

	"github.com/ricardo/anthrogo/pkg/command"
)

type Tools struct{}

func (Tools) Name() string        { return "/tools" }
func (Tools) Aliases() []string   { return nil }
func (Tools) Description() string { return "List registered tools" }
func (Tools) Type() command.Type  { return command.TypeLocal }

func (Tools) Run(ctx context.Context, _ string, host command.Host) (command.Result, error) {
	var b strings.Builder
	b.WriteString("Tools:\n")
	for _, t := range host.Tools().All() {
		fmt.Fprintf(&b, "  %s  — %s\n", t.Name(), t.Description(ctx))
	}
	return command.Result{Text: b.String()}, nil
}
