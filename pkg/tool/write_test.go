package tool

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWrite_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "out.txt")
	_, err := (&Write{}).Call(context.Background(), map[string]any{"file_path": p, "content": "hello"}, &Context{})
	require.NoError(t, err)
	got, err := os.ReadFile(p)
	require.NoError(t, err)
	require.Equal(t, "hello", string(got))
}

func TestWrite_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.txt")
	require.NoError(t, os.WriteFile(p, []byte("old"), 0o644))
	_, err := (&Write{}).Call(context.Background(), map[string]any{"file_path": p, "content": "new"}, &Context{})
	require.NoError(t, err)
	got, err := os.ReadFile(p)
	require.NoError(t, err)
	require.Equal(t, "new", string(got))
}

func TestWrite_MissingFields(t *testing.T) {
	got, err := (&Write{}).Call(context.Background(), map[string]any{}, &Context{})
	require.NoError(t, err)
	require.True(t, got.IsError)
}
