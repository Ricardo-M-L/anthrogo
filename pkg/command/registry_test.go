package command

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubCmd struct {
	name string
}

func (s stubCmd) Name() string                                      { return s.name }
func (s stubCmd) Aliases() []string                                 { return nil }
func (s stubCmd) Description() string                               { return "stub" }
func (s stubCmd) Type() Type                                        { return TypeLocal }
func (s stubCmd) Run(context.Context, string, Host) (Result, error) { return Result{Text: "ok"}, nil }

func TestRegistry_RegisterAndLookup(t *testing.T) {
	r := NewRegistry()
	r.Register(stubCmd{name: "/help"})
	r.Register(stubCmd{name: "/tools"})
	c, ok := r.Lookup("/help")
	require.True(t, ok)
	require.Equal(t, "/help", c.Name())
	_, ok = r.Lookup("/missing")
	require.False(t, ok)
}

func TestRegistry_FuzzyMatch(t *testing.T) {
	r := NewRegistry()
	r.Register(stubCmd{name: "/clear"})
	r.Register(stubCmd{name: "/compact"})
	r.Register(stubCmd{name: "/cwd"})
	matches := r.Fuzzy("c")
	require.Len(t, matches, 3)
	matches = r.Fuzzy("cw")
	require.Len(t, matches, 1)
	require.Equal(t, "/cwd", matches[0].Name())
}
