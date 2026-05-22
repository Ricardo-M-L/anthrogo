package message

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTextBlockRoundTrip(t *testing.T) {
	t.Parallel()
	b := Block{Type: BlockText, Text: "hi"}
	data, err := json.Marshal(b)
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"text","text":"hi"}`, string(data))

	var got Block
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, b, got)
}

func TestToolUseBlockRoundTrip(t *testing.T) {
	t.Parallel()
	b := Block{
		Type:      BlockToolUse,
		ToolUseID: "abc",
		ToolName:  "Bash",
		Input:     map[string]any{"command": "ls"},
	}
	data, err := json.Marshal(b)
	require.NoError(t, err)

	var got Block
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, b.ToolName, got.ToolName)
	require.Equal(t, b.ToolUseID, got.ToolUseID)
	require.Equal(t, "ls", got.Input["command"])
}

func TestToolResultBlockMarksError(t *testing.T) {
	t.Parallel()
	b := Block{
		Type:      BlockToolResult,
		ToolUseID: "abc",
		Text:      "command failed: not found",
		IsError:   true,
	}
	data, err := json.Marshal(b)
	require.NoError(t, err)
	require.Contains(t, string(data), `"is_error":true`)
}
