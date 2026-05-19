package tool

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGlob_FindsByExtension(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{"a.go", "sub/b.go", "c.txt"} {
		full := filepath.Join(dir, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(""), 0o644))
	}
	res, err := (&Glob{}).Call(context.Background(), map[string]any{
		"path": dir, "pattern": "**/*.go",
	}, &Context{})
	require.NoError(t, err)
	require.Contains(t, res.Text, "a.go")
	require.Contains(t, res.Text, "sub/b.go")
	require.NotContains(t, res.Text, "c.txt")
}

func TestGlob_NoMatch_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	res, err := (&Glob{}).Call(context.Background(), map[string]any{"path": dir, "pattern": "**/*.foo"}, &Context{})
	require.NoError(t, err)
	require.NotEmpty(t, res.Text) // user-facing "no matches"
	require.False(t, res.IsError)
}
