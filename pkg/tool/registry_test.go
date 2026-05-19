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

func (s stubTool) Name() string                        { return s.name }
func (s stubTool) Description(context.Context) string  { return "desc:" + s.name }
func (s stubTool) Schema() map[string]any              { return s.schema }
func (s stubTool) UserFacingName(map[string]any) string { return s.name }
func (s stubTool) IsReadOnly() bool                    { return true }
func (s stubTool) IsConcurrencySafe() bool             { return true }
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

func TestRegistry_DuplicatePanics(t *testing.T) {
	r := NewRegistry()
	r.Register(stubTool{name: "A"})
	require.Panics(t, func() { r.Register(stubTool{name: "A"}) })
}
