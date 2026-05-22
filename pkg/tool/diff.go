package tool

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Diff wraps `git diff` for the working tree or staged area.
type Diff struct{ DefaultPermission }

func (Diff) Name() string                       { return "Diff" }
func (Diff) Description(context.Context) string { return diffDescription }
func (Diff) IsReadOnly() bool                   { return true }
func (Diff) IsConcurrencySafe() bool            { return true }

func (Diff) UserFacingName(input map[string]any) string {
	if p, ok := input["path"].(string); ok && p != "" {
		return "Diff: " + p
	}
	return "Diff"
}

func (Diff) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Optional file or directory path. Empty = whole repo.",
			},
			"cached": map[string]any{
				"type":        "boolean",
				"description": "Show staged changes (--cached).",
			},
			"context": map[string]any{
				"type":        "integer",
				"description": "Number of context lines (default 3).",
			},
			"stat": map[string]any{
				"type":        "boolean",
				"description": "Show --stat summary only.",
			},
			"range": map[string]any{
				"type":        "string",
				"description": "Commit range like 'HEAD~3..HEAD' or 'main..feature'. Mutually exclusive with cached.",
			},
		},
	}
}

func (Diff) Call(_ context.Context, input map[string]any, tcx *Context) (Result, error) {
	rangeVal, _ := input["range"].(string)
	cached, _ := input["cached"].(bool)

	if rangeVal != "" && cached {
		return errResult("range and cached are mutually exclusive"), nil
	}

	args := []string{"diff"}

	if rangeVal != "" {
		args = append(args, rangeVal)
	} else if cached {
		args = append(args, "--cached")
	}
	if stat, _ := input["stat"].(bool); stat {
		args = append(args, "--stat")
	}
	if ctxLines := intField(input, "context", -1); ctxLines >= 0 {
		args = append(args, fmt.Sprintf("-U%d", ctxLines))
	}
	if p, ok := input["path"].(string); ok && p != "" {
		args = append(args, "--", p)
	}

	cmd := exec.Command("git", args...)
	if tcx != nil && tcx.Cwd != "" {
		cmd.Dir = tcx.Cwd
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb

	err := cmd.Run()
	combined := out.String()
	if combined == "" {
		combined = errb.String()
	}

	// git diff exits 0 (no diff) or 1 (differences found) — both are normal.
	// Any other error code (e.g. 128: not a git repo) is a real error.
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok || exitErr.ExitCode() != 1 {
			msg := strings.TrimSpace(errb.String())
			if msg == "" {
				msg = err.Error()
			}
			return errResult(msg), nil
		}
	}

	return Result{Type: ResultText, Text: combined, ForLLM: combined}, nil
}

const diffDescription = `Show git diff for the working tree, staged area (cached), a commit range (range), or a path. Supports --stat, custom context lines.`
