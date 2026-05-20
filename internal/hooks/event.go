package hooks

// EventName is one of the 9 hook event names.
type EventName string

const (
	EventPreToolUse       EventName = "PreToolUse"
	EventPostToolUse      EventName = "PostToolUse"
	EventUserPromptSubmit EventName = "UserPromptSubmit"
	EventStop             EventName = "Stop"
	EventSubagentStop     EventName = "SubagentStop"
	EventNotification     EventName = "Notification"
	EventPreCompact       EventName = "PreCompact"
	EventSessionStart     EventName = "SessionStart"
	EventSessionEnd       EventName = "SessionEnd"
)

// Common is the envelope every payload carries.
type Common struct {
	HookEventName EventName `json:"hook_event_name"`
	SessionID     string    `json:"session_id"`
	Cwd           string    `json:"cwd"`
	Version       string    `json:"anthrogo_version"`
}

type PreToolUsePayload struct {
	Common
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
}

type PostToolUsePayload struct {
	Common
	ToolName     string         `json:"tool_name"`
	ToolInput    map[string]any `json:"tool_input"`
	ToolResponse map[string]any `json:"tool_response"`
}

type UserPromptSubmitPayload struct {
	Common
	Prompt string `json:"prompt"`
}

type StopPayload struct {
	Common
	StopReason string `json:"stop_reason"`
}

type NotificationPayload struct {
	Common
	Message string `json:"message"`
	Kind    string `json:"kind"`
}

type PreCompactPayload struct {
	Common
	Trigger string `json:"trigger"`
}

type SessionStartPayload struct {
	Common
	Kind string `json:"kind"`
}

type SessionEndPayload struct {
	Common
	Kind string `json:"kind"`
}

// Output is what the hook writes to stdout. All fields optional.
type Output struct {
	Continue           *bool               `json:"continue,omitempty"`
	StopReason         string              `json:"stopReason,omitempty"`
	HookSpecificOutput *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

type HookSpecificOutput struct {
	HookEventName            EventName      `json:"hookEventName"`
	PermissionDecision       string         `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string         `json:"permissionDecisionReason,omitempty"`
	ModifiedInput            map[string]any `json:"modifiedInput,omitempty"`
	AdditionalContext        string         `json:"additionalContext,omitempty"`
}
