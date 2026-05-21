package skill

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestRegistry_InstallFromURL_RejectsOversize(t *testing.T) {
	// Serve more than 50 MB of data to trigger the size limit.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		// Write slightly more than maxSkillArchiveBytes bytes.
		chunk := make([]byte, 1024*1024) // 1 MB
		for i := 0; i < 52; i++ {       // 52 MB total
			fmt.Fprintf(w, "%s", chunk)
		}
	}))
	defer srv.Close()

	dest := t.TempDir()
	r := NewRegistry(nil)
	_, _, err := r.Install(srv.URL+"/skill.tar.gz", dest)
	require.Error(t, err)
	require.Contains(t, err.Error(), "50MB limit")

	// No skill should have been installed.
	require.Len(t, r.List(), 0)
}

func TestRegistry_InstallFromDir_RejectsSymlinks(t *testing.T) {
	src := t.TempDir()
	// Write a valid SKILL.md so the name check passes.
	require.NoError(t, os.WriteFile(
		filepath.Join(src, "SKILL.md"),
		[]byte("---\nname: sym-skill\ndescription: test\n---\nbody\n"),
		0o644,
	))
	// Create a symlink inside the skill directory.
	require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(src, "evil-link")))

	dest := t.TempDir()
	r := NewRegistry(nil)
	_, _, err := r.Install(src, dest)
	require.Error(t, err)
	require.Contains(t, err.Error(), "symlink")

	// No skill should have been installed.
	_, ok := r.Get("sym-skill")
	require.False(t, ok)
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
