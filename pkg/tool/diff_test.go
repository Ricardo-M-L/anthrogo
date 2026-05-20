package tool

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiff_WorkingTreeChanges(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	dir := initGitRepo(t)

	// Write a file and commit it.
	f := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(f, []byte("hello\n"), 0644))
	gitExec(t, dir, "add", "file.txt")
	gitExec(t, dir, "commit", "-m", "init")

	// Modify the file — unstaged change.
	require.NoError(t, os.WriteFile(f, []byte("hello\nworld\n"), 0644))

	tcx := &Context{Cwd: dir}
	res, err := Diff{}.Call(context.Background(), map[string]any{}, tcx)
	require.NoError(t, err)
	require.False(t, res.IsError, "unexpected error: %s", res.Text)
	require.Contains(t, res.Text, "+++ b/file.txt")
	require.Contains(t, res.Text, "+world")
}

func TestDiff_Cached(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	dir := initGitRepo(t)

	f := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(f, []byte("line1\n"), 0644))
	gitExec(t, dir, "add", "file.txt")
	gitExec(t, dir, "commit", "-m", "init")

	require.NoError(t, os.WriteFile(f, []byte("line1\nline2\n"), 0644))
	gitExec(t, dir, "add", "file.txt")

	tcx := &Context{Cwd: dir}
	res, err := Diff{}.Call(context.Background(), map[string]any{"cached": true}, tcx)
	require.NoError(t, err)
	require.False(t, res.IsError, "unexpected error: %s", res.Text)
	require.Contains(t, res.Text, "+line2")
}

func TestDiff_Stat(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	dir := initGitRepo(t)

	f := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(f, []byte("a\n"), 0644))
	gitExec(t, dir, "add", "file.txt")
	gitExec(t, dir, "commit", "-m", "init")
	require.NoError(t, os.WriteFile(f, []byte("a\nb\n"), 0644))

	tcx := &Context{Cwd: dir}
	res, err := Diff{}.Call(context.Background(), map[string]any{"stat": true}, tcx)
	require.NoError(t, err)
	require.False(t, res.IsError, "unexpected error: %s", res.Text)
	// --stat output contains the filename and insertion count
	require.Contains(t, res.Text, "file.txt")
}

func TestDiff_NoDiff_Empty(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	dir := initGitRepo(t)
	f := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(f, []byte("clean\n"), 0644))
	gitExec(t, dir, "add", "file.txt")
	gitExec(t, dir, "commit", "-m", "init")

	tcx := &Context{Cwd: dir}
	res, err := Diff{}.Call(context.Background(), map[string]any{}, tcx)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Empty(t, res.Text)
}

// initGitRepo creates a temp dir with a git repo configured for testing.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitExec(t, dir, "init")
	gitExec(t, dir, "config", "user.email", "test@test.com")
	gitExec(t, dir, "config", "user.name", "Test")
	return dir
}

// gitExec runs a git command in dir and fails the test on error.
func gitExec(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %s", args, out)
	}
}
