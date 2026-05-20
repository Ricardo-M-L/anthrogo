package builtins

import (
	"context"
	"fmt"
	"strings"

	"github.com/ricardo/anthrogo/internal/tui"
	"github.com/ricardo/anthrogo/pkg/command"
)

// Theme implements the /theme builtin command.
type Theme struct{}

func (Theme) Name() string        { return "/theme" }
func (Theme) Aliases() []string   { return nil }
func (Theme) Description() string { return "Manage TUI theme (subcommands: list, show, set <name>)" }
func (Theme) Type() command.Type  { return command.TypeLocal }

func (Theme) Run(_ context.Context, args string, host command.Host) (command.Result, error) {
	args = strings.TrimSpace(args)
	switch {
	case args == "" || args == "list":
		var b strings.Builder
		b.WriteString("Available themes:\n")
		for _, n := range tui.ThemeNames() {
			fmt.Fprintf(&b, "  %s\n", n)
		}
		b.WriteString("\nApply with: /theme set <name>. Custom themes via YAML theme.name=custom + per-field hex colors.\n")
		return command.Result{Text: b.String()}, nil

	case args == "show":
		return command.Result{Text: "current theme: " + currentThemeName(host)}, nil

	case strings.HasPrefix(args, "set "):
		name := strings.TrimSpace(strings.TrimPrefix(args, "set "))
		_, ok := tui.ThemeByName(name)
		if !ok {
			return command.Result{Text: "unknown theme: " + name + " (use /theme list)"}, nil
		}
		if setter, ok := host.(interface{ SetTheme(name string) error }); ok {
			if err := setter.SetTheme(name); err != nil {
				return command.Result{Text: "theme set failed: " + err.Error()}, nil
			}
			return command.Result{Text: "theme set to " + name + " (re-renders on next event)"}, nil
		}
		return command.Result{Text: "theme set unsupported by current surface"}, nil

	default:
		return command.Result{Text: "usage: /theme [list | show | set <name>]"}, nil
	}
}

func currentThemeName(host command.Host) string {
	if g, ok := host.(interface{ ThemeName() string }); ok {
		return g.ThemeName()
	}
	return "(unknown)"
}
