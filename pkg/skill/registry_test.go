package skill

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegistry_GetReturnsFalseForMissing(t *testing.T) {
	r := NewRegistry(nil)
	_, ok := r.Get("nonexistent")
	require.False(t, ok)
}

func TestRegistry_ListSortedByName(t *testing.T) {
	r := NewRegistry([]Skill{
		{Name: "z-skill", Description: "last"},
		{Name: "a-skill", Description: "first"},
		{Name: "m-skill", Description: "middle"},
	})
	list := r.List()
	require.Len(t, list, 3)
	require.Equal(t, "a-skill", list[0].Name)
	require.Equal(t, "m-skill", list[1].Name)
	require.Equal(t, "z-skill", list[2].Name)
}

func TestRegistry_InstallFromLocalDir(t *testing.T) {
	src := t.TempDir()
	err := os.WriteFile(
		filepath.Join(src, "SKILL.md"),
		[]byte("---\nname: test-skill\ndescription: A test skill\n---\n\nbody\n"),
		0o644,
	)
	require.NoError(t, err)

	dest := t.TempDir()
	r := NewRegistry(nil)
	sk, _, err := r.Install(src, dest)
	require.NoError(t, err)
	require.Equal(t, "test-skill", sk.Name)

	_, err = os.Stat(filepath.Join(dest, "test-skill", "SKILL.md"))
	require.NoError(t, err)

	// Skill is accessible from registry.
	got, ok := r.Get("test-skill")
	require.True(t, ok)
	require.Equal(t, "test-skill", got.Name)
}

func TestRegistry_InstallFromLocalDir_AlreadyExists(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(src, "SKILL.md"),
		[]byte("---\nname: test-skill\ndescription: A test skill\n---\n\nbody\n"),
		0o644,
	))
	dest := t.TempDir()
	r := NewRegistry(nil)
	_, _, err := r.Install(src, dest)
	require.NoError(t, err)

	// Second install should fail.
	_, _, err = r.Install(src, dest)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")
}

func TestRegistry_InstallFromLocalDir_NoSkillMD(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	r := NewRegistry(nil)
	_, _, err := r.Install(src, dest)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no SKILL.md")
}

func TestRegistry_InstallFromLocalDir_EmptyName(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(src, "SKILL.md"),
		[]byte("---\nname: \ndescription: A test\n---\n\nbody\n"),
		0o644,
	))
	dest := t.TempDir()
	r := NewRegistry(nil)
	_, _, err := r.Install(src, dest)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty skill name")
}

func TestRegistry_Reload_ReplacesAtomically(t *testing.T) {
	r := NewRegistry([]Skill{
		{Name: "old-skill", Description: "old"},
	})
	_, ok := r.Get("old-skill")
	require.True(t, ok)

	// Reload from valid-home testdata
	homeRoot := filepath.Join("testdata", "valid-home")
	warnings, err := r.Reload(homeRoot, "")
	require.NoError(t, err)
	require.Empty(t, warnings)

	// Old skill should be gone
	_, ok = r.Get("old-skill")
	require.False(t, ok)

	// New skill from testdata should be present
	sk, ok := r.Get("git-flow")
	require.True(t, ok)
	require.Equal(t, "git-flow", sk.Name)
}
