package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReferences_FindsAllOccurrences(t *testing.T) {
	dir := t.TempDir()
	// a.go references Foo three times.
	src := "package x\n\n// Foo is great\nfunc Foo() { Foo() }\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte(src), 0o644))

	res, err := References{}.Call(context.Background(), map[string]any{
		"name": "Foo",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	// Should have multiple lines (one per match per line).
	lines := strings.Split(strings.TrimSpace(res.Text), "\n")
	require.Greater(t, len(lines), 1, "expected multiple reference lines, got: %s", res.Text)
}

func TestReferences_WordBoundary(t *testing.T) {
	dir := t.TempDir()
	// "Foo" should match; "FooBar" and "BarFoo" should NOT match as standalone "Foo".
	src := "package x\n\n// Foo is fine\n// FooBar should not match\n// BarFoo should not match\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte(src), 0o644))

	res, err := References{}.Call(context.Background(), map[string]any{
		"name": "Foo",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)

	// Count lines matching each case.
	lines := strings.Split(strings.TrimSpace(res.Text), "\n")
	fooOnlyCount := 0
	fooBarCount := 0
	barFooCount := 0
	for _, l := range lines {
		if strings.Contains(l, "Foo is fine") {
			fooOnlyCount++
		}
		if strings.Contains(l, "FooBar") {
			fooBarCount++
		}
		if strings.Contains(l, "BarFoo") {
			barFooCount++
		}
	}
	require.Equal(t, 1, fooOnlyCount, "expected 1 line with 'Foo is fine'")
	require.Equal(t, 0, fooBarCount, "FooBar should not match word-boundary Foo")
	require.Equal(t, 0, barFooCount, "BarFoo should not match word-boundary Foo")
}

func TestReferences_BinaryFileSkipped(t *testing.T) {
	dir := t.TempDir()
	// Write a "binary" file with NUL bytes containing the word "Foo".
	binary := []byte("Foo\x00binary\x00data")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "binary.bin"), binary, 0o644))
	// Write a text file that also contains "Foo".
	require.NoError(t, os.WriteFile(filepath.Join(dir, "text.go"), []byte("package x\n// Foo\n"), 0o644))

	res, err := References{}.Call(context.Background(), map[string]any{
		"name": "Foo",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	// Should find the text file.
	require.Contains(t, res.Text, "text.go")
	// Should NOT find the binary file.
	require.NotContains(t, res.Text, "binary.bin")
}

func TestReferences_SkipsVendor(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\n// uses Foo\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "vendor"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "vendor", "lib.go"), []byte("package lib\n\n// Foo defined here\n"), 0o644))

	res, err := References{}.Call(context.Background(), map[string]any{
		"name": "Foo",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "main.go")
	require.NotContains(t, res.Text, "vendor")
}

func TestReferences_NoMatch(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package x\n// nothing here\n"), 0o644))

	res, err := References{}.Call(context.Background(), map[string]any{
		"name": "Nonexistent",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Equal(t, "no references found", res.Text)
}

func TestReferences_RequiresName(t *testing.T) {
	res, err := References{}.Call(context.Background(), map[string]any{
		"path": t.TempDir(),
	}, &Context{})
	require.NoError(t, err)
	require.True(t, res.IsError)
}

func TestReferences_OutputFormat(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package x\n// Foo reference\n"), 0o644))

	res, err := References{}.Call(context.Background(), map[string]any{
		"name": "Foo",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	// Output format: path:line:col: content
	lines := strings.Split(strings.TrimSpace(res.Text), "\n")
	require.NotEmpty(t, lines)
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Should have at least 3 colons (path:line:col: text)
		parts := strings.SplitN(line, ":", 4)
		require.GreaterOrEqual(t, len(parts), 3, "line should have path:line:col format: %s", line)
	}
}

func TestReferences_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package x\n// Foo in a\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.go"), []byte("package x\n// Foo in b\n"), 0o644))

	res, err := References{}.Call(context.Background(), map[string]any{
		"name": "Foo",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "a.go")
	require.Contains(t, res.Text, "b.go")
}

func TestReferences_LargeFileSkipped(t *testing.T) {
	dir := t.TempDir()
	// Create a file larger than 1MB.
	bigFile := filepath.Join(dir, "big.go")
	f, err := os.Create(bigFile)
	require.NoError(t, err)
	// Write > 1MB of content with "Foo" inside.
	chunk := []byte("// line with Foo in it\n")
	total := 0
	for total < 1024*1024+100 {
		_, _ = f.Write(chunk)
		total += len(chunk)
	}
	f.Close()

	// Also write a small file.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "small.go"), []byte("package x\n// Foo here\n"), 0o644))

	res, err := References{}.Call(context.Background(), map[string]any{
		"name": "Foo",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	// small.go should be found
	require.Contains(t, res.Text, "small.go")
	// big.go should be skipped (> 1MB)
	require.NotContains(t, res.Text, "big.go")
}
