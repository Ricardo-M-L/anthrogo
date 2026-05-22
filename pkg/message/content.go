package message

import (
	"encoding/json"
	"fmt"
)

// BlockType enumerates the variants of a ContentBlock (mirrors Anthropic
// Messages API: text / tool_use / tool_result / image / thinking).
type BlockType string

const (
	BlockText       BlockType = "text"
	BlockToolUse    BlockType = "tool_use"
	BlockToolResult BlockType = "tool_result"
	BlockImage      BlockType = "image"
	BlockThinking   BlockType = "thinking"
)

// Block is a single content block within a Message. One struct with
// type-specific fields, rather than a Go interface union, keeps marshalling
// straightforward; only the fields relevant to Type are emitted.
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

	// ProviderMetadata is opaque per-provider data captured at tool_use
	// emit time and replayed verbatim when the assistant turn is sent back
	// to the provider on the next turn. Carries Gemini 3's
	// `extra_content.google.thought_signature` so multi-turn tool use
	// works (without it, Gemini 3 errors HTTP 400). Omitted from JSON
	// serialization so we don't pollute the session JSONL.
	ProviderMetadata []byte `json:"-"`
}

// ImageSource follows the Anthropic spec: base64-encoded media.
type ImageSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // image/png, image/jpeg, ...
	Data      string `json:"data"`
}

// rawBlock supports the asymmetric tool_result shape, where Anthropic accepts
// either a string or a content[] array. Unmarshal coerces both into Text.
type rawBlock struct {
	Type        BlockType       `json:"type"`
	Text        string          `json:"text,omitempty"`
	ToolUseID   string          `json:"id,omitempty"`
	ResultID    string          `json:"tool_use_id,omitempty"`
	ToolName    string          `json:"name,omitempty"`
	Input       json.RawMessage `json:"input,omitempty"`
	Content     json.RawMessage `json:"content,omitempty"`
	IsError     bool            `json:"is_error,omitempty"`
	ImageSource *ImageSource    `json:"source,omitempty"`
}

func (b *Block) UnmarshalJSON(data []byte) error {
	var r rawBlock
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	b.Type = r.Type
	b.Text = r.Text
	b.IsError = r.IsError
	b.ImageSource = r.ImageSource
	switch r.Type {
	case BlockToolUse:
		b.ToolUseID = r.ToolUseID
		b.ToolName = r.ToolName
		if len(r.Input) > 0 {
			if err := json.Unmarshal(r.Input, &b.Input); err != nil {
				return fmt.Errorf("tool_use input: %w", err)
			}
		}
	case BlockToolResult:
		b.ToolUseID = r.ResultID
		if r.Text == "" && len(r.Content) > 0 {
			// content can be a string or array; collapse to text for M1
			var s string
			if err := json.Unmarshal(r.Content, &s); err == nil {
				b.Text = s
			}
		}
	}
	return nil
}

func (b Block) MarshalJSON() ([]byte, error) {
	switch b.Type {
	case BlockText:
		return json.Marshal(struct {
			Type BlockType `json:"type"`
			Text string    `json:"text"`
		}{b.Type, b.Text})
	case BlockToolUse:
		return json.Marshal(struct {
			Type  BlockType      `json:"type"`
			ID    string         `json:"id"`
			Name  string         `json:"name"`
			Input map[string]any `json:"input"`
		}{b.Type, b.ToolUseID, b.ToolName, b.Input})
	case BlockToolResult:
		out := struct {
			Type      BlockType `json:"type"`
			ToolUseID string    `json:"tool_use_id"`
			Content   string    `json:"content"`
			IsError   bool      `json:"is_error,omitempty"`
		}{b.Type, b.ToolUseID, b.Text, b.IsError}
		return json.Marshal(out)
	case BlockImage:
		return json.Marshal(struct {
			Type   BlockType    `json:"type"`
			Source *ImageSource `json:"source"`
		}{b.Type, b.ImageSource})
	case BlockThinking:
		return json.Marshal(struct {
			Type BlockType `json:"type"`
			Text string    `json:"text"`
		}{b.Type, b.Text})
	default:
		return nil, fmt.Errorf("unknown block type %q", b.Type)
	}
}
