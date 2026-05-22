package tool

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"os"

	"github.com/stretchr/testify/require"
)

func TestGit_StatusInRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	dir := t.TempDir()
	gitExec(t, dir, "init")
	gitExec(t, dir, "config", "user.email", "test@test.com")
	gitExec(t, dir, "config", "user.name", "Test")

	// Write an untracked file.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hi"), 0644))

	tcx := &Context{Cwd: dir}
	res, err := Git{}.Call(context.Background(), map[string]any{"command": "status"}, tcx)
	require.NoError(t, err)
	require.False(t, res.IsError, "unexpected error: %s", res.Text)
	require.Contains(t, res.Text, "new.txt")
}

func TestGit_RejectsShellMetacharsInArgs(t *testing.T) {
	res, err := Git{}.Call(context.Background(), map[string]any{
		"command": "log",
		"args":    "; rm -rf /",
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "metacharacter")
}

func TestGit_RejectsUnknownSubcommand(t *testing.T) {
	for _, bad := range []string{"push", "commit", "reset", "checkout", "merge"} {
		res, err := Git{}.Call(context.Background(), map[string]any{"command": bad}, nil)
		require.NoError(t, err, "command=%s", bad)
		require.True(t, res.IsError, "expected error for command=%s", bad)
		require.Contains(t, res.Text, "not allowed")
	}
}

func TestGit_AllowedCommands(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	dir := t.TempDir()
	gitExec(t, dir, "init")
	gitExec(t, dir, "config", "user.email", "test@test.com")
	gitExec(t, dir, "config", "user.name", "Test")

	tcx := &Context{Cwd: dir}

	for _, cmd := range []string{"status", "log", "branch", "remote"} {
		res, err := Git{}.Call(context.Background(), map[string]any{"command": cmd}, tcx)
		require.NoError(t, err, "command=%s", cmd)
		// These should not return "not allowed" errors.
		if res.IsError {
			// log in empty repo is expected to fail with "does not have any commits" or similar
			require.NotContains(t, res.Text, "not allowed", "command=%s", cmd)
		}
	}
}
