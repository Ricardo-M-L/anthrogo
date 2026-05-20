package builtins

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// sessionsFakeHost is a minimal host used only by the sessions tests.
// It overrides Cwd() to return a temp dir that acts as a fake "project dir"
// (i.e. the dir that session.ProjectDir would resolve to). Because session.ProjectDir
// hashes the cwd and places files under ~/.anthrogo/projects/<hash>/, we can't
// directly control its output path from outside. Instead, we unit-test listSessions
// and showSession directly with a synthetic directory.

func TestSessions_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	res, err := listSessions(dir)
	require.NoError(t, err)
	require.Equal(t, "(no sessions yet)", res.Text)
}

func TestSessions_ListsJSONLs(t *testing.T) {
	dir := t.TempDir()
	// Create two .jsonl files.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "aaa-111.jsonl"), []byte(`{}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bbb-222.jsonl"), []byte(`{}{}`), 0o644))
	// A non-.jsonl file — should be ignored.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notajsonl.txt"), []byte(`x`), 0o644))

	res, err := listSessions(dir)
	require.NoError(t, err)
	require.Contains(t, res.Text, "aaa-111")
	require.Contains(t, res.Text, "bbb-222")
	require.NotContains(t, res.Text, "notajsonl")
	// Header present.
	require.Contains(t, res.Text, "ID")
	require.Contains(t, res.Text, "Modified")
}

func TestSessions_ShowKnown(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "abc-123-def.jsonl"), []byte(`{}`), 0o644))

	res, err := showSession(dir, "abc-123")
	require.NoError(t, err)
	require.Contains(t, res.Text, "abc-123-def.jsonl")
	require.Contains(t, res.Text, "path:")
	require.Contains(t, res.Text, "modified:")
	require.Contains(t, res.Text, "size:")
}

func TestSessions_ShowUnknown(t *testing.T) {
	dir := t.TempDir()
	res, err := showSession(dir, "no-such-prefix")
	require.NoError(t, err)
	require.Contains(t, res.Text, "no match")
}

func TestSessions_ShowAmbiguous(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "abc-aaa.jsonl"), []byte(`{}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "abc-bbb.jsonl"), []byte(`{}`), 0o644))

	res, err := showSession(dir, "abc-")
	require.NoError(t, err)
	require.Contains(t, res.Text, "ambiguous")
}

func TestSessions_UsageMessage(t *testing.T) {
	h := newFakeHost()
	h.cwd = t.TempDir() // non-hash path — session.ProjectDir will create a subdir
	res, err := (Sessions{}).Run(context.Background(), "badcmd", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "usage")
}
