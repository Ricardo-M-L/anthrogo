package tool

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEdit_ExactReplace(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.go")
	require.NoError(t, os.WriteFile(p, []byte("var a int\nvar b int\n"), 0o644))

	_, err := (&Edit{}).Call(context.Background(), map[string]any{
		"file_path": p, "old_string": "var a int", "new_string": "var a int64",
	}, &Context{})
	require.NoError(t, err)
	got, _ := os.ReadFile(p)
	require.Equal(t, "var a int64\nvar b int\n", string(got))
}

func TestEdit_ReplaceAll(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.txt")
	require.NoError(t, os.WriteFile(p, []byte("foo foo foo\n"), 0o644))
	_, err := (&Edit{}).Call(context.Background(), map[string]any{
		"file_path": p, "old_string": "foo", "new_string": "bar", "replace_all": true,
	}, &Context{})
	require.NoError(t, err)
	got, _ := os.ReadFile(p)
	require.Equal(t, "bar bar bar\n", string(got))
}

func TestEdit_NonUnique_NoReplaceAll_Errors(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.txt")
	require.NoError(t, os.WriteFile(p, []byte("foo foo\n"), 0o644))
	got, err := (&Edit{}).Call(context.Background(), map[string]any{
		"file_path": p, "old_string": "foo", "new_string": "bar",
	}, &Context{})
	require.NoError(t, err)
	require.True(t, got.IsError)
}

func TestEdit_NoMatch_Errors(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.txt")
	require.NoError(t, os.WriteFile(p, []byte("hello\n"), 0o644))
	got, _ := (&Edit{}).Call(context.Background(), map[string]any{
		"file_path": p, "old_string": "world", "new_string": "x",
	}, &Context{})
	require.True(t, got.IsError)
}
