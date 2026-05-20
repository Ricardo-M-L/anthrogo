package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_ListSorted(t *testing.T) {
	r := NewRegistry([]Plugin{
		{Name: "zebra"},
		{Name: "alpha"},
		{Name: "mango"},
	})
	list := r.List()
	require.Len(t, list, 3)
	assert.Equal(t, "alpha", list[0].Name)
	assert.Equal(t, "mango", list[1].Name)
	assert.Equal(t, "zebra", list[2].Name)
}

func TestRegistry_GetMiss(t *testing.T) {
	r := NewRegistry(nil)
	_, ok := r.Get("nope")
	assert.False(t, ok)
}

func TestRegistry_Reload(t *testing.T) {
	home := testdata("valid-home")
	r := NewRegistry(nil)
	assert.Empty(t, r.List())

	warnings, err := r.Reload(home, "")
	require.NoError(t, err)
	_ = warnings
	list := r.List()
	require.Len(t, list, 1)
	assert.Equal(t, "git-tools", list[0].Name)
}

func TestRegistry_InstallAndRemove(t *testing.T) {
	src := testdata("valid-home/git-tools")
	destRoot := t.TempDir()

	r := NewRegistry(nil)
	got, _, err := r.Install(src, destRoot)
	require.NoError(t, err)
	assert.Equal(t, "git-tools", got.Name)

	// Should now appear in List()
	list := r.List()
	require.Len(t, list, 1)
	assert.Equal(t, "git-tools", list[0].Name)

	// plugin.yaml must exist at destination
	_, err = os.Stat(filepath.Join(destRoot, "git-tools", "plugin.yaml"))
	require.NoError(t, err)

	// Double install should fail
	_, _, err = r.Install(src, destRoot)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// Remove
	err = r.Remove("git-tools", destRoot)
	require.NoError(t, err)
	assert.Empty(t, r.List())

	// Directory should be gone
	_, err = os.Stat(filepath.Join(destRoot, "git-tools"))
	assert.True(t, os.IsNotExist(err))
}

func TestRegistry_InstallBadSrc(t *testing.T) {
	r := NewRegistry(nil)
	_, _, err := r.Install("/nonexistent-src-xyz", t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no plugin.yaml")
}

func TestRegistry_RemoveMissing(t *testing.T) {
	r := NewRegistry(nil)
	err := r.Remove("nope", t.TempDir())
	require.Error(t, err)
}

func TestCopyDir_PreservesModes(t *testing.T) {
	src := testdata("valid-home/git-tools")
	dst := t.TempDir()
	err := copyDir(src, filepath.Join(dst, "git-tools"))
	require.NoError(t, err)

	// audit.sh should be executable
	info, err := os.Stat(filepath.Join(dst, "git-tools", "hooks", "audit.sh"))
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&0o111, "audit.sh should be executable")
}
