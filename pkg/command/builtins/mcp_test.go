package builtins

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/internal/mcp"
)

func TestMCP_NoManager(t *testing.T) {
	h := newFakeHost()
	res, err := (MCP{}).Run(context.Background(), "", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "no MCP manager")
}

func TestMCP_EmptyManagerLists(t *testing.T) {
	h := newFakeHost()
	h.mgr = mcp.NewManager(nil)
	res, _ := (MCP{}).Run(context.Background(), "", h)
	require.Contains(t, res.Text, "no MCP servers")
}

func TestMCP_StatusForUnknownServer(t *testing.T) {
	h := newFakeHost()
	h.mgr = mcp.NewManager(nil)
	res, _ := (MCP{}).Run(context.Background(), "status nope", h)
	require.Contains(t, res.Text, "no such MCP server")
}

func TestMCP_UnknownSubcommand(t *testing.T) {
	h := newFakeHost()
	h.mgr = mcp.NewManager(nil)
	res, _ := (MCP{}).Run(context.Background(), "garbage", h)
	require.Contains(t, res.Text, "usage:")
}
