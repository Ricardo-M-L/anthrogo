package builtins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ricardo/anthrogo/internal/yamlsafe"
	"github.com/ricardo/anthrogo/pkg/command"
)

// Hook implements the /hook slash command for managing hook bundles.
type Hook struct {
	HomeRoot string // ~/.anthrogo/hooks/
}

func (Hook) Name() string      { return "/hook" }
func (Hook) Aliases() []string { return nil }
func (Hook) Description() string {
	return "Manage hook bundles (subcommands: list, install <src>, remove <name>)"
}
func (Hook) Type() command.Type { return command.TypeLocal }

func (h Hook) Run(_ context.Context, args string, _ command.Host) (command.Result, error) {
	args = strings.TrimSpace(args)
	switch {
	case args == "" || args == "list":
		return listHookBundles(h.HomeRoot)
	case strings.HasPrefix(args, "install "):
		src := strings.TrimSpace(strings.TrimPrefix(args, "install "))
		return installHookBundle(src, h.HomeRoot)
	case strings.HasPrefix(args, "remove "):
		name := strings.TrimSpace(strings.TrimPrefix(args, "remove "))
		return removeHookBundle(name, h.HomeRoot)
	default:
		return command.Result{Text: "usage: /hook [list | install <src> | remove <name>]"}, nil
	}
}

func listHookBundles(root string) (command.Result, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return command.Result{Text: "(no hook bundles installed)"}, nil
		}
		return command.Result{Text: "list: " + err.Error()}, nil
	}
	var lines []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, e.Name(), "hook.yaml")); err == nil {
			lines = append(lines, "  "+e.Name())
		}
	}
	if len(lines) == 0 {
		return command.Result{Text: "(no hook bundles installed)"}, nil
	}
	return command.Result{
		Text: "Hook bundles:\n" + strings.Join(lines, "\n") +
			"\n\nNote: reference hook bundles in settings.yaml hooks: section via absolute path to hook.sh",
	}, nil
}

func installHookBundle(src, root string) (command.Result, error) {
	// URL/git+ install is deferred for M13.6; only local-dir supported.
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") || strings.HasPrefix(src, "git+") {
		return command.Result{
			Text: "install: URL/git+ install for hooks is deferred — copy the bundle dir locally first, then run /hook install <local-dir>",
		}, nil
	}

	info, err := os.Stat(src)
	if err != nil {
		return command.Result{Text: "install: " + err.Error()}, nil
	}
	if !info.IsDir() {
		return command.Result{Text: "install: src must be a directory containing hook.yaml"}, nil
	}

	raw, err := os.ReadFile(filepath.Join(src, "hook.yaml"))
	if err != nil {
		return command.Result{Text: "install: no hook.yaml in " + src}, nil
	}
	var m struct {
		Name string `yaml:"name"`
	}
	var warns []string
	if err := yamlsafe.Unmarshal(raw, &m, "hook.yaml", &warns); err != nil {
		return command.Result{Text: "install: bad yaml: " + err.Error()}, nil
	}
	for _, w := range warns {
		fmt.Fprintf(os.Stderr, "anthrogo: %s\n", w)
	}
	if m.Name == "" {
		return command.Result{Text: "install: empty hook name in hook.yaml"}, nil
	}

	dest := filepath.Join(root, m.Name)
	if _, err := os.Stat(dest); err == nil {
		return command.Result{Text: fmt.Sprintf("install: hook bundle %q already exists at %s", m.Name, dest)}, nil
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return command.Result{Text: "install: mkdir: " + err.Error()}, nil
	}
	if err := copyTree(src, dest); err != nil {
		return command.Result{Text: "install: copy: " + err.Error()}, nil
	}
	return command.Result{
		Text: fmt.Sprintf(
			"installed hook bundle: %s\nadd to settings.yaml manually:\nhooks:\n  PreToolUse:\n    - command: %s/hook.sh",
			m.Name, dest,
		),
	}, nil
}

func removeHookBundle(name, root string) (command.Result, error) {
	target := filepath.Join(root, name)
	info, err := os.Stat(target)
	if err != nil {
		return command.Result{Text: "remove: " + err.Error()}, nil
	}
	if !info.IsDir() {
		return command.Result{Text: "remove: not a hook bundle dir"}, nil
	}
	if err := os.RemoveAll(target); err != nil {
		return command.Result{Text: "remove: " + err.Error()}, nil
	}
	return command.Result{Text: "removed hook bundle: " + name}, nil
}

// copyTree recursively copies src to dst, preserving file modes.
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}
