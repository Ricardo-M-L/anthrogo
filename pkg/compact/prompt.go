package compact

import (
	"encoding/json"

	"github.com/ricardo/anthrogo/pkg/message"
)

const SummarySystemPrompt = `You are summarizing an in-progress conversation between a user and an AI coding assistant.

Produce a dense, faithful summary that:
- Preserves all factual claims the assistant made about the codebase (file paths, function names, decisions taken).
- Preserves outstanding questions or in-progress tasks.
- Drops chit-chat, retracted statements, and verbose tool output.
- Does NOT invent new information.

Output ONLY the summary as plain prose. No preamble, no markdown headings, no apologies.`

// buildSummaryUserMessage constructs the single synthetic user message we send
// to the LLM along with the head messages JSON-serialized inside it.
func buildSummaryUserMessage(head []message.Message) message.Message {
	raw, err := json.Marshal(head)
	if err != nil {
		raw = []byte("[]") // shouldn't happen with well-typed message structs
	}
	text := "Conversation to summarize (JSON, oldest first):\n\n" + string(raw)
	return message.Message{
		Role:    message.RoleUser,
		Content: []message.Block{{Type: message.BlockText, Text: text}},
	}
}
