package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClaudeMd_MergesFromCwdUpward(t *testing.T) {
	root := t.TempDir()
	mid := filepath.Join(root, "a")
	leaf := filepath.Join(mid, "b")
	require.NoError(t, os.MkdirAll(leaf, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# root rules\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(leaf, "CLAUDE.md"), []byte("# leaf rules\n"), 0o644))

	got, err := LoadClaudeMd(leaf, root)
	require.NoError(t, err)
	require.Contains(t, got, "root rules")
	require.Contains(t, got, "leaf rules")
	// leaf rules win when read order matters: emit root first, then leaf.
	require.True(t, indexOf(got, "root") < indexOf(got, "leaf"))
}

func TestClaudeMd_NoneFound(t *testing.T) {
	dir := t.TempDir()
	got, err := LoadClaudeMd(dir, dir)
	require.NoError(t, err)
	require.Empty(t, got)
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
