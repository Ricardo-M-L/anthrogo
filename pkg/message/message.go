package message

// Role is the speaker label sent to the API.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system" // local-only; never sent over the wire
)

// Message is one turn in the conversation.
type Message struct {
	Role    Role    `json:"role"`
	Content []Block `json:"content"`
}

// Text helper: build a single-block user/assistant message.
func Text(role Role, s string) Message {
	return Message{Role: role, Content: []Block{{Type: BlockText, Text: s}}}
}
