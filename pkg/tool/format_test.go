package tool

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormat_GoFile(t *testing.T) {
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not on PATH")
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "bad.go")
	// Poorly formatted Go: no tabs, extra spaces
	unformatted := "package main\n\nfunc main()   {\nfoo:=1\n_ = foo\n}\n"
	require.NoError(t, os.WriteFile(src, []byte(unformatted), 0644))

	res, err := Format{}.Call(context.Background(), map[string]any{"path": src}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError, "unexpected error: %s", res.Text)
	require.Contains(t, res.Text, "formatted")

	// Re-read and verify tab indentation was applied.
	data, err := os.ReadFile(src)
	require.NoError(t, err)
	require.True(t, strings.Contains(string(data), "\t"), "expected tab indentation after gofmt")
}

func TestFormat_MissingPath(t *testing.T) {
	res, err := Format{}.Call(context.Background(), map[string]any{}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "path is required")
}

func TestFormat_UnsupportedExtension(t *testing.T) {
	res, err := Format{}.Call(context.Background(), map[string]any{"path": "/tmp/file.xyz"}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "unsupported")
}

func TestFormat_PrettierMissingSkips(t *testing.T) {
	// Only test this if prettier is not on PATH.
	if _, err := exec.LookPath("prettier"); err == nil {
		t.Skip("prettier is on PATH; skipping missing-prettier test")
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "file.ts")
	require.NoError(t, os.WriteFile(src, []byte("const x=1\n"), 0644))

	res, err := Format{}.Call(context.Background(), map[string]any{"path": src}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "not found on PATH")
}
