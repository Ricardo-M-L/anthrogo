package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRead_ReadsFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "hello.txt")
	require.NoError(t, os.WriteFile(p, []byte("line1\nline2\nline3\n"), 0o644))

	got, err := (&Read{}).Call(context.Background(), map[string]any{"file_path": p}, &Context{})
	require.NoError(t, err)
	require.Contains(t, got.Text, "line1")
	require.Contains(t, got.Text, "line2")
}

func TestRead_RespectsOffsetLimit(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "many.txt")
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("line\n")
	}
	require.NoError(t, os.WriteFile(p, []byte(sb.String()), 0o644))

	got, err := (&Read{}).Call(context.Background(), map[string]any{
		"file_path": p, "offset": 10.0, "limit": 5.0,
	}, &Context{})
	require.NoError(t, err)
	require.Equal(t, 5, strings.Count(got.Text, "\n"))
}

func TestRead_MissingFile_ReturnsError(t *testing.T) {
	got, err := (&Read{}).Call(context.Background(), map[string]any{"file_path": "/nonexistent/foo"}, &Context{})
	require.NoError(t, err) // tool returns Result.IsError=true rather than Go error
	require.True(t, got.IsError)
}
