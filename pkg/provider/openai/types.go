package openai

import "encoding/json"

type chatRequest struct {
	Model         string      `json:"model"`
	Messages      []chatMsg   `json:"messages"`
	Tools         []chatTool  `json:"tools,omitempty"`
	Stream        bool        `json:"stream"`
	MaxTokens     int         `json:"max_tokens,omitempty"`
	Temperature   float64     `json:"temperature,omitempty"`
	StreamOptions *streamOpts `json:"stream_options,omitempty"`
}

type streamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatMsg struct {
	Role       string         `json:"role"`
	Content    any            `json:"content,omitempty"` // string OR []chatContent
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type chatContent struct {
	Type     string        `json:"type"` // "text" | "image_url"
	Text     string        `json:"text,omitempty"`
	ImageURL *chatImageURL `json:"image_url,omitempty"`
}

type chatImageURL struct {
	URL    string `json:"url"`              // "data:image/png;base64,<base64>"
	Detail string `json:"detail,omitempty"` // "low" | "high" | "auto"
}

type chatToolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function chatFunc `json:"function"`
	// ExtraContent carries provider-specific opaque metadata that must be
	// echoed back on subsequent turns. Gemini 3 requires
	// `extra_content.google.thought_signature` to be present on every
	// historical assistant tool_call or the request errors HTTP 400.
	ExtraContent json.RawMessage `json:"extra_content,omitempty"`
}

type chatFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatTool struct {
	Type     string           `json:"type"`
	Function chatToolFuncSpec `json:"function"`
}

type chatToolFuncSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// SSE chunk shape
type chatChunk struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Choices []chatChoice `json:"choices"`
	Usage   *chatUsage   `json:"usage,omitempty"`
}

type chatChoice struct {
	Index        int       `json:"index"`
	Delta        chatDelta `json:"delta"`
	FinishReason string    `json:"finish_reason"`
}

type chatDelta struct {
	Role      string              `json:"role,omitempty"`
	Content   string              `json:"content,omitempty"`
	ToolCalls []chatToolCallDelta `json:"tool_calls,omitempty"`
	// ReasoningContent is the chain-of-thought stream emitted by DeepSeek-R1,
	// Qwen-QwQ, GLM-Z1, and other "thinking" models via OpenAI-compat. Most
	// providers put it in this field; some use "reasoning". We read both.
	ReasoningContent string `json:"reasoning_content,omitempty"`
	Reasoning        string `json:"reasoning,omitempty"`
}

type chatToolCallDelta struct {
	Index        int             `json:"index"`
	ID           string          `json:"id,omitempty"`
	Type         string          `json:"type,omitempty"`
	Function     chatFuncDelta   `json:"function"`
	ExtraContent json.RawMessage `json:"extra_content,omitempty"`
}

type chatFuncDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
