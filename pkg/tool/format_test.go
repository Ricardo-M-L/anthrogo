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

func TestFormat_BatchOfThree(t *testing.T) {
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not on PATH")
	}

	dir := t.TempDir()
	unformatted := "package main\n\nfunc f()   {\n}\n"

	var paths []string
	for _, name := range []string{"a.go", "b.go", "c.go"} {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, []byte(unformatted), 0644))
		paths = append(paths, p)
	}

	res, err := Format{}.Call(context.Background(),
		map[string]any{"paths": []any{paths[0], paths[1], paths[2]}},
		&Context{})
	require.NoError(t, err)
	require.False(t, res.IsError, "unexpected error: %s", res.Text)
	require.Contains(t, res.Text, "formatted 3/3")
	for _, p := range paths {
		require.Contains(t, res.Text, filepath.Base(p))
	}
}

func TestFormat_PartialFailure(t *testing.T) {
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not on PATH")
	}
	// Only test partial-failure if python formatter also absent (so .py fails).
	if _, err := exec.LookPath("black"); err == nil {
		t.Skip("black is on PATH; skipping partial-failure test")
	}
	if _, err := exec.LookPath("ruff"); err == nil {
		t.Skip("ruff is on PATH; skipping partial-failure test")
	}

	dir := t.TempDir()
	goFile := filepath.Join(dir, "ok.go")
	pyFile := filepath.Join(dir, "fail.py")
	require.NoError(t, os.WriteFile(goFile, []byte("package main\n\nfunc f()   {\n}\n"), 0644))
	require.NoError(t, os.WriteFile(pyFile, []byte("x=1\n"), 0644))

	res, err := Format{}.Call(context.Background(),
		map[string]any{"paths": []any{goFile, pyFile}},
		&Context{})
	require.NoError(t, err)
	// 1 succeeded (go), 1 failed (py) → IsError=false (partial success still returns a result)
	require.False(t, res.IsError, "partial success must not be IsError: %s", res.Text)
	require.Contains(t, res.Text, "formatted 1/2")
	require.Contains(t, res.Text, "FAILED")
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
