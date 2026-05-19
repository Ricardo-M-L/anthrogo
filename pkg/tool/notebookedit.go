package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

type NotebookEdit struct{ DefaultPermission }

func (NotebookEdit) Name() string                       { return "NotebookEdit" }
func (NotebookEdit) Description(context.Context) string { return notebookEditDescription }
func (NotebookEdit) UserFacingName(input map[string]any) string {
	if p, _ := input["file_path"].(string); p != "" {
		return "NotebookEdit " + p
	}
	return "NotebookEdit"
}
func (NotebookEdit) IsReadOnly() bool        { return false }
func (NotebookEdit) IsConcurrencySafe() bool { return false }

func (NotebookEdit) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{"type": "string"},
			"operation": map[string]any{
				"type": "string",
				"enum": []string{"replace_cell", "insert_cell_before", "insert_cell_after", "delete_cell"},
			},
			"cell_index": map[string]any{"type": "integer"},
			"cell_type":  map[string]any{"type": "string", "enum": []string{"code", "markdown", "raw"}},
			"new_source": map[string]any{"type": "string"},
		},
		"required": []string{"file_path", "operation", "cell_index"},
	}
}

func (NotebookEdit) Call(_ context.Context, input map[string]any, _ *Context) (Result, error) {
	path, _ := input["file_path"].(string)
	op, _ := input["operation"].(string)
	idx := intField(input, "cell_index", -1)
	newSource, _ := input["new_source"].(string)
	cellType, _ := input["cell_type"].(string)
	if cellType == "" {
		cellType = "code"
	}
	if path == "" || op == "" || idx < 0 {
		return errResult("file_path, operation, and cell_index are required"), nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return errResult(err.Error()), nil
	}
	var nb map[string]any
	if err := json.Unmarshal(raw, &nb); err != nil {
		return errResult("invalid notebook JSON: " + err.Error()), nil
	}
	format, _ := nb["nbformat"].(float64)
	if format != 4 {
		return errResult(fmt.Sprintf("unsupported nbformat: %v (M2 supports v4 only)", format)), nil
	}
	cells, _ := nb["cells"].([]any)
	makeCell := func() map[string]any {
		return map[string]any{
			"cell_type":       cellType,
			"source":          []any{newSource},
			"metadata":        map[string]any{},
			"outputs":         []any{},
			"execution_count": nil,
		}
	}
	switch op {
	case "replace_cell":
		if idx >= len(cells) {
			return errResult(fmt.Sprintf("cell_index %d out of range (have %d cells)", idx, len(cells))), nil
		}
		cell, _ := cells[idx].(map[string]any)
		cell["source"] = []any{newSource}
		cells[idx] = cell
	case "insert_cell_before":
		if idx > len(cells) {
			return errResult("cell_index out of range"), nil
		}
		cells = append(cells[:idx], append([]any{makeCell()}, cells[idx:]...)...)
	case "insert_cell_after":
		if idx >= len(cells) {
			return errResult("cell_index out of range"), nil
		}
		cells = append(cells[:idx+1], append([]any{makeCell()}, cells[idx+1:]...)...)
	case "delete_cell":
		if idx >= len(cells) {
			return errResult("cell_index out of range"), nil
		}
		cells = append(cells[:idx], cells[idx+1:]...)
	default:
		return errResult("unknown operation: " + op), nil
	}
	nb["cells"] = cells

	out, err := json.MarshalIndent(nb, "", "  ")
	if err != nil {
		return errResult(err.Error()), nil
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return errResult(err.Error()), nil
	}
	msg := fmt.Sprintf("%s applied at cell %d", op, idx)
	return Result{Type: ResultText, Text: msg, ForLLM: msg}, nil
}

const notebookEditDescription = `Edit a Jupyter notebook (.ipynb, nbformat v4). Operations: replace_cell, insert_cell_before, insert_cell_after, delete_cell. cell_index is 0-based.`
