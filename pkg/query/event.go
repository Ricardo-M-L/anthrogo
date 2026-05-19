package query

import "github.com/ricardo/anthrogo/pkg/message"

// EventKind discriminates the engine's outgoing event stream.
// Surfaces (TUI / headless) consume this; provider events are internal.
type EventKind string

const (
	KindAssistantDelta EventKind = "assistant_delta"
	KindAssistantStop  EventKind = "assistant_stop"
	KindToolUseRequest EventKind = "tool_use_request"
	KindToolResult     EventKind = "tool_result"
	KindPermissionAsk  EventKind = "permission_ask"
	KindTurnComplete   EventKind = "turn_complete"
	KindUsage          EventKind = "usage"
	KindError          EventKind = "error"
)

type Event struct {
	Kind EventKind

	// assistant_delta / thinking_delta
	Text string

	// tool_use_request / tool_result
	ToolUseID string
	ToolName  string
	ToolInput map[string]any
	IsError   bool

	// usage
	Usage message.Usage

	// error
	Err error

	// turn_complete
	StopReason string
}
