package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func sampleNotebook() string {
	return `{
		"cells": [
			{"cell_type": "code", "source": ["print('a')"], "outputs": [], "execution_count": null, "metadata": {}},
			{"cell_type": "markdown", "source": ["# header"], "metadata": {}}
		],
		"metadata": {"kernelspec": {"name": "python3"}},
		"nbformat": 4,
		"nbformat_minor": 5
	}`
}

func writeNB(t *testing.T) string {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.ipynb")
	require.NoError(t, os.WriteFile(p, []byte(sampleNotebook()), 0o644))
	return p
}

func TestNotebookEdit_ReplaceCell(t *testing.T) {
	p := writeNB(t)
	_, err := (&NotebookEdit{}).Call(context.Background(), map[string]any{
		"file_path":  p,
		"operation":  "replace_cell",
		"cell_index": float64(0),
		"new_source": "print('b')",
	}, &Context{})
	require.NoError(t, err)
	raw, _ := os.ReadFile(p)
	var nb map[string]any
	require.NoError(t, json.Unmarshal(raw, &nb))
	cells := nb["cells"].([]any)
	src := cells[0].(map[string]any)["source"].([]any)
	require.Equal(t, "print('b')", src[0])
}

func TestNotebookEdit_InsertBefore(t *testing.T) {
	p := writeNB(t)
	_, err := (&NotebookEdit{}).Call(context.Background(), map[string]any{
		"file_path":  p,
		"operation":  "insert_cell_before",
		"cell_index": float64(0),
		"cell_type":  "code",
		"new_source": "import x",
	}, &Context{})
	require.NoError(t, err)
	raw, _ := os.ReadFile(p)
	var nb map[string]any
	_ = json.Unmarshal(raw, &nb)
	cells := nb["cells"].([]any)
	require.Len(t, cells, 3)
	require.Equal(t, "import x", cells[0].(map[string]any)["source"].([]any)[0])
}

func TestNotebookEdit_DeleteCell(t *testing.T) {
	p := writeNB(t)
	_, err := (&NotebookEdit{}).Call(context.Background(), map[string]any{
		"file_path":  p,
		"operation":  "delete_cell",
		"cell_index": float64(1),
	}, &Context{})
	require.NoError(t, err)
	raw, _ := os.ReadFile(p)
	var nb map[string]any
	_ = json.Unmarshal(raw, &nb)
	cells := nb["cells"].([]any)
	require.Len(t, cells, 1)
}

func TestNotebookEdit_RejectsV3(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "old.ipynb")
	require.NoError(t, os.WriteFile(p, []byte(`{"cells": [], "nbformat": 3, "nbformat_minor": 0, "metadata": {}}`), 0o644))
	res, _ := (&NotebookEdit{}).Call(context.Background(), map[string]any{
		"file_path":  p,
		"operation":  "delete_cell",
		"cell_index": float64(0),
	}, &Context{})
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "nbformat")
}

func TestNotebookEdit_BadIndex(t *testing.T) {
	p := writeNB(t)
	res, _ := (&NotebookEdit{}).Call(context.Background(), map[string]any{
		"file_path":  p,
		"operation":  "replace_cell",
		"cell_index": float64(99),
		"new_source": "x",
	}, &Context{})
	require.True(t, res.IsError)
}
