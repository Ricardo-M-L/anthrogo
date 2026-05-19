package tool

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrep_FindsMatchesInTree(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package x\nfunc Foo() {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.go"), []byte("package x\nfunc Bar() {}\n"), 0o644))
	res, err := (&Grep{}).Call(context.Background(), map[string]any{
		"pattern": "func ", "path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.Contains(t, res.Text, "a.go")
	require.Contains(t, res.Text, "b.go")
}

func TestGrep_PathGlobFilter(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("foo\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("foo\n"), 0o644))
	res, err := (&Grep{}).Call(context.Background(), map[string]any{
		"pattern": "foo", "path": dir, "glob": "**/*.go",
	}, &Context{})
	require.NoError(t, err)
	require.Contains(t, res.Text, "a.go")
	require.NotContains(t, res.Text, "a.txt")
}

func TestGrep_RegexInvalid(t *testing.T) {
	res, _ := (&Grep{}).Call(context.Background(), map[string]any{"pattern": "([", "path": t.TempDir()}, &Context{})
	require.True(t, res.IsError)
}

func TestGrep_CountMode_IsDeterministic(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{"a.go", "b.go", "c.go", "d.go", "e.go"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, p), []byte("foo\n"), 0o644))
	}
	first, _ := (&Grep{}).Call(context.Background(), map[string]any{
		"pattern": "foo", "path": dir, "output_mode": "count",
	}, &Context{})
	for i := 0; i < 10; i++ {
		got, _ := (&Grep{}).Call(context.Background(), map[string]any{
			"pattern": "foo", "path": dir, "output_mode": "count",
		}, &Context{})
		require.Equal(t, first.Text, got.Text, "iteration %d differs from first", i)
	}
}
