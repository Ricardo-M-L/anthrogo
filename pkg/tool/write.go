package tool

import (
	"context"
	"os"
	"path/filepath"
)

type Write struct{ DefaultPermission }

func (Write) Name() string                      { return "Write" }
func (Write) Description(context.Context) string { return writeDescription }
func (Write) UserFacingName(input map[string]any) string {
	if p, ok := input["file_path"].(string); ok {
		return "Write " + p
	}
	return "Write"
}
func (Write) IsReadOnly() bool        { return false }
func (Write) IsConcurrencySafe() bool { return false }

func (Write) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{"type": "string"},
			"content":   map[string]any{"type": "string"},
		},
		"required": []string{"file_path", "content"},
	}
}

func (Write) Call(_ context.Context, input map[string]any, _ *Context) (Result, error) {
	path, _ := input["file_path"].(string)
	content, _ := input["content"].(string)
	if path == "" {
		return errResult("file_path is required"), nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return errResult(err.Error()), nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return errResult(err.Error()), nil
	}
	return Result{Type: ResultText, Text: "wrote " + path, ForLLM: "wrote " + path}, nil
}

const writeDescription = `Write file_path with content. Creates parent directories. Overwrites any existing file at file_path. Returns an error string in the result when path is missing or write fails.`
