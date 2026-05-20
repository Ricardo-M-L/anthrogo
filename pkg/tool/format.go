package tool

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Format formats one or more source files using the language-appropriate formatter.
type Format struct{ DefaultPermission }

func (Format) Name() string                      { return "Format" }
func (Format) Description(context.Context) string { return formatDescription }
func (Format) IsReadOnly() bool                  { return false }
func (Format) IsConcurrencySafe() bool           { return true }

func (Format) UserFacingName(input map[string]any) string {
	if paths := strSliceField(input, "paths"); len(paths) > 0 {
		return "Format: " + strings.Join(paths, " ")
	}
	if p, ok := input["path"].(string); ok && p != "" {
		return "Format: " + p
	}
	return "Format"
}

func (Format) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Deprecated; use paths.",
			},
			"paths": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
				"description": "List of file paths to format. Preferred over singular path.",
			},
			"language": map[string]any{
				"type":        "string",
				"description": "Optional language hint (go, javascript, typescript, python, rust). Inferred from file extension if absent.",
			},
		},
	}
}

func (Format) Call(_ context.Context, input map[string]any, tcx *Context) (Result, error) {
	// Coerce: prefer paths array, fall back to singular path.
	var pathList []string
	if ps := strSliceField(input, "paths"); len(ps) > 0 {
		pathList = ps
	} else if p, ok := input["path"].(string); ok && p != "" {
		pathList = []string{p}
	}
	if len(pathList) == 0 {
		return errResult("path is required"), nil
	}

	lang, _ := input["language"].(string)

	var succeeded []string
	var failures []string

	for _, path := range pathList {
		effectiveLang := lang
		if effectiveLang == "" {
			effectiveLang = langFromExt(path)
		}
		if err := formatOne(path, effectiveLang, tcx); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %s", path, err.Error()))
		} else {
			succeeded = append(succeeded, path)
		}
	}

	total := len(pathList)
	done := len(succeeded)

	var msg string
	if len(failures) == 0 {
		msg = fmt.Sprintf("formatted %d/%d: %s", done, total, strings.Join(succeeded, " "))
	} else {
		parts := make([]string, 0, done+len(failures))
		parts = append(parts, succeeded...)
		for _, f := range failures {
			parts = append(parts, "FAILED "+f)
		}
		msg = fmt.Sprintf("formatted %d/%d: %s", done, total, strings.Join(parts, " "))
	}

	if done == 0 {
		return errResult(msg), nil
	}
	return Result{Type: ResultText, Text: msg, ForLLM: msg}, nil
}

// formatOne formats a single file, returning an error on failure.
func formatOne(path, lang string, tcx *Context) error {
	var args []string
	var bin string

	switch lang {
	case "go":
		bin = "gofmt"
		args = []string{"-w", path}

	case "javascript", "typescript", "json", "css", "html", "yaml", "markdown":
		bin = "prettier"
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("prettier not found on PATH")
		}
		args = []string{"--write", path}

	case "python":
		if _, err := exec.LookPath("black"); err == nil {
			bin = "black"
			args = []string{path}
		} else if _, err := exec.LookPath("ruff"); err == nil {
			bin = "ruff"
			args = []string{"format", path}
		} else {
			return fmt.Errorf("no Python formatter found on PATH (tried black, ruff)")
		}

	case "rust":
		bin = "rustfmt"
		args = []string{path}

	default:
		return fmt.Errorf("unsupported language or file extension: %s", path)
	}

	cmd := exec.Command(bin, args...)
	if tcx != nil && tcx.Cwd != "" {
		cmd.Dir = tcx.Cwd
	}
	var errb bytes.Buffer
	cmd.Stderr = &errb

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

// strSliceField extracts a []string from input[key], accepting both
// []string and []any (JSON-decoded arrays).
func strSliceField(input map[string]any, key string) []string {
	v, ok := input[key]
	if !ok {
		return nil
	}
	switch val := v.(type) {
	case []string:
		return val
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// langFromExt maps a file extension to a language token.
func langFromExt(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".json":
		return "json"
	case ".css", ".scss", ".less":
		return "css"
	case ".html", ".htm":
		return "html"
	case ".yaml", ".yml":
		return "yaml"
	case ".md", ".markdown":
		return "markdown"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	default:
		return ""
	}
}

const formatDescription = `Format one or more source files using language-appropriate formatters (gofmt / prettier / black / rustfmt). Use paths (array) for batch; legacy singular path still accepted.`
