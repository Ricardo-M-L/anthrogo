package hooks

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEventName_StringRoundtrip(t *testing.T) {
	require.Equal(t, "PreToolUse", string(EventPreToolUse))
	require.Equal(t, "SessionEnd", string(EventSessionEnd))
}

func TestPayload_PreToolUse_MarshalsAllFields(t *testing.T) {
	p := PreToolUsePayload{
		Common: Common{
			HookEventName: EventPreToolUse,
			SessionID:     "abc",
			Cwd:           "/x",
			Version:       "0.4.0-dev",
		},
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "ls"},
	}
	raw, err := json.Marshal(p)
	require.NoError(t, err)
	var back map[string]any
	require.NoError(t, json.Unmarshal(raw, &back))
	require.Equal(t, "PreToolUse", back["hook_event_name"])
	require.Equal(t, "abc", back["session_id"])
	require.Equal(t, "/x", back["cwd"])
	require.Equal(t, "Bash", back["tool_name"])
	ti, ok := back["tool_input"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "ls", ti["command"])
}
