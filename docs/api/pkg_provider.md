# `github.com/ricardo/anthrogo/pkg/provider`

```go
package provider // import "github.com/ricardo/anthrogo/pkg/provider"


TYPES

type Event struct {
	Kind EventKind

	// text_delta / thinking_delta
	Text string

	// tool_use_start
	ToolUseID string
	ToolName  string

	// tool_input_delta (partial JSON of input)
	PartialJSON string

	// block_stop / message_stop
	StopReason string // "end_turn" | "tool_use" | "max_tokens" | ...

	// usage
	Usage message.Usage

	// error
	Err error
}
    Event is the streaming output unit. Producers must send EventMessageStop or
    EventError exactly once at end of stream.

type EventKind string
    EventKind discriminates Event.

const (
	EventStart          EventKind = "message_start"
	EventTextDelta      EventKind = "text_delta"
	EventToolUseStart   EventKind = "tool_use_start"
	EventToolInputDelta EventKind = "tool_input_delta"
	EventBlockStop      EventKind = "block_stop"
	EventThinkingDelta  EventKind = "thinking_delta"
	EventMessageStop    EventKind = "message_stop"
	EventUsage          EventKind = "usage"
	EventError          EventKind = "error"
)
type Provider interface {
	Stream(ctx context.Context, req Request) (<-chan Event, error)
}
    Provider abstracts an LLM backend. M1 ships one impl (Anthropic SDK).
    M6 adds OpenAI-compat, DeepSeek, Kimi, MiniMax, GLM, Bedrock, Vertex.

type Request struct {
	Model        string
	Messages     []message.Message
	SystemPrompt string
	MaxTokens    int
	Temperature  float64
	Tools        []ToolSpec
	StopReasons  []string
}
    Request is one turn's request to the backend.

type ToolSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}
    ToolSpec is the schema the provider sees for one tool.

```
