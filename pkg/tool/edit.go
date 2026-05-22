package tool

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type Edit struct{ DefaultPermission }

func (Edit) Name() string                       { return "Edit" }
func (Edit) Description(context.Context) string { return editDescription }
func (Edit) UserFacingName(input map[string]any) string {
	if p, ok := input["file_path"].(string); ok {
		return "Edit " + p
	}
	return "Edit"
}
func (Edit) IsReadOnly() bool        { return false }
func (Edit) IsConcurrencySafe() bool { return false }

func (Edit) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path":   map[string]any{"type": "string"},
			"old_string":  map[string]any{"type": "string"},
			"new_string":  map[string]any{"type": "string"},
			"replace_all": map[string]any{"type": "boolean", "default": false},
		},
		"required": []string{"file_path", "old_string", "new_string"},
	}
}

func (Edit) Call(_ context.Context, input map[string]any, _ *Context) (Result, error) {
	path, _ := input["file_path"].(string)
	oldS, _ := input["old_string"].(string)
	newS, _ := input["new_string"].(string)
	all, _ := input["replace_all"].(bool)
	if path == "" || oldS == "" {
		return errResult("file_path and old_string are required"), nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return errResult(err.Error()), nil
	}
	src := string(raw)
	count := strings.Count(src, oldS)
	if count == 0 {
		return errResult("old_string not found in file"), nil
	}
	if count > 1 && !all {
		return errResult(fmt.Sprintf("old_string is not unique (%d matches); pass replace_all=true or expand context", count)), nil
	}
	var out string
	if all {
		out = strings.ReplaceAll(src, oldS, newS)
	} else {
		out = strings.Replace(src, oldS, newS, 1)
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return errResult(err.Error()), nil
	}
	return Result{Type: ResultText, Text: "edit applied", ForLLM: "edit applied to " + path}, nil
}

const editDescription = `Replace old_string with new_string in file_path. old_string must be unique unless replace_all=true. Use larger surrounding context to disambiguate when the literal occurs multiple times.`
