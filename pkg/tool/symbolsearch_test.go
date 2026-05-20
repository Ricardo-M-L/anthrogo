package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSymbolSearch_GoFunc(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package x\n\nfunc Foo() {}\n"), 0o644))

	res, err := SymbolSearch{}.Call(context.Background(), map[string]any{
		"name": "Foo",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "a.go")
	require.Contains(t, res.Text, "func Foo")
}

func TestSymbolSearch_GoTypeKindFilter(t *testing.T) {
	dir := t.TempDir()
	src := "package x\n\nfunc Foo() {}\n\ntype Foo struct{}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte(src), 0o644))

	// kind=type should return only the type definition
	res, err := SymbolSearch{}.Call(context.Background(), map[string]any{
		"name": "Foo",
		"kind": "type",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "type Foo")
	// Should NOT contain the func line
	require.NotContains(t, res.Text, "func Foo")
}

func TestSymbolSearch_GoFuncKindFilter(t *testing.T) {
	dir := t.TempDir()
	src := "package x\n\nfunc Foo() {}\n\ntype Foo struct{}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte(src), 0o644))

	// kind=func should return only the function definition
	res, err := SymbolSearch{}.Call(context.Background(), map[string]any{
		"name": "Foo",
		"kind": "func",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "func Foo")
	require.NotContains(t, res.Text, "type Foo")
}

func TestSymbolSearch_GoConst(t *testing.T) {
	dir := t.TempDir()
	src := "package x\n\nconst MyConst = 42\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte(src), 0o644))

	res, err := SymbolSearch{}.Call(context.Background(), map[string]any{
		"name": "MyConst",
		"kind": "const",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "MyConst")
}

func TestSymbolSearch_GoVar(t *testing.T) {
	dir := t.TempDir()
	src := "package x\n\nvar MyVar = 42\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte(src), 0o644))

	res, err := SymbolSearch{}.Call(context.Background(), map[string]any{
		"name": "MyVar",
		"kind": "var",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "MyVar")
}

func TestSymbolSearch_JS(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.ts"), []byte("function Foo() {}\n"), 0o644))

	res, err := SymbolSearch{}.Call(context.Background(), map[string]any{
		"name": "Foo",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "a.ts")
	require.Contains(t, res.Text, "Foo")
}

func TestSymbolSearch_JSKindFunc(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.ts"), []byte("function Foo() {}\n"), 0o644))

	res, err := SymbolSearch{}.Call(context.Background(), map[string]any{
		"name": "Foo",
		"kind": "func",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "a.ts")
}

func TestSymbolSearch_Python(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.py"), []byte("def foo():\n    pass\n"), 0o644))

	res, err := SymbolSearch{}.Call(context.Background(), map[string]any{
		"name": "foo",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "a.py")
	require.Contains(t, res.Text, "def foo")
}

func TestSymbolSearch_Rust(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.rs"), []byte("fn bar() {}\n"), 0o644))

	res, err := SymbolSearch{}.Call(context.Background(), map[string]any{
		"name": "bar",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "a.rs")
	require.Contains(t, res.Text, "fn bar")
}

func TestSymbolSearch_Ruby(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.rb"), []byte("def baz\nend\n"), 0o644))

	res, err := SymbolSearch{}.Call(context.Background(), map[string]any{
		"name": "baz",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "a.rb")
	require.Contains(t, res.Text, "def baz")
}

func TestSymbolSearch_SkipsVendor(t *testing.T) {
	dir := t.TempDir()
	// main.go with func Foo
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc Foo() {}\n"), 0o644))
	// vendor/foo.go also with func Foo — should be skipped
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "vendor"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "vendor", "foo.go"), []byte("package x\n\nfunc Foo() {}\n"), 0o644))

	res, err := SymbolSearch{}.Call(context.Background(), map[string]any{
		"name": "Foo",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "main.go")
	require.NotContains(t, res.Text, "vendor")
}

func TestSymbolSearch_SkipsNodeModules(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.js"), []byte("function Foo() {}\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "node_modules", "pkg"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "node_modules", "pkg", "index.js"), []byte("function Foo() {}\n"), 0o644))

	res, err := SymbolSearch{}.Call(context.Background(), map[string]any{
		"name": "Foo",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "index.js")
	require.NotContains(t, res.Text, "node_modules")
}

func TestSymbolSearch_NoMatch(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package x\n\nfunc Bar() {}\n"), 0o644))

	res, err := SymbolSearch{}.Call(context.Background(), map[string]any{
		"name": "Foo",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Equal(t, "no matches", res.Text)
}

func TestSymbolSearch_RequiresName(t *testing.T) {
	res, err := SymbolSearch{}.Call(context.Background(), map[string]any{
		"path": t.TempDir(),
	}, &Context{})
	require.NoError(t, err)
	require.True(t, res.IsError)
}

func TestSymbolSearch_GoMethod(t *testing.T) {
	dir := t.TempDir()
	src := "package x\n\ntype S struct{}\n\nfunc (s *S) Foo() error { return nil }\n\nfunc Foo() {}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte(src), 0o644))

	// kind=method should match only the receiver-bound Foo, not the bare func Foo.
	res, err := SymbolSearch{}.Call(context.Background(), map[string]any{
		"name": "Foo",
		"kind": "method",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "a.go")
	// The matched line should contain the receiver.
	require.Contains(t, res.Text, "*S")
	// Bare func line must not appear.
	lines := strings.Split(strings.TrimSpace(res.Text), "\n")
	require.Len(t, lines, 1, "expected exactly one hit (the method)")
}

func TestSymbolSearch_GoFuncMatchesMethodToo(t *testing.T) {
	dir := t.TempDir()
	src := "package x\n\ntype S struct{}\n\nfunc (s *S) Foo() {}\n\nfunc Foo() {}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte(src), 0o644))

	// kind=func should match both the method and the bare func.
	res, err := SymbolSearch{}.Call(context.Background(), map[string]any{
		"name": "Foo",
		"kind": "func",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	lines := strings.Split(strings.TrimSpace(res.Text), "\n")
	require.Len(t, lines, 2, "expected two hits (method + bare func)")
}

func TestSymbolSearch_OutputFormat(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package x\n\nfunc Hello() {}\n"), 0o644))

	res, err := SymbolSearch{}.Call(context.Background(), map[string]any{
		"name": "Hello",
		"path": dir,
	}, &Context{})
	require.NoError(t, err)
	// Output format: path:line: content
	lines := strings.Split(strings.TrimSpace(res.Text), "\n")
	require.NotEmpty(t, lines)
	// Each line should have path:line: format
	for _, line := range lines {
		if line == "" {
			continue
		}
		require.Contains(t, line, ":")
	}
}
