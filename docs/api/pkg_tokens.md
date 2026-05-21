# `github.com/ricardo/anthrogo/pkg/tokens`

```go
package tokens // import "github.com/ricardo/anthrogo/pkg/tokens"


FUNCTIONS

func SetAnthropicAPICounter(fn func(model string, blocks []message.Block) int)
    SetAnthropicAPICounter registers a callback that will be invoked for
    Claude-family models instead of the char/4 approximation. Pass nil to clear
    the hook and restore the default fallback.


TYPES

type Counter struct {
	// Has unexported fields.
}
    Counter is bound to a specific model. Construct via NewCounter; if the
    model uses tiktoken (OpenAI-family or compatible), the encoder is cached;
    otherwise CountText falls back to char/4 approximation.

func NewCounter(model string) *Counter
    NewCounter returns a Counter for the given model. If the model maps
    to a tiktoken encoding, the encoder is lazily initialised and cached.
    For claude-* or unrecognised models the Counter uses a char/4 approximation.

func (c *Counter) CountBlocks(blocks []message.Block) int
    CountBlocks counts tokens across a slice of message.Block values:
      - BlockText: tokenize Text
      - BlockToolUse: tokenize ToolName + JSON of Input
      - BlockToolResult: tokenize Text
      - BlockThinking: tokenize Text
      - BlockImage: 0 (image tokens are model-specific; provider returns real
        count via EventUsage; M8.10 does not estimate)

    For Claude-family models (no tiktoken encoding), if an AnthropicAPICounter
    has been set via SetAnthropicAPICounter, it is invoked first. A return value
    of -1 means the API call failed; in that case the char/4 fallback is used.

func (c *Counter) CountMessages(msgs []message.Message) int
    CountMessages sums CountBlocks across each message plus a small per-message
    overhead (~3 tokens for role tagging — matches OpenAI's documented format).

func (c *Counter) CountText(s string) int
    CountText returns the token count for s under the model's encoding,
    or (len(s)+3)/4 if no encoding is available (Anthropic fallback). The +3
    rounds up so single-character strings return 1, not 0.

```
