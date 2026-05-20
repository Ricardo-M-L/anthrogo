package builtins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ricardo/anthrogo/internal/session"
	"github.com/ricardo/anthrogo/pkg/command"
)

// Sessions implements the /sessions builtin command.
type Sessions struct{}

func (Sessions) Name() string        { return "/sessions" }
func (Sessions) Aliases() []string   { return nil }
func (Sessions) Description() string { return "List session JSONLs for the current cwd (subcommands: show <id-prefix>)" }
func (Sessions) Type() command.Type  { return command.TypeLocal }

func (Sessions) Run(ctx context.Context, args string, host command.Host) (command.Result, error) {
	cwd := host.Cwd()
	args = strings.TrimSpace(args)
	dir, err := session.ProjectDir(cwd)
	if err != nil {
		return command.Result{Text: "sessions: " + err.Error()}, nil
	}
	switch {
	case args == "" || args == "list":
		return listSessions(dir)
	case strings.HasPrefix(args, "show "):
		prefix := strings.TrimSpace(strings.TrimPrefix(args, "show "))
		return showSession(dir, prefix)
	default:
		return command.Result{Text: "usage: /sessions [list | show <id-prefix>]"}, nil
	}
}

func listSessions(dir string) (command.Result, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return command.Result{Text: "(no sessions yet)"}, nil
		}
		return command.Result{Text: "sessions: " + err.Error()}, nil
	}
	type row struct {
		ID, Modified, Size string
	}
	var rows []row
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".jsonl")
		rows = append(rows, row{
			ID:       id,
			Modified: info.ModTime().Format("2006-01-02 15:04"),
			Size:     fmt.Sprintf("%d B", info.Size()),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Modified > rows[j].Modified })
	if len(rows) == 0 {
		return command.Result{Text: "(no sessions yet)"}, nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-38s  %-16s  %s\n", "ID", "Modified", "Size")
	for _, r := range rows {
		fmt.Fprintf(&b, "%-38s  %-16s  %s\n", r.ID, r.Modified, r.Size)
	}
	return command.Result{Text: b.String()}, nil
}

func showSession(dir, prefix string) (command.Result, error) {
	if prefix == "" {
		return command.Result{Text: "usage: /sessions show <id-prefix>"}, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return command.Result{Text: "sessions: " + err.Error()}, nil
	}
	var matched []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".jsonl") {
			matched = append(matched, e.Name())
		}
	}
	if len(matched) == 0 {
		return command.Result{Text: "sessions: no match for " + prefix}, nil
	}
	if len(matched) > 1 {
		return command.Result{Text: "sessions: ambiguous prefix " + prefix + " (matches: " + strings.Join(matched, ", ") + ")"}, nil
	}
	info, err := os.Stat(filepath.Join(dir, matched[0]))
	if err != nil {
		return command.Result{Text: "sessions: " + err.Error()}, nil
	}
	return command.Result{Text: fmt.Sprintf("session: %s\npath: %s\nmodified: %s\nsize: %d bytes\n",
		matched[0],
		filepath.Join(dir, matched[0]),
		info.ModTime().Format("2006-01-02 15:04:05"),
		info.Size(),
	)}, nil
}
