package builtins

import (
	"context"
	"fmt"
	"strings"

	"github.com/ricardo/anthrogo/pkg/command"
)

type Help struct {
	Reg *command.Registry
}

func (Help) Name() string        { return "/help" }
func (Help) Aliases() []string   { return []string{"/?"} }
func (Help) Description() string { return "List all slash commands" }
func (Help) Type() command.Type  { return command.TypeLocal }

func (h *Help) Run(_ context.Context, _ string, host command.Host) (command.Result, error) {
	reg := h.Reg
	if reg == nil {
		reg = host.Registry()
	}
	var b strings.Builder
	b.WriteString("Slash commands:\n")
	for _, c := range reg.All() {
		fmt.Fprintf(&b, "  %s  — %s\n", c.Name(), c.Description())
	}
	return command.Result{Text: b.String()}, nil
}
