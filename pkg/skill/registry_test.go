package skill

import (
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
