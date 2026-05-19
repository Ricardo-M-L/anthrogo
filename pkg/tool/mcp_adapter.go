// Package tool — MCP adapter notes (M3.A1 SDK survey):
//
// Imported as: github.com/modelcontextprotocol/go-sdk/mcp
// Key types:
//   - mcp.Client               — top-level client, created via mcp.NewClient(*Implementation, *ClientOptions)
//   - mcp.ClientSession        — connected handle returned by client.Connect(ctx, Transport, nil)
//   - mcp.StdioTransport       — implements Transport for stdin/stdout (server-side, no fields)
//   - mcp.CommandTransport     — implements Transport for a child process (Command *exec.Cmd, TerminateDuration time.Duration)
//   - mcp.Tool                 — descriptor: Name string, Title string, Description string,
//                                InputSchema any (map[string]any on client side), OutputSchema any
//   - mcp.ListToolsParams      — Cursor string for pagination
//   - mcp.ListToolsResult      — Tools []*mcp.Tool, NextCursor string
//   - mcp.CallToolParams       — Name string, Arguments any
//   - mcp.CallToolResult       — Content []mcp.Content, StructuredContent any, IsError bool
//   - mcp.Implementation       — {Name, Version string} passed to NewClient/NewServer
//   - mcp.ClientOptions        — Logger *slog.Logger, ToolListChangedHandler, LoggingMessageHandler, KeepAlive, etc.
//
// Client construction:
//   client := mcp.NewClient(&mcp.Implementation{Name: "anthrogo", Version: "v0.3"}, nil)
//   transport := &mcp.CommandTransport{Command: exec.Command(cmd, args...)}
//   session, err := client.Connect(ctx, transport, nil)
//
// Session API used in B3-B6:
//   session.ListTools(ctx, &mcp.ListToolsParams{})    → *ListToolsResult, error
//   session.Tools(ctx, params)                        → iter.Seq2[*Tool, error]  (pagination helper)
//   session.CallTool(ctx, &mcp.CallToolParams{Name, Arguments}) → *CallToolResult, error
//   session.Close()                                   → error
//   session.Wait()                                    → error (blocks until server exits)
//
// Notifications (set in ClientOptions):
//   ToolListChangedHandler  func(context.Context, *ToolListChangedRequest)
//   LoggingMessageHandler   func(context.Context, *LoggingMessageRequest)
//   ProgressNotificationHandler func(context.Context, *ProgressNotificationClientRequest)
//
// Tasks B3-B6 implement the SDK adapter shims using these names.

package tool

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	mcpPrefix         = "mcp__"
	mcpSep            = "__"
	mcpMaxToolNameLen = 64
	mcpServerMaxLen   = 16
	mcpToolMaxLen     = 41 // 64 - len("mcp__") - len("__") - mcpServerMaxLen
)

// MCPToolName composes an MCP tool name from a server and tool name, fitting
// the Anthropic 64-char limit. When either piece overflows its per-piece
// budget, both are truncated and an 8-char sha256 hex suffix is appended for
// collision resistance.
func MCPToolName(server, tool string) string {
	candidate := mcpPrefix + server + mcpSep + tool
	if len(candidate) <= mcpMaxToolNameLen &&
		len(server) <= mcpServerMaxLen &&
		len(tool) <= mcpToolMaxLen {
		return candidate
	}

	srv := server
	if len(srv) > mcpServerMaxLen {
		srv = srv[:mcpServerMaxLen]
	}
	const suffixLen = 1 + 8 // "_<8-hex>"
	toolBudget := mcpMaxToolNameLen - len(mcpPrefix) - len(srv) - len(mcpSep) - suffixLen
	if toolBudget < 1 {
		toolBudget = 1
	}
	toolSlice := tool
	if len(toolSlice) > toolBudget {
		toolSlice = toolSlice[:toolBudget]
	}
	sum := sha256.Sum256([]byte(server + "/" + tool))
	suffix := hex.EncodeToString(sum[:4])
	out := fmt.Sprintf("%s%s%s%s_%s", mcpPrefix, srv, mcpSep, toolSlice, suffix)
	if len(out) > mcpMaxToolNameLen {
		out = out[:mcpMaxToolNameLen]
	}
	return out
}

// MCPInvoker is the callback the manager hands the adapter so the adapter
// doesn't import internal/mcp.
type MCPInvoker func(ctx context.Context, args map[string]any) (*sdk.CallToolResult, error)

// MCPAdapter wraps one MCP server's tool descriptor as a tool.Tool.
type MCPAdapter struct {
	DefaultPermission

	composedName string
	originalName string
	server       string
	descriptor   *sdk.Tool
	invoker      MCPInvoker
}

func NewMCPAdapter(serverName string, descriptor *sdk.Tool, invoker MCPInvoker) *MCPAdapter {
	return &MCPAdapter{
		composedName: MCPToolName(serverName, descriptor.Name),
		originalName: descriptor.Name,
		server:       serverName,
		descriptor:   descriptor,
		invoker:      invoker,
	}
}

func (a *MCPAdapter) Name() string                         { return a.composedName }
func (a *MCPAdapter) Description(_ context.Context) string { return fmt.Sprintf("[%s] %s", a.server, a.descriptor.Description) }
func (a *MCPAdapter) UserFacingName(map[string]any) string { return a.composedName }
func (a *MCPAdapter) IsReadOnly() bool                     { return false }
func (a *MCPAdapter) IsConcurrencySafe() bool              { return true }

func (a *MCPAdapter) Schema() map[string]any {
	if a.descriptor.InputSchema == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	if m, ok := a.descriptor.InputSchema.(map[string]any); ok {
		return m
	}
	raw, err := json.Marshal(a.descriptor.InputSchema)
	if err != nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	return out
}

func (a *MCPAdapter) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error) {
	res, err := a.invoker(ctx, input)
	if err != nil {
		msg := fmt.Sprintf("mcp call %s failed: %s", a.composedName, err.Error())
		return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true}, nil
	}
	text := mcpContentToText(res.Content)
	return Result{
		Type:    ResultText,
		Text:    text,
		ForLLM:  text,
		IsError: res.IsError,
		Data:    map[string]any{"server": a.server, "tool": a.originalName},
	}, nil
}

// mcpContentToText joins MCP content blocks into a single string.
func mcpContentToText(content []sdk.Content) string {
	var parts []string
	for _, c := range content {
		switch v := c.(type) {
		case *sdk.TextContent:
			parts = append(parts, v.Text)
		default:
			raw, _ := json.Marshal(c)
			parts = append(parts, string(raw))
		}
	}
	return strings.Join(parts, "\n")
}
