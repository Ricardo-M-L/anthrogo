package tool

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"
)

// Format formats a source file using the language-appropriate formatter.
type Format struct{ DefaultPermission }

func (Format) Name() string                      { return "Format" }
func (Format) Description(context.Context) string { return formatDescription }
func (Format) IsReadOnly() bool                  { return false }
func (Format) IsConcurrencySafe() bool           { return true }

func (Format) UserFacingName(input map[string]any) string {
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
				"type": "string",
			},
			"language": map[string]any{
				"type":        "string",
				"description": "Optional language hint (go, javascript, typescript, python, rust). Inferred from file extension if absent.",
			},
		},
		"required": []string{"path"},
	}
}

func (Format) Call(_ context.Context, input map[string]any, tcx *Context) (Result, error) {
	path, ok := input["path"].(string)
	if !ok || path == "" {
		return errResult("path is required"), nil
	}

	lang, _ := input["language"].(string)
	if lang == "" {
		lang = langFromExt(path)
	}

	var args []string
	var bin string

	switch lang {
	case "go":
		bin = "gofmt"
		args = []string{"-w", path}

	case "javascript", "typescript", "json", "css", "html", "yaml", "markdown":
		bin = "prettier"
		if _, err := exec.LookPath(bin); err != nil {
			return errResult("prettier not found on PATH"), nil
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
			return errResult("no Python formatter found on PATH (tried black, ruff)"), nil
		}

	case "rust":
		bin = "rustfmt"
		args = []string{path}

	default:
		return errResult("unsupported language or file extension: " + path), nil
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
		return errResult(msg), nil
	}

	out := "formatted " + path
	return Result{Type: ResultText, Text: out, ForLLM: out}, nil
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

const formatDescription = `Format a source file using language-appropriate formatter (gofmt / prettier / black / rustfmt).`
