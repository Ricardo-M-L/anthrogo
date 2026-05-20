package builtins

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/subagent"
)

func TestSubagents_NoRegistry(t *testing.T) {
	h := newFakeHost()
	// h.subagents is nil
	res, err := (Subagents{}).Run(context.Background(), "", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "no subagent registry configured")
}

func TestSubagents_ListEmpty(t *testing.T) {
	h := newFakeHost()
	h.subagents = subagent.NewRegistry()
	res, err := (Subagents{}).Run(context.Background(), "", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "(no subagent types)")
}

func TestSubagents_ListDefault(t *testing.T) {
	h := newFakeHost()
	h.subagents = subagent.DefaultRegistry()
	res, err := (Subagents{}).Run(context.Background(), "", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "general-purpose")
}

func TestSubagents_Show(t *testing.T) {
	h := newFakeHost()
	h.subagents = subagent.DefaultRegistry()
	res, err := (Subagents{}).Run(context.Background(), "show general-purpose", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "name: general-purpose")
	require.Contains(t, res.Text, "description:")
}

func TestSubagents_ShowNotFound(t *testing.T) {
	h := newFakeHost()
	h.subagents = subagent.DefaultRegistry()
	res, err := (Subagents{}).Run(context.Background(), "show nonexistent", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "not found")
}

func TestSubagents_ShowToolAllowlist(t *testing.T) {
	h := newFakeHost()
	reg := subagent.NewRegistry()
	reg.Register(subagent.Spec{
		Name:          "my-type",
		Description:   "A type with allowlist",
		ToolAllowlist: []string{"Read", "Grep"},
	})
	h.subagents = reg
	res, err := (Subagents{}).Run(context.Background(), "show my-type", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "tool_allowlist: Read, Grep")
}

func TestSubagents_ShowNoAllowlist(t *testing.T) {
	h := newFakeHost()
	reg := subagent.NewRegistry()
	reg.Register(subagent.Spec{
		Name:        "my-type",
		Description: "A type without allowlist",
	})
	h.subagents = reg
	res, err := (Subagents{}).Run(context.Background(), "show my-type", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "tool_allowlist: (inherit all)")
}

func TestSubagents_Reload(t *testing.T) {
	h := newFakeHost()
	h.subagents = subagent.DefaultRegistry()
	// Empty roots — reload should succeed and keep general-purpose.
	res, err := (Subagents{HomeRoot: "", CwdRoot: ""}).Run(context.Background(), "reload", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "reloaded subagent registry")
	// general-purpose should survive reload.
	_, ok := h.subagents.Get("general-purpose")
	require.True(t, ok, "general-purpose must survive reload")
}

func TestSubagents_UsageOnUnknownArgs(t *testing.T) {
	h := newFakeHost()
	h.subagents = subagent.DefaultRegistry()
	res, err := (Subagents{}).Run(context.Background(), "unknowncmd", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "usage")
}
