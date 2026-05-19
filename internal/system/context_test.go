package system

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGitStatus_NonRepo_ReturnsEmpty(t *testing.T) {
	got, err := GitStatusSnapshot(t.TempDir())
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestGitStatus_Repo_IncludesBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"-c", "user.name=t", "-c", "user.email=t@x", "commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		require.NoError(t, cmd.Run())
	}
	require.NoError(t, exec.Command("touch", filepath.Join(dir, "new.txt")).Run())
	got, err := GitStatusSnapshot(dir)
	require.NoError(t, err)
	require.Contains(t, got, "Current branch: main")
}
