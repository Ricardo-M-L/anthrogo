package builtins

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ricardo/anthrogo/internal/config"
	"github.com/ricardo/anthrogo/pkg/command"
)

// System implements /system [show | edit [home|project] | reset [home|project]].
type System struct {
	// HomeOverlayPath is the path to the home-level overlay.
	// If empty, derived from $HOME at run time (back-compat).
	HomeOverlayPath string
	// ProjectOverlayPath is the path to the project (cwd-level) overlay.
	// If empty, /system show will not display a project section.
	ProjectOverlayPath string
}

func (System) Name() string      { return "/system" }
func (System) Aliases() []string { return nil }
func (System) Description() string {
	return "Inspect or edit the system prompt overlays (subcommands: show, edit [home|project], reset [home|project])"
}
func (System) Type() command.Type { return command.TypeLocal }

func (s System) Run(_ context.Context, args string, host command.Host) (command.Result, error) {
	args = strings.TrimSpace(args)
	home := os.Getenv("HOME")
	homeOverlayPath := s.HomeOverlayPath
	if homeOverlayPath == "" {
		homeOverlayPath = config.SystemOverlayPath(home)
	}
	projectOverlayPath := s.ProjectOverlayPath
	switch args {
	case "", "show":
		return showSystem(host, homeOverlayPath, projectOverlayPath)
	case "edit", "edit home":
		return editSystem(homeOverlayPath, "home")
	case "edit project":
		return editSystem(projectOverlayPath, "project")
	case "reset", "reset home":
		return resetSystem(homeOverlayPath, "home")
	case "reset project":
		return resetSystem(projectOverlayPath, "project")
	default:
		return command.Result{Text: "usage: /system [show | edit [home|project] | reset [home|project]]"}, nil
	}
}

func showSystem(host command.Host, homeOverlayPath, projectOverlayPath string) (command.Result, error) {
	eng := host.Engine()
	var b strings.Builder
	if eng != nil {
		b.WriteString("# Active system prompt (current turn)\n\n")
		b.WriteString(eng.SystemPrompt())
		b.WriteString("\n\n---\n\n")
	}

	home, _ := os.ReadFile(homeOverlayPath)
	fmt.Fprintf(&b, "# Home overlay (%s)\n\n", homeOverlayPath)
	if len(home) > 0 {
		b.Write(home)
		b.WriteString("\n\n")
	} else {
		b.WriteString("(empty)\n\n")
	}

	if projectOverlayPath != "" {
		project, _ := os.ReadFile(projectOverlayPath)
		fmt.Fprintf(&b, "# Project overlay (%s)\n\n", projectOverlayPath)
		if len(project) > 0 {
			b.Write(project)
			b.WriteString("\n")
		} else {
			b.WriteString("(empty)\n")
		}
	}
	return command.Result{Text: b.String()}, nil
}

func editSystem(overlayPath, label string) (command.Result, error) {
	if overlayPath == "" {
		return command.Result{Text: "no " + label + " overlay path configured"}, nil
	}
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
	cmd := exec.Command(editor, overlayPath)
	return command.Result{
		Text: fmt.Sprintf("opening %s overlay in %s...", label, editor),
		ExecCmd: &command.ExecRequest{
			Cmd: cmd,
			OnComplete: func(err error) string {
				if err != nil {
					return fmt.Sprintf("editor (%s overlay) exited with error: %v", label, err)
				}
				if info, statErr := os.Stat(overlayPath); statErr == nil {
					return fmt.Sprintf("%s overlay saved (%d bytes). Effective next turn (current engine's system prompt is frozen for this session).", label, info.Size())
				}
				return fmt.Sprintf("%s overlay saved.", label)
			},
		},
	}, nil
}

func resetSystem(overlayPath, label string) (command.Result, error) {
	if overlayPath == "" {
		return command.Result{Text: "no " + label + " overlay path configured"}, nil
	}
	err := os.Remove(overlayPath)
	if err != nil && !os.IsNotExist(err) {
		return command.Result{Text: "reset failed: " + err.Error()}, nil
	}
	return command.Result{Text: label + " overlay reset (file removed). Effective next session — current engine's system prompt is frozen."}, nil
}
