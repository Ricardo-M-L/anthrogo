package builtins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ricardo/anthrogo/internal/config"
	"github.com/ricardo/anthrogo/pkg/command"
)

// System implements /system [show | edit | reset].
type System struct{}

func (System) Name() string        { return "/system" }
func (System) Aliases() []string   { return nil }
func (System) Description() string { return "Inspect or edit the user system prompt overlay (subcommands: show, edit, reset)" }
func (System) Type() command.Type  { return command.TypeLocal }

func (System) Run(_ context.Context, args string, host command.Host) (command.Result, error) {
	args = strings.TrimSpace(args)
	home := os.Getenv("HOME")
	overlayPath := config.SystemOverlayPath(home)
	switch args {
	case "", "show":
		return showSystem(host, overlayPath)
	case "edit":
		return editSystem(overlayPath)
	case "reset":
		return resetSystem(overlayPath)
	default:
		return command.Result{Text: "usage: /system [show | edit | reset]"}, nil
	}
}

func showSystem(host command.Host, overlayPath string) (command.Result, error) {
	eng := host.Engine()
	var b strings.Builder
	if eng != nil {
		b.WriteString("# Active system prompt (current turn)\n\n")
		b.WriteString(eng.SystemPrompt())
		b.WriteString("\n\n---\n\n")
	}
	overlay, err := os.ReadFile(overlayPath)
	if err == nil && len(overlay) > 0 {
		fmt.Fprintf(&b, "# Persistent user overlay (%s)\n\n%s\n", overlayPath, string(overlay))
	} else {
		fmt.Fprintf(&b, "# Persistent user overlay (%s)\n\n(empty — use /system edit to add one)\n", overlayPath)
	}
	return command.Result{Text: b.String()}, nil
}

func editSystem(overlayPath string) (command.Result, error) {
	if _, err := os.Stat(overlayPath); os.IsNotExist(err) {
		seed := "# anthrogo system prompt overlay\n# This text is appended verbatim to the system prompt sent to the model.\n# Save and restart anthrogo to apply.\n\n"
		if err := os.MkdirAll(filepath.Dir(overlayPath), 0o755); err != nil {
			return command.Result{Text: "could not create directory: " + err.Error()}, nil
		}
		if err := os.WriteFile(overlayPath, []byte(seed), 0o644); err != nil {
			return command.Result{Text: "could not seed overlay file: " + err.Error()}, nil
		}
	}
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	return command.Result{Text: fmt.Sprintf(
		"Edit your overlay outside anthrogo:\n  %s %s\nor open %s in any editor.\nChanges take effect when anthrogo restarts.",
		editor, overlayPath, overlayPath,
	)}, nil
}

func resetSystem(overlayPath string) (command.Result, error) {
	err := os.Remove(overlayPath)
	if err != nil && !os.IsNotExist(err) {
		return command.Result{Text: "reset failed: " + err.Error()}, nil
	}
	return command.Result{Text: "overlay reset (file removed). Effective next session — current engine's system prompt is frozen."}, nil
}
