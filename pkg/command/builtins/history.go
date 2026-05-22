package builtins

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ricardo/anthrogo/pkg/command"
)

// History is the /history slash command: list, search, and clear past input history.
type History struct {
	// Path overrides the default ~/.anthrogo/input_history location (useful in tests).
	Path string
}

func (History) Name() string      { return "/history" }
func (History) Aliases() []string { return nil }
func (History) Description() string {
	return "Show or replay past input prompts (subcommands: list [N], search <kw>, clear)"
}
func (History) Type() command.Type { return command.TypeLocal }

func (h History) Run(_ context.Context, args string, _ command.Host) (command.Result, error) {
	path := h.Path
	if path == "" {
		path = filepath.Join(os.Getenv("HOME"), ".anthrogo", "input_history")
	}
	args = strings.TrimSpace(args)
	switch {
	case args == "" || args == "list" || strings.HasPrefix(args, "list "):
		limit := 20
		if strings.HasPrefix(args, "list ") {
			if n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(args, "list "))); err == nil && n > 0 {
				limit = n
			}
		}
		return listHistory(path, limit)
	case strings.HasPrefix(args, "search "):
		kw := strings.TrimSpace(strings.TrimPrefix(args, "search "))
		return searchHistory(path, kw)
	case args == "clear":
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return command.Result{Text: "history clear failed: " + err.Error()}, nil
		}
		return command.Result{Text: "input history cleared"}, nil
	default:
		return command.Result{Text: "usage: /history [list [N] | search <keyword> | clear]"}, nil
	}
}

func listHistory(path string, limit int) (command.Result, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return command.Result{Text: "(no input history yet)"}, nil
		}
		return command.Result{Text: "history: " + err.Error()}, nil
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if len(lines) == 0 {
		return command.Result{Text: "(no input history yet)"}, nil
	}
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	var b strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&b, "%4d  %s\n", i+1, line)
	}
	return command.Result{Text: b.String()}, nil
}

func searchHistory(path, kw string) (command.Result, error) {
	if kw == "" {
		return command.Result{Text: "usage: /history search <keyword>"}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return command.Result{Text: "(no input history yet)"}, nil
		}
		return command.Result{Text: "history: " + err.Error()}, nil
	}
	defer f.Close()
	var hits []string
	sc := bufio.NewScanner(f)
	lk := strings.ToLower(kw)
	n := 0
	for sc.Scan() {
		n++
		line := sc.Text()
		if strings.Contains(strings.ToLower(line), lk) {
			hits = append(hits, fmt.Sprintf("%4d  %s", n, line))
		}
	}
	if len(hits) == 0 {
		return command.Result{Text: "(no matches)"}, nil
	}
	return command.Result{Text: strings.Join(hits, "\n")}, nil
}
