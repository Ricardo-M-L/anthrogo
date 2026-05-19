package tool

import (
	"context"
	"errors"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestMCPToolName_Short(t *testing.T) {
	got := MCPToolName("fs", "read_file")
	require.Equal(t, "mcp__fs__read_file", got)
}

func TestMCPToolName_ExactBudget(t *testing.T) {
	// server len 16, tool len 41 ⇒ exact 64-char fit.
	server := strings.Repeat("s", 16)
	tool := strings.Repeat("t", 41)
	got := MCPToolName(server, tool)
	require.Len(t, got, 64)
	require.Equal(t, "mcp__"+server+"__"+tool, got)
}

func TestMCPToolName_TooLongTruncated(t *testing.T) {
	server := strings.Repeat("s", 30)
	tool := strings.Repeat("t", 60)
	got := MCPToolName(server, tool)
	require.LessOrEqual(t, len(got), 64)
	require.True(t, strings.HasPrefix(got, "mcp__ssssssssssssssss__"), "got %q", got)
	require.Regexp(t, `_[0-9a-f]{8}$`, got, "expected sha-8 suffix on truncation")
}

func TestMCPToolName_DeterministicOnSameInput(t *testing.T) {
	a := MCPToolName("longserver_name_overflow", "longtool_"+strings.Repeat("x", 100))
	b := MCPToolName("longserver_name_overflow", "longtool_"+strings.Repeat("x", 100))
	require.Equal(t, a, b)
}

func TestMCPAdapter_NameAndSchema(t *testing.T) {
	desc := &sdk.Tool{
		Name:        "read_file",
		Description: "Read a file",
		InputSchema: map[string]any{"type": "object"},
	}
	a := NewMCPAdapter("fs", desc, nil)
	require.Equal(t, "mcp__fs__read_file", a.Name())
	require.Contains(t, a.Description(context.Background()), "Read a file")
	sch := a.Schema()
	require.Equal(t, "object", sch["type"])
}

func TestMCPAdapter_CallSuccess(t *testing.T) {
	desc := &sdk.Tool{Name: "echo"}
	invoker := func(_ context.Context, args map[string]any) (*sdk.CallToolResult, error) {
		require.Equal(t, "hi", args["msg"])
		return &sdk.CallToolResult{
			Content: []sdk.Content{&sdk.TextContent{Text: "echo: hi"}},
		}, nil
	}
	a := NewMCPAdapter("svc", desc, invoker)
	res, err := a.Call(context.Background(), map[string]any{"msg": "hi"}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Equal(t, "echo: hi", res.Text)
}

func TestMCPAdapter_CallError(t *testing.T) {
	desc := &sdk.Tool{Name: "boom"}
	invoker := func(context.Context, map[string]any) (*sdk.CallToolResult, error) {
		return nil, errors.New("server crashed")
	}
	a := NewMCPAdapter("svc", desc, invoker)
	res, _ := a.Call(context.Background(), nil, &Context{})
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "server crashed")
}

func TestMCPAdapter_IsErrorPropagates(t *testing.T) {
	desc := &sdk.Tool{Name: "bad"}
	invoker := func(context.Context, map[string]any) (*sdk.CallToolResult, error) {
		return &sdk.CallToolResult{
			Content: []sdk.Content{&sdk.TextContent{Text: "tool reported failure"}},
			IsError: true,
		}, nil
	}
	a := NewMCPAdapter("svc", desc, invoker)
	res, _ := a.Call(context.Background(), nil, &Context{})
	require.True(t, res.IsError)
}
