package builtins

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/internal/mcp"
	"github.com/ricardo/anthrogo/pkg/tool"
)

// stubSentinelTool is a minimal tool.Tool used in reload tests.
type stubSentinelTool struct {
	tool.DefaultPermission
	name string
}

func (s stubSentinelTool) Name() string                                            { return s.name }
func (s stubSentinelTool) Description(context.Context) string                      { return "sentinel" }
func (s stubSentinelTool) Schema() map[string]any                                   { return map[string]any{} }
func (s stubSentinelTool) UserFacingName(map[string]any) string                     { return s.name }
func (s stubSentinelTool) IsReadOnly() bool                                         { return true }
func (s stubSentinelTool) IsConcurrencySafe() bool                                  { return true }
func (s stubSentinelTool) Call(_ context.Context, _ map[string]any, _ *tool.Context) (tool.Result, error) {
	return tool.Result{}, nil
}

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

func TestMCP_Status_RedactsSensitiveHeaders(t *testing.T) {
	h := newFakeHost()
	mgr := mcp.NewManager(nil)
	mgr.AddServer("myserver", mcp.MCPServerConfig{
		Type:     "sse",
		Endpoint: "http://example.com/sse",
		Headers: map[string]string{
			"Authorization": "Bearer secret-token",
			"X-Safe-Header": "visible-value",
		},
	})
	h.mgr = mgr

	res, err := (MCP{}).Run(context.Background(), "status myserver", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "<redacted>", "sensitive header value must be redacted")
	require.NotContains(t, res.Text, "secret-token", "raw secret must not appear in output")
	require.Contains(t, res.Text, "visible-value", "non-sensitive header value must be visible")
	require.Contains(t, res.Text, "Authorization", "header key must appear")
}

func TestMCP_Reload_RemovesAndReregisters(t *testing.T) {
	h := newFakeHost()
	h.mgr = mcp.NewManager(nil) // empty manager — no servers, no live procs

	// Seed the registry with a sentinel mcp__ tool and a non-mcp tool.
	h.tools.Register(tool.Read{})
	h.tools.Register(stubSentinelTool{name: "mcp__test__sentinel"})

	res, err := (MCP{}).Run(context.Background(), "reload", h)
	require.NoError(t, err)
	// The sentinel mcp__ tool must be gone.
	_, stillThere := h.tools.Get("mcp__test__sentinel")
	require.False(t, stillThere, "mcp__ sentinel should have been removed by reload")
	// Non-mcp tools must survive.
	_, readThere := h.tools.Get("Read")
	require.True(t, readThere, "Read tool must survive reload")
	// Result text must mention counts.
	require.Contains(t, res.Text, "removed 1 MCP tools")
	require.Contains(t, res.Text, "registered 0")
}
