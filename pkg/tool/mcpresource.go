package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPResourceManager is the slice of internal/mcp.Manager that this tool needs.
// Declared here so pkg/tool doesn't import internal/mcp.
type MCPResourceManager interface {
	AllResources(ctx context.Context) map[string][]*sdk.Resource
	ReadResource(ctx context.Context, server, uri string) (*sdk.ReadResourceResult, error)
}

// MCPResource is a built-in tool for listing or reading MCP server resources.
type MCPResource struct {
	DefaultPermission
	mgr MCPResourceManager
}

// NewMCPResource constructs an MCPResource tool backed by the given manager.
func NewMCPResource(m MCPResourceManager) *MCPResource { return &MCPResource{mgr: m} }

func (*MCPResource) Name() string                       { return "MCPResource" }
func (*MCPResource) Description(context.Context) string { return mcpResourceDesc }
func (*MCPResource) UserFacingName(input map[string]any) string {
	if uri, _ := input["uri"].(string); uri != "" {
		return "MCPResource: " + uri
	}
	if srv, _ := input["server"].(string); srv != "" {
		return "MCPResource: list " + srv
	}
	return "MCPResource"
}
func (*MCPResource) IsReadOnly() bool        { return true }
func (*MCPResource) IsConcurrencySafe() bool { return true }

func (*MCPResource) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"server": map[string]any{
				"type":        "string",
				"description": "MCP server name.",
			},
			"uri": map[string]any{
				"type":        "string",
				"description": "Resource URI to read (omit to list all resources on server).",
			},
		},
		"required": []string{"server"},
	}
}

func (t *MCPResource) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error) {
	server, _ := input["server"].(string)
	if server == "" {
		msg := "mcpresource: server is required"
		return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true}, nil
	}
	if t.mgr == nil {
		msg := "mcpresource: no manager configured"
		return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true}, nil
	}

	uri, _ := input["uri"].(string)
	if uri == "" {
		// List resources for the named server.
		all := t.mgr.AllResources(ctx)
		list, ok := all[server]
		if !ok {
			msg := "mcpresource: server " + server + " not ready or has no resources"
			return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true}, nil
		}
		type entry struct {
			URI         string `json:"uri"`
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
			MIMEType    string `json:"mime_type,omitempty"`
			Size        int64  `json:"size,omitempty"`
		}
		entries := make([]entry, 0, len(list))
		for _, r := range list {
			entries = append(entries, entry{
				URI:         r.URI,
				Name:        r.Name,
				Description: r.Description,
				MIMEType:    r.MIMEType,
				Size:        r.Size,
			})
		}
		raw, _ := json.MarshalIndent(entries, "", "  ")
		return Result{Type: ResultText, Text: string(raw), ForLLM: string(raw)}, nil
	}

	// Read a specific resource by URI.
	res, err := t.mgr.ReadResource(ctx, server, uri)
	if err != nil {
		msg := fmt.Sprintf("mcpresource read failed: %v", err)
		return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true}, nil
	}
	var b strings.Builder
	for _, c := range res.Contents {
		if c.Text != "" {
			b.WriteString(c.Text)
		} else if len(c.Blob) > 0 {
			fmt.Fprintf(&b, "[binary blob, %d bytes, mime=%s, uri=%s]\n", len(c.Blob), c.MIMEType, c.URI)
		}
	}
	out := b.String()
	return Result{
		Type:   ResultText,
		Text:   out,
		ForLLM: out,
		Data:   map[string]any{"server": server, "uri": uri},
	}, nil
}

const mcpResourceDesc = `List or read resources advertised by an MCP server. Pass {server: "<name>"} to list all resources, or {server: "<name>", uri: "<uri>"} to read a specific resource.`
