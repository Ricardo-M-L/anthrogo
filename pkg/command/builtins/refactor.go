package builtins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/ricardo/anthrogo/pkg/command"
	"github.com/ricardo/anthrogo/pkg/query"
)

// Refactor implements the /refactor slash command.
// Usage: /refactor <glob-pattern> -- <instruction>
type Refactor struct{}

func (Refactor) Name() string      { return "/refactor" }
func (Refactor) Aliases() []string { return nil }
func (Refactor) Description() string {
	return "Refactor multiple files matching a glob with a natural-language instruction. Usage: /refactor <pattern> -- <instruction>"
}
func (Refactor) Type() command.Type { return command.TypeLocal }

const (
	refactorMaxFiles      = 50
	refactorMaxFileBytes  = 50 * 1024
	refactorMaxTotalBytes = 500 * 1024
)

func (Refactor) Run(ctx context.Context, args string, host command.Host) (command.Result, error) {
	args = strings.TrimSpace(args)
	if args == "" {
		return command.Result{Text: "usage: /refactor <pattern> -- <instruction>"}, nil
	}
	sep := strings.Index(args, " -- ")
	if sep < 0 {
		return command.Result{Text: "usage: /refactor <pattern> -- <instruction>"}, nil
	}
	pattern := strings.TrimSpace(args[:sep])
	instruction := strings.TrimSpace(args[sep+4:])
	if pattern == "" || instruction == "" {
		return command.Result{Text: "usage: /refactor <pattern> -- <instruction>"}, nil
	}

	cwd := host.Cwd()
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return command.Result{Text: "refactor: cannot determine cwd: " + err.Error()}, nil
		}
	}

	fsys := os.DirFS(cwd)
	matches, err := doublestar.Glob(fsys, pattern)
	if err != nil {
		return command.Result{Text: "refactor: glob error: " + err.Error()}, nil
	}
	if len(matches) == 0 {
		return command.Result{Text: "refactor: no files match " + pattern}, nil
	}
	if len(matches) > refactorMaxFiles {
		return command.Result{
			Text: fmt.Sprintf("refactor: too many matches (%d > %d); narrow the pattern", len(matches), refactorMaxFiles),
		}, nil
	}

	// Build the subagent prompt body.
	var b strings.Builder
	fmt.Fprintf(&b, "Refactor instruction:\n%s\n\nFiles in scope (absolute paths):\n", instruction)
	totalBytes := 0
	inScope := 0
	for _, rel := range matches {
		abs := filepath.Join(cwd, rel)
		info, statErr := os.Stat(abs)
		if statErr != nil || info.IsDir() {
			continue
		}
		sz := int(info.Size())
		fmt.Fprintf(&b, "  %s (%d bytes)\n", abs, sz)
		if sz > refactorMaxFileBytes {
			fmt.Fprintf(&b, "    note: file exceeds 50 KB; use Read with offset/limit\n")
		}
		totalBytes += sz
		inScope++
	}
	if totalBytes > refactorMaxTotalBytes {
		fmt.Fprintf(&b,
			"\nWARNING: total in-scope bytes (%d) exceed %d; the refactor may be too large for a single pass.\n",
			totalBytes, refactorMaxTotalBytes,
		)
	}
	b.WriteString("\nApply the refactor: read each file with Read, make targeted edits with Edit (prefer Edit over Write for partial changes). Verify each edit by re-reading. At the end, summarize what changed per file.\n")

	eng := host.Engine()
	if eng == nil {
		// No engine available (e.g., unit test without engine); return AgentTask
		// descriptor so callers can inspect the dispatch intent.
		return command.Result{
			Text: fmt.Sprintf("dispatching refactor subagent: %d file(s) in scope", inScope),
			AgentTask: &command.AgentTask{
				Description:   fmt.Sprintf("refactor %d file(s): %s", inScope, refactorTruncate(instruction, 60)),
				Prompt:        b.String(),
				SubagentType:  "refactor",
				ToolAllowlist: []string{"Read", "Edit", "Write", "Glob", "Grep"},
			},
		}, nil
	}

	host.AppendUIMessage(fmt.Sprintf("refactor: dispatching subagent for %d file(s)…", inScope))

	result, runErr := eng.RunSubagent(ctx, query.SubagentOptions{
		Type:        "refactor",
		Description: fmt.Sprintf("refactor %d file(s): %s", inScope, refactorTruncate(instruction, 60)),
		Prompt:      b.String(),
	})
	if runErr != nil {
		return command.Result{Text: "refactor: subagent error: " + runErr.Error()}, nil
	}

	summary := fmt.Sprintf("refactored %d file(s) in scope\n\n%s", inScope, result)
	return command.Result{Text: summary}, nil
}

func refactorTruncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
