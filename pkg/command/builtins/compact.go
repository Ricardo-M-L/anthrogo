package builtins

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ricardo/anthrogo/pkg/command"
	"github.com/ricardo/anthrogo/pkg/query"
)

type Compact struct{}

func (Compact) Name() string        { return "/compact" }
func (Compact) Aliases() []string   { return nil }
func (Compact) Description() string {
	return "Summarize earlier conversation to reduce context. Use '--reset-budget' to also zero the cost counter."
}
func (Compact) Type() command.Type  { return command.TypeLocal }

func (Compact) Run(ctx context.Context, args string, host command.Host) (command.Result, error) {
	opts := query.CompactOptions{Trigger: "manual"}
	tokens := strings.Fields(args)
	resetBudget := false
	filtered := tokens[:0]
	for _, tok := range tokens {
		if tok == "--reset-budget" {
			resetBudget = true
		} else {
			filtered = append(filtered, tok)
		}
	}
	tokens = filtered
	for i, tok := range tokens {
		if (tok == "--keep" || tok == "-k") && i+1 < len(tokens) {
			if n, err := strconv.Atoi(tokens[i+1]); err == nil && n > 0 {
				opts.KeepRecent = n
			}
		} else if strings.HasPrefix(tok, "--keep=") {
			if n, err := strconv.Atoi(strings.TrimPrefix(tok, "--keep=")); err == nil && n > 0 {
				opts.KeepRecent = n
			}
		} else if strings.HasPrefix(tok, "-k=") {
			if n, err := strconv.Atoi(strings.TrimPrefix(tok, "-k=")); err == nil && n > 0 {
				opts.KeepRecent = n
			}
		}
	}
	eng := host.Engine()
	if eng == nil {
		return command.Result{Text: "compact: no active engine"}, nil
	}
	s, err := eng.Compact(ctx, opts)
	if err != nil {
		return command.Result{Text: fmt.Sprintf("compact failed: %v", err)}, nil
	}
	if s.Skipped {
		return command.Result{Text: "compact: " + s.SkipReason}, nil
	}
	msg := fmt.Sprintf(
		"compacted %d → %d messages (~%d → ~%d bytes)",
		s.OriginalCount, s.NewCount, s.OriginalBytes, s.NewBytes,
	)
	if resetBudget {
		eng.ResetUsage()
		msg += "\nsession usage counter reset to zero."
	}
	host.AppendUIMessage(msg)
	return command.Result{Text: msg}, nil
}
