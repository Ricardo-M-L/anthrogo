package query

import "github.com/ricardo/anthrogo/pkg/message"

// EventKind discriminates the engine's outgoing event stream.
// Surfaces (TUI / headless) consume this; provider events are internal.
type EventKind string

const (
	KindAssistantDelta EventKind = "assistant_delta"
	KindThinkingDelta  EventKind = "thinking_delta"
	KindAssistantStop  EventKind = "assistant_stop"
	KindToolUseRequest EventKind = "tool_use_request"
	KindToolResult     EventKind = "tool_result"
	KindPermissionAsk  EventKind = "permission_ask"
	KindTurnComplete   EventKind = "turn_complete"
	KindUsage          EventKind = "usage"
	KindError          EventKind = "error"
	KindStreamRetry    EventKind = "stream_retry"
	KindCancelDraining EventKind = "cancel_draining"
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

	// stream_retry: emitted before each retry attempt.
	// Attempt is 1-based (1 = first retry, 3 = last before exhaust).
	RetryAttempt int
	RetryDelayMs int

	// cancel_draining: emitted when ctx is cancelled but in-flight tools are
	// still running. InFlightCount is the number of goroutines still pending.
	InFlightCount int
	RemainingMs   int
}
