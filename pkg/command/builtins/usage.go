package builtins

import (
	"context"
	"fmt"
	"strings"

	"github.com/ricardo/anthrogo/pkg/command"
)

type Usage struct{}

func (Usage) Name() string        { return "/usage" }
func (Usage) Aliases() []string   { return nil }
func (Usage) Description() string { return "Show current token usage + auto-compact threshold" }
func (Usage) Type() command.Type  { return command.TypeLocal }

func (Usage) Run(ctx context.Context, args string, host command.Host) (command.Result, error) {
	eng := host.Engine()
	if eng == nil {
		return command.Result{Text: "no active engine"}, nil
	}
	cumulative := eng.Usage()
	sinceCompact := eng.UsageSinceLastCompact()
	var b strings.Builder
	fmt.Fprintf(&b, "Session totals: %d input + %d output = %d tokens\n",
		cumulative.InputTokens, cumulative.OutputTokens,
		cumulative.InputTokens+cumulative.OutputTokens)
	fmt.Fprintf(&b, "Since last compact: %d input + %d output = %d tokens\n",
		sinceCompact.InputTokens, sinceCompact.OutputTokens,
		sinceCompact.InputTokens+sinceCompact.OutputTokens)
	threshold, keep := eng.AutoCompactConfig()
	if threshold > 0 {
		used := sinceCompact.InputTokens + sinceCompact.OutputTokens
		remaining := threshold - used
		if remaining < 0 {
			remaining = 0
		}
		fmt.Fprintf(&b, "Auto-compact at: %d tokens (keep recent: %d) — %d tokens until trigger\n",
			threshold, keep, remaining)
	} else {
		b.WriteString("Auto-compact: disabled (set --auto-compact <N> or auto_compact_threshold in YAML)\n")
	}
	return command.Result{Text: b.String()}, nil
}
