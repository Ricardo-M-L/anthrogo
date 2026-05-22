package tool

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubTool struct {
	DefaultPermission
	name   string
	schema map[string]any
}

func (s stubTool) Name() string                         { return s.name }
func (s stubTool) Description(context.Context) string   { return "desc:" + s.name }
func (s stubTool) Schema() map[string]any               { return s.schema }
func (s stubTool) UserFacingName(map[string]any) string { return s.name }
func (s stubTool) IsReadOnly() bool                     { return true }
func (s stubTool) IsConcurrencySafe() bool              { return true }
func (s stubTool) Call(context.Context, map[string]any, *Context) (Result, error) {
	return Result{Text: "ok", ForLLM: "ok"}, nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	r.Register(stubTool{name: "A", schema: map[string]any{"type": "object"}})
	got, ok := r.Get("A")
	require.True(t, ok)
	require.Equal(t, "A", got.Name())
}

func TestRegistry_All_PreservesOrder(t *testing.T) {
	r := NewRegistry()
	r.Register(stubTool{name: "A"})
	r.Register(stubTool{name: "B"})
	r.Register(stubTool{name: "C"})
	names := []string{}
	for _, t := range r.All() {
		names = append(names, t.Name())
	}
	require.Equal(t, []string{"A", "B", "C"}, names)
}

func TestRegistry_Register_LogsAndContinuesOnDuplicate(t *testing.T) {
	r := NewRegistry()
	r.Register(stubTool{name: "A"})
	// Second registration should NOT panic; original tool should be preserved.
	require.NotPanics(t, func() { r.Register(stubTool{name: "A"}) })
	all := r.All()
	require.Len(t, all, 1, "duplicate should be silently dropped")
}

func TestRegistry_TryRegister_ReturnsErrorOnDuplicate(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, r.TryRegister(stubTool{name: "X"}))
	err := r.TryRegister(stubTool{name: "X"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already registered")
	// Registry must still contain exactly one entry.
	require.Len(t, r.All(), 1)
}

func TestRegistry_RemoveByPrefix(t *testing.T) {
	t.Run("empty registry returns 0", func(t *testing.T) {
		r := NewRegistry()
		require.Equal(t, 0, r.RemoveByPrefix("mcp__"))
	})

	t.Run("removes only matching names", func(t *testing.T) {
		r := NewRegistry()
		r.Register(stubTool{name: "Read"})
		r.Register(stubTool{name: "mcp__fs__read"})
		r.Register(stubTool{name: "mcp__fs__write"})
		r.Register(stubTool{name: "Bash"})
		n := r.RemoveByPrefix("mcp__")
		require.Equal(t, 2, n)
		_, ok1 := r.Get("mcp__fs__read")
		_, ok2 := r.Get("mcp__fs__write")
		require.False(t, ok1)
		require.False(t, ok2)
	})

	t.Run("preserves order of non-matching tools", func(t *testing.T) {
		r := NewRegistry()
		r.Register(stubTool{name: "A"})
		r.Register(stubTool{name: "mcp__x__y"})
		r.Register(stubTool{name: "B"})
		r.Register(stubTool{name: "C"})
		r.RemoveByPrefix("mcp__")
		names := []string{}
		for _, t := range r.All() {
			names = append(names, t.Name())
		}
		require.Equal(t, []string{"A", "B", "C"}, names)
	})
}
