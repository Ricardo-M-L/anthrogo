package tool

import (
	"context"
	"errors"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// fakeMCPResourceManager is a test double for MCPResourceManager.
type fakeMCPResourceManager struct {
	resources   map[string][]*sdk.Resource
	readResult  *sdk.ReadResourceResult
	readErr     error
}

func (f *fakeMCPResourceManager) AllResources(_ context.Context) map[string][]*sdk.Resource {
	return f.resources
}

func (f *fakeMCPResourceManager) ReadResource(_ context.Context, _, _ string) (*sdk.ReadResourceResult, error) {
	return f.readResult, f.readErr
}

func TestMCPResource_Schema(t *testing.T) {
	tool := NewMCPResource(nil)
	schema := tool.Schema()
	require.Equal(t, "object", schema["type"])
	props, _ := schema["properties"].(map[string]any)
	require.Contains(t, props, "server")
	require.Contains(t, props, "uri")
	required, _ := schema["required"].([]string)
	require.Equal(t, []string{"server"}, required)
}

func TestMCPResource_NilManager_ReturnsError(t *testing.T) {
	tool := NewMCPResource(nil)
	res, err := tool.Call(context.Background(), map[string]any{"server": "fs"}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "no manager configured")
}

func TestMCPResource_MissingServer_ReturnsError(t *testing.T) {
	tool := NewMCPResource(&fakeMCPResourceManager{resources: map[string][]*sdk.Resource{}})
	res, err := tool.Call(context.Background(), map[string]any{}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "server is required")
}

func TestMCPResource_UnknownServer_ReturnsError(t *testing.T) {
	mgr := &fakeMCPResourceManager{
		resources: map[string][]*sdk.Resource{},
	}
	tool := NewMCPResource(mgr)
	res, err := tool.Call(context.Background(), map[string]any{"server": "unknown"}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "not ready or has no resources")
}

func TestMCPResource_List_HappyPath(t *testing.T) {
	mgr := &fakeMCPResourceManager{
		resources: map[string][]*sdk.Resource{
			"fs": {
				{URI: "file:///tmp/notes.md", Name: "notes", MIMEType: "text/markdown", Description: "Daily notes", Size: 1234},
				{URI: "file:///tmp/log.txt", Name: "log", MIMEType: "text/plain"},
			},
		},
	}
	tool := NewMCPResource(mgr)
	res, err := tool.Call(context.Background(), map[string]any{"server": "fs"}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "file:///tmp/notes.md")
	require.Contains(t, res.Text, "notes")
	require.Contains(t, res.Text, "text/markdown")
	require.Contains(t, res.Text, "Daily notes")
	require.Contains(t, res.Text, "file:///tmp/log.txt")
}

func TestMCPResource_Read_HappyPath(t *testing.T) {
	mgr := &fakeMCPResourceManager{
		readResult: &sdk.ReadResourceResult{
			Contents: []*sdk.ResourceContents{
				{URI: "file:///tmp/notes.md", Text: "hello world", MIMEType: "text/plain"},
			},
		},
	}
	tool := NewMCPResource(mgr)
	res, err := tool.Call(context.Background(), map[string]any{
		"server": "fs",
		"uri":    "file:///tmp/notes.md",
	}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Equal(t, "hello world", res.Text)
	require.Equal(t, "fs", res.Data["server"])
	require.Equal(t, "file:///tmp/notes.md", res.Data["uri"])
}

func TestMCPResource_Read_BlobContent(t *testing.T) {
	mgr := &fakeMCPResourceManager{
		readResult: &sdk.ReadResourceResult{
			Contents: []*sdk.ResourceContents{
				{URI: "file:///tmp/img.png", Blob: []byte{0x89, 0x50, 0x4E, 0x47}, MIMEType: "image/png"},
			},
		},
	}
	tool := NewMCPResource(mgr)
	res, err := tool.Call(context.Background(), map[string]any{
		"server": "fs",
		"uri":    "file:///tmp/img.png",
	}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "binary blob")
	require.Contains(t, res.Text, "4 bytes")
}

func TestMCPResource_Read_Error(t *testing.T) {
	mgr := &fakeMCPResourceManager{
		readErr: errors.New("connection refused"),
	}
	tool := NewMCPResource(mgr)
	res, err := tool.Call(context.Background(), map[string]any{
		"server": "fs",
		"uri":    "file:///tmp/notes.md",
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "connection refused")
}

func TestMCPResource_UserFacingName(t *testing.T) {
	tool := NewMCPResource(nil)
	require.Equal(t, "MCPResource: file:///x.md", tool.UserFacingName(map[string]any{"server": "fs", "uri": "file:///x.md"}))
	require.Equal(t, "MCPResource: list fs", tool.UserFacingName(map[string]any{"server": "fs"}))
	require.Equal(t, "MCPResource", tool.UserFacingName(map[string]any{}))
}

func TestMCPResource_IsReadOnly(t *testing.T) {
	tool := NewMCPResource(nil)
	require.True(t, tool.IsReadOnly())
	require.True(t, tool.IsConcurrencySafe())
}
