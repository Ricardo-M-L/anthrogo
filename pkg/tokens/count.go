package tokens

import (
	"encoding/json"
	"strings"
	"sync"

	tiktoken "github.com/pkoukk/tiktoken-go"

	"github.com/ricardo/anthrogo/pkg/message"
)

// AnthropicAPICounter, when non-nil, is used for Claude-prefixed models
// instead of the char/4 fallback. Returns -1 on error (callsite falls
// back). Set via SetAnthropicAPICounter from cmd/anthrogo when YAML
// enables this.
var (
	apiMu               sync.RWMutex
	anthropicAPICounter func(model string, blocks []message.Block) int
)

// SetAnthropicAPICounter registers a callback that will be invoked for
// Claude-family models instead of the char/4 approximation. Pass nil to
// clear the hook and restore the default fallback.
func SetAnthropicAPICounter(fn func(model string, blocks []message.Block) int) {
	apiMu.Lock()
	defer apiMu.Unlock()
	anthropicAPICounter = fn
}

// Counter is bound to a specific model. Construct via NewCounter; if the
// model uses tiktoken (OpenAI-family or compatible), the encoder is cached;
// otherwise CountText falls back to char/4 approximation.
type Counter struct {
	enc   *tiktoken.Tiktoken // nil for char/4 fallback
	model string             // remembered for API counter lookup
}

// counterCache memoizes per-encoding-name to avoid repeated init cost.
var (
	counterMu    sync.Mutex
	counterCache = map[string]*tiktoken.Tiktoken{}
)

func encodingForModel(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.HasPrefix(m, "gpt-4o"),
		strings.HasPrefix(m, "o1"),
		strings.HasPrefix(m, "o3"),
		strings.HasPrefix(m, "gpt-5"):
		return "o200k_base"
	case strings.HasPrefix(m, "gpt-4"),
		strings.HasPrefix(m, "gpt-3.5"),
		strings.HasPrefix(m, "text-embedding-3"):
		return "cl100k_base"
	case strings.HasPrefix(m, "deepseek"),
		strings.HasPrefix(m, "kimi"),
		strings.HasPrefix(m, "moonshot"),
		strings.HasPrefix(m, "minimax"),
		strings.HasPrefix(m, "abab"),
		strings.HasPrefix(m, "glm"):
		// approximation; close enough for budget calc
		return "cl100k_base"
	default:
		return "" // signals char/4 fallback
	}
}

// isAnthropicModel returns true for Claude-family model name prefixes
// (first-party and Bedrock/Vertex variants).
func isAnthropicModel(model string) bool {
	m := strings.ToLower(model)
	return strings.HasPrefix(m, "claude-") ||
		strings.HasPrefix(m, "anthropic.claude-") ||
		strings.HasPrefix(m, "anthropic.claude")
}

// NewCounter returns a Counter for the given model. If the model maps to a
// tiktoken encoding, the encoder is lazily initialised and cached. For
// claude-* or unrecognised models the Counter uses a char/4 approximation.
func NewCounter(model string) *Counter {
	enc := encodingForModel(model)
	if enc == "" {
		return &Counter{model: model}
	}
	counterMu.Lock()
	defer counterMu.Unlock()
	if cached, ok := counterCache[enc]; ok {
		return &Counter{enc: cached, model: model}
	}
	tk, err := tiktoken.GetEncoding(enc)
	if err != nil {
		return &Counter{model: model} // fallback
	}
	counterCache[enc] = tk
	return &Counter{enc: tk, model: model}
}

// CountText returns the token count for s under the model's encoding,
// or (len(s)+3)/4 if no encoding is available (Anthropic fallback).
// The +3 rounds up so single-character strings return 1, not 0.
func (c *Counter) CountText(s string) int {
	if c == nil || c.enc == nil {
		// Anthropic's published guideline: ~4 chars/token.
		return (len(s) + 3) / 4
	}
	return len(c.enc.Encode(s, nil, nil))
}

// CountBlocks counts tokens across a slice of message.Block values:
//   - BlockText: tokenize Text
//   - BlockToolUse: tokenize ToolName + JSON of Input
//   - BlockToolResult: tokenize Text
//   - BlockThinking: tokenize Text
//   - BlockImage: 0 (image tokens are model-specific; provider returns real
//     count via EventUsage; M8.10 does not estimate)
//
// For Claude-family models (no tiktoken encoding), if an AnthropicAPICounter
// has been set via SetAnthropicAPICounter, it is invoked first. A return value
// of -1 means the API call failed; in that case the char/4 fallback is used.
func (c *Counter) CountBlocks(blocks []message.Block) int {
	if c == nil {
		return 0
	}
	// For Anthropic models without a tiktoken encoder, try the API counter.
	if c.enc == nil && isAnthropicModel(c.model) {
		apiMu.RLock()
		fn := anthropicAPICounter
		apiMu.RUnlock()
		if fn != nil {
			if n := fn(c.model, blocks); n >= 0 {
				return n
			}
		}
	}
	total := 0
	for _, b := range blocks {
		switch b.Type {
		case message.BlockText:
			total += c.CountText(b.Text)
		case message.BlockToolUse:
			total += c.CountText(b.ToolName)
			if len(b.Input) > 0 {
				if data, err := json.Marshal(b.Input); err == nil {
					total += c.CountText(string(data))
				}
			}
		case message.BlockToolResult:
			total += c.CountText(b.Text)
		case message.BlockThinking:
			total += c.CountText(b.Text)
		case message.BlockImage:
			// skip — image tokens are not local-computable
		}
	}
	return total
}

// CountMessages sums CountBlocks across each message plus a small per-message
// overhead (~3 tokens for role tagging — matches OpenAI's documented format).
func (c *Counter) CountMessages(msgs []message.Message) int {
	total := 0
	for _, m := range msgs {
		total += 3
		total += c.CountBlocks(m.Content)
	}
	return total
}
