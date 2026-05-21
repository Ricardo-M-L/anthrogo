# `github.com/ricardo/anthrogo/pkg/compact`

```go
package compact // import "github.com/ricardo/anthrogo/pkg/compact"


CONSTANTS

const SummarySystemPrompt = `You are summarizing an in-progress conversation between a user and an AI coding assistant.

Produce a dense, faithful summary that:
- Preserves all factual claims the assistant made about the codebase (file paths, function names, decisions taken).
- Preserves outstanding questions or in-progress tasks.
- Drops chit-chat, retracted statements, and verbose tool output.
- Does NOT invent new information.

Output ONLY the summary as plain prose. No preamble, no markdown headings, no apologies.`

FUNCTIONS

func ApproxBytes(msgs []message.Message) int
    ApproxBytes is a stand-in for token counting. Returns the sum of JSON-
    marshaled byte lengths across all messages.

    Deprecated: Use pkg/tokens.Counter.CountMessages for accurate token counts.
    ApproxBytes is retained for tests and legacy callers but is no longer called
    by compact.Run (replaced in M8.10).


TYPES

type Input struct {
	Provider   provider.Provider
	Model      string
	Messages   []message.Message
	KeepRecent int // default 10
	MaxTokens  int // default 4096 for the summary call
}
    Input is the pure input to Run. No engine, no hooks.

type Output struct {
	NewMessages    []message.Message
	OriginalCount  int
	NewCount       int
	OriginalTokens int
	NewTokens      int
	SummaryText    string
	Skipped        bool
	SkipReason     string
}
    Output is what Run returns. NewMessages is empty if Skipped.

func Run(ctx context.Context, in Input) (Output, error)
    Run produces a compacted message list. Pure (modulo provider call).

    Algorithm:
     1. If len(messages) <= KeepRecent, skip.
     2. Find split: search forward from desiredSplit = len-KeepRecent until
        we find an assistant message. tail = messages[split:] begins with an
        assistant turn so the new conversation (summary_user + tail...) is valid
        per the Anthropic API (user → assistant alternation).
     3. Summarise the entire head (no MCP carve-out; MCP-aware preservation is
        deferred to a later milestone).
     4. newMessages = [summary_user_msg, tail...]

```
