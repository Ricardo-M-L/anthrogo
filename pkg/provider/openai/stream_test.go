package openai

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/provider"
)

func testReqSimple() provider.Request {
	return provider.Request{
		Model: "gpt-4",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: []message.Block{{Type: message.BlockText, Text: "hello"}}},
		},
	}
}

func testReqWithSystem() provider.Request {
	r := testReqSimple()
	r.SystemPrompt = "be helpful"
	return r
}

func testReqWithToolResult() provider.Request {
	return provider.Request{
		Model: "gpt-4",
		Messages: []message.Message{
			{
				Role: message.RoleUser,
				Content: []message.Block{
					{Type: message.BlockToolResult, ToolUseID: "call-123", Text: "result text"},
				},
			},
		},
	}
}

func TestMapFinishReason(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"stop", "end_turn"},
		{"tool_calls", "tool_use"},
		{"length", "max_tokens"},
		{"content_filter", "content_filter"},
		{"", ""},
	}
	for _, c := range cases {
		require.Equal(t, c.want, mapFinishReason(c.in), "input: %q", c.in)
	}
}

func TestBuildRequest_SystemPrompt(t *testing.T) {
	req := buildRequest("gpt-4", testReqWithSystem())
	require.Equal(t, "system", req.Messages[0].Role)
	require.Equal(t, "be helpful", req.Messages[0].Content)
}

func TestBuildRequest_ToolMessages(t *testing.T) {
	req := buildRequest("gpt-4", testReqWithToolResult())
	found := false
	for _, m := range req.Messages {
		if m.Role == "tool" {
			found = true
			require.Equal(t, "call-123", m.ToolCallID)
			require.Equal(t, "result text", m.Content)
		}
	}
	require.True(t, found, "expected tool-role message")
}

func TestBuildRequest_StreamTrue(t *testing.T) {
	req := buildRequest("m", testReqSimple())
	require.True(t, req.Stream)
	require.NotNil(t, req.StreamOptions)
	require.True(t, req.StreamOptions.IncludeUsage)
}

func TestBuildRequest_Tools(t *testing.T) {
	r := testReqSimple()
	r.Tools = []provider.ToolSpec{
		{Name: "bash", Description: "run shell", InputSchema: map[string]any{"type": "object"}},
	}
	req := buildRequest("m", r)
	require.Len(t, req.Tools, 1)
	require.Equal(t, "function", req.Tools[0].Type)
	require.Equal(t, "bash", req.Tools[0].Function.Name)
}

func TestBuildRequest_AssistantToolUse(t *testing.T) {
	r := provider.Request{
		Model: "gpt-4",
		Messages: []message.Message{
			{
				Role: message.RoleAssistant,
				Content: []message.Block{
					{Type: message.BlockText, Text: "I will call bash"},
					{Type: message.BlockToolUse, ToolUseID: "tc-1", ToolName: "bash", Input: map[string]any{"cmd": "ls"}},
				},
			},
		},
	}
	req := buildRequest("m", r)
	require.Len(t, req.Messages, 1)
	require.Equal(t, "assistant", req.Messages[0].Role)
	require.Len(t, req.Messages[0].ToolCalls, 1)
	require.Equal(t, "tc-1", req.Messages[0].ToolCalls[0].ID)
}
