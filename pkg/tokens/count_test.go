package tokens

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/message"
)

func TestCounter_OpenAIModel(t *testing.T) {
	c := NewCounter("gpt-4o")
	n := c.CountText("Hello, world!")
	// Exact value depends on tokenizer version; assert reasonable range
	require.InDelta(t, 4, n, 2, "gpt-4o token count for 'Hello, world!' should be ~4")
}

func TestCounter_AnthropicFallback(t *testing.T) {
	c := NewCounter("claude-sonnet-4-6")
	n := c.CountText("Hello, world!")
	// 13 chars / 4 = 3 (rounded up). Assert non-zero and <= 5.
	require.GreaterOrEqual(t, n, 3)
	require.LessOrEqual(t, n, 5)
}

func TestCounter_CountBlocks_TextAndToolUse(t *testing.T) {
	c := NewCounter("claude-sonnet-4-6") // char/4 fallback for deterministic test
	blocks := []message.Block{
		{Type: message.BlockText, Text: "hello"},
		{
			Type:     message.BlockToolUse,
			ToolName: "bash",
			Input:    map[string]any{"cmd": "ls"},
		},
		{Type: message.BlockToolResult, Text: "file.txt"},
		{Type: message.BlockImage}, // should contribute 0
	}
	n := c.CountBlocks(blocks)
	// "hello" = 5 chars => 2 tokens (ceil(5/4))
	// "bash" = 4 chars => 1 token; {"cmd":"ls"} = 11 chars => 3 tokens
	// "file.txt" = 8 chars => 2 tokens
	// image = 0
	// total = 2 + 1 + 3 + 2 = 8
	require.InDelta(t, 8, n, 3)
}

func TestCounter_CountMessages_AddsRoleOverhead(t *testing.T) {
	c := NewCounter("claude-sonnet-4-6")
	msgs := []message.Message{
		{Role: message.RoleUser, Content: []message.Block{{Type: message.BlockText, Text: "hi"}}},
		{Role: message.RoleAssistant, Content: []message.Block{{Type: message.BlockText, Text: "hi"}}},
	}
	n := c.CountMessages(msgs)
	// Each msg: 3 overhead + ceil(2/4)=1 text token = 4; total ~8
	require.InDelta(t, 8, n, 2)
}

func TestCounter_UnknownModel_UsesCharFour(t *testing.T) {
	c := NewCounter("some-fictional-model")
	// "aaaa bbbb" = 9 chars; (9+3)/4 = 3
	require.Equal(t, 3, c.CountText("aaaa bbbb"))
}

func TestCounter_DeepSeekModel_UsesCL100K(t *testing.T) {
	c := NewCounter("deepseek-chat")
	require.NotNil(t, c.enc, "deepseek should use cl100k_base, not fallback")
	n := c.CountText("Hello, world!")
	require.Greater(t, n, 0)
}

func TestCounter_GPT4Model_UsesCL100K(t *testing.T) {
	c := NewCounter("gpt-4-turbo")
	require.NotNil(t, c.enc)
}

func TestCounter_O1Model_UsesO200K(t *testing.T) {
	c := NewCounter("o1-mini")
	require.NotNil(t, c.enc)
}

func TestCounter_CountBlocks_ThinkingBlock(t *testing.T) {
	c := NewCounter("claude-sonnet-4-6")
	blocks := []message.Block{
		{Type: message.BlockThinking, Text: "thinking text"},
	}
	n := c.CountBlocks(blocks)
	// "thinking text" = 13 chars => (13+3)/4 = 4
	require.Equal(t, 4, n)
}

func TestCounter_Nil_DoesNotPanic(t *testing.T) {
	var c *Counter
	require.NotPanics(t, func() {
		n := c.CountText("hello")
		// nil counter uses char/4
		require.Greater(t, n, 0)
	})
}

func TestCounter_UsesAnthropicAPICounter_WhenSet(t *testing.T) {
	called := false
	SetAnthropicAPICounter(func(model string, blocks []message.Block) int {
		called = true
		return 99
	})
	defer SetAnthropicAPICounter(nil)
	c := NewCounter("claude-sonnet-4-6")
	n := c.CountBlocks([]message.Block{{Type: message.BlockText, Text: "hi"}})
	require.True(t, called)
	require.Equal(t, 99, n)
}

func TestCounter_FallsBackOnAPICounterError(t *testing.T) {
	SetAnthropicAPICounter(func(model string, blocks []message.Block) int { return -1 })
	defer SetAnthropicAPICounter(nil)
	c := NewCounter("claude-sonnet-4-6")
	n := c.CountBlocks([]message.Block{{Type: message.BlockText, Text: "hi"}})
	// Should fall back to char/4: "hi" is 2 chars → (2+3)/4 = 1
	require.GreaterOrEqual(t, n, 1)
	require.LessOrEqual(t, n, 3)
}

func TestCounter_OpenAIModelIgnoresAnthropicAPICounter(t *testing.T) {
	called := false
	SetAnthropicAPICounter(func(string, []message.Block) int {
		called = true
		return 999
	})
	defer SetAnthropicAPICounter(nil)
	c := NewCounter("gpt-4o")
	_ = c.CountBlocks([]message.Block{{Type: message.BlockText, Text: "hi"}})
	require.False(t, called)
}

func TestEncodingForModel(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		{"gpt-4o", "o200k_base"},
		{"gpt-4o-mini", "o200k_base"},
		{"o1-preview", "o200k_base"},
		{"o3-mini", "o200k_base"},
		{"gpt-5", "o200k_base"},
		{"gpt-4-turbo", "cl100k_base"},
		{"gpt-3.5-turbo", "cl100k_base"},
		{"text-embedding-3-small", "cl100k_base"},
		{"deepseek-chat", "cl100k_base"},
		{"kimi-latest", "cl100k_base"},
		{"moonshot-v1-8k", "cl100k_base"},
		{"minimax-text", "cl100k_base"},
		{"glm-4", "cl100k_base"},
		{"claude-3-5-sonnet", ""},
		{"claude-sonnet-4-6", ""},
		{"unknown-model", ""},
	}
	for _, tt := range tests {
		got := encodingForModel(tt.model)
		require.Equal(t, tt.expected, got, "model=%s", tt.model)
	}
}
