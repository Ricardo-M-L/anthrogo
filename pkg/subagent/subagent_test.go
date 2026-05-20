package subagent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultRegistry_ContainsGeneralPurpose(t *testing.T) {
	r := DefaultRegistry()
	s, ok := r.Get("general-purpose")
	require.True(t, ok, "expected general-purpose to be registered")
	require.Equal(t, "general-purpose", s.Name)
	require.NotEmpty(t, s.Description)
	require.NotEmpty(t, s.SystemPromptSuffix)
}

func TestRegistry_GetUnknown_ReturnsFalse(t *testing.T) {
	r := DefaultRegistry()
	_, ok := r.Get("nonexistent-type")
	require.False(t, ok)
}

func TestRegistry_List_Sorted(t *testing.T) {
	r := NewRegistry()
	r.Register(Spec{Name: "zebra", Description: "z"})
	r.Register(Spec{Name: "alpha", Description: "a"})
	r.Register(Spec{Name: "middle", Description: "m"})

	list := r.List()
	require.Len(t, list, 3)
	require.Equal(t, "alpha", list[0].Name)
	require.Equal(t, "middle", list[1].Name)
	require.Equal(t, "zebra", list[2].Name)
}

func TestRegistry_List_Empty(t *testing.T) {
	r := NewRegistry()
	require.Empty(t, r.List())
}

func TestRegistry_Register_Get(t *testing.T) {
	r := NewRegistry()
	spec := Spec{
		Name:          "test-type",
		Description:   "for testing",
		ToolAllowlist: []string{"Read", "Grep"},
	}
	r.Register(spec)
	got, ok := r.Get("test-type")
	require.True(t, ok)
	require.Equal(t, spec, got)
}
