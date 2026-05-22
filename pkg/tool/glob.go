package tool

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

type Glob struct{ DefaultPermission }

func (Glob) Name() string                       { return "Glob" }
func (Glob) Description(context.Context) string { return globDescription }
func (Glob) UserFacingName(input map[string]any) string {
	if p, ok := input["pattern"].(string); ok {
		return "Glob " + p
	}
	return "Glob"
}
func (Glob) IsReadOnly() bool        { return true }
func (Glob) IsConcurrencySafe() bool { return true }

func (Glob) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{"type": "string"},
			"path":    map[string]any{"type": "string", "description": "Base directory (default: cwd)."},
		},
		"required": []string{"pattern"},
	}
}

func (Glob) Call(_ context.Context, input map[string]any, tcx *Context) (Result, error) {
	pattern, _ := input["pattern"].(string)
	base, _ := input["path"].(string)
	if pattern == "" {
		return errResult("pattern is required"), nil
	}
	if base == "" {
		if tcx != nil && tcx.Cwd != "" {
			base = tcx.Cwd
		} else {
			base, _ = os.Getwd()
		}
	}
	matches, err := doublestar.Glob(os.DirFS(base), pattern)
	if err != nil {
		return errResult(err.Error()), nil
	}
	sort.Slice(matches, func(i, j int) bool {
		fi, _ := os.Stat(filepath.Join(base, matches[i]))
		fj, _ := os.Stat(filepath.Join(base, matches[j]))
		if fi == nil || fj == nil {
			return matches[i] < matches[j]
		}
		return fi.ModTime().After(fj.ModTime())
	})
	if len(matches) == 0 {
		msg := "no matches for pattern " + pattern + " under " + base
		return Result{Type: ResultText, Text: msg, ForLLM: msg}, nil
	}
	out := strings.Join(matches, "\n")
	return Result{Type: ResultText, Text: out, ForLLM: out, Data: map[string]any{"count": len(matches)}}, nil
}

const globDescription = `Find files by glob pattern (doublestar syntax: ** for any depth). Returns paths newest-first, relative to path (or cwd).`
