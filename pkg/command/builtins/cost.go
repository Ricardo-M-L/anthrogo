package builtins

import (
	"context"
	"fmt"
	"strings"

	"github.com/ricardo/anthrogo/pkg/command"
)

// Cost implements the /cost builtin command.
type Cost struct{}

func (Cost) Name() string        { return "/cost" }
func (Cost) Aliases() []string   { return nil }
func (Cost) Description() string { return "Show estimated USD cost of the session" }
func (Cost) Type() command.Type  { return command.TypeLocal }

func (Cost) Run(ctx context.Context, args string, host command.Host) (command.Result, error) {
	eng := host.Engine()
	if eng == nil {
		return command.Result{Text: "no active engine"}, nil
	}
	usd, ok := eng.EstimatedCost()
	if !ok {
		return command.Result{Text: "no pricing configured for current model; add a `pricing:` stanza to settings.yaml to enable cost tracking"}, nil
	}
	u := eng.Usage()
	var b strings.Builder
	fmt.Fprintf(&b, "Session usage: %d input + %d output = %d total tokens\n",
		u.InputTokens, u.OutputTokens, u.InputTokens+u.OutputTokens)
	fmt.Fprintf(&b, "Estimated cost: $%.4f USD\n", usd)
	return command.Result{Text: b.String()}, nil
}
