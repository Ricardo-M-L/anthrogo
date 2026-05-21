# `github.com/ricardo/anthrogo/pkg/message`

```go
package message // import "github.com/ricardo/anthrogo/pkg/message"


TYPES

type Block struct {
	Type BlockType `json:"type"`

	// text / thinking
	Text string `json:"text,omitempty"`

	// tool_use
	ToolUseID string         `json:"id,omitempty"`
	ToolName  string         `json:"name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`

	// tool_result
	IsError bool `json:"is_error,omitempty"`

	// image
	ImageSource *ImageSource `json:"source,omitempty"`
}
    Block is a single content block within a Message. One struct with
    type-specific fields, rather than a Go interface union, keeps marshalling
    straightforward; only the fields relevant to Type are emitted.

func ParseUserPrompt(prompt string) ([]Block, error)
    ParseUserPrompt converts a user prompt string into a slice of Blocks.
    Recognizes "@image:<path>" tokens anywhere in the string; each matched
    token loads the file, base64-encodes it, and emits a BlockImage at the
    same position. Text on either side of image refs is preserved as BlockText
    blocks. Returns an error if any referenced file cannot be read.

func (b Block) MarshalJSON() ([]byte, error)

func (b *Block) UnmarshalJSON(data []byte) error

type BlockType string
    BlockType enumerates the variants of a ContentBlock (mirrors Anthropic
    Messages API: text / tool_use / tool_result / image / thinking).

const (
	BlockText       BlockType = "text"
	BlockToolUse    BlockType = "tool_use"
	BlockToolResult BlockType = "tool_result"
	BlockImage      BlockType = "image"
	BlockThinking   BlockType = "thinking"
)
type ImageSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // image/png, image/jpeg, ...
	Data      string `json:"data"`
}
    ImageSource follows the Anthropic spec: base64-encoded media.

type Message struct {
	Role    Role    `json:"role"`
	Content []Block `json:"content"`
}
    Message is one turn in the conversation.

func Text(role Role, s string) Message
    Text helper: build a single-block user/assistant message.

type Role string
    Role is the speaker label sent to the API.

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system" // local-only; never sent over the wire
)
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}
    Usage tracks token accounting across a turn.

func (u *Usage) Add(o Usage)
    Add accumulates another Usage in place.

```
