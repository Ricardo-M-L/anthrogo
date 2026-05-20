package session

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ricardo/anthrogo/pkg/message"
)

type Kind string

const (
	KindSessionMeta      Kind = "session_meta"
	KindUserMessage      Kind = "user_message"
	KindAssistantMessage Kind = "assistant_message"
	KindToolUseRequest   Kind = "tool_use_request"
	KindToolResult       Kind = "tool_result"
	KindTurnComplete     Kind = "turn_complete"
	KindError            Kind = "error"
	KindUsage            Kind = "usage"
	KindCompact          Kind = "compact"
)

type Record struct {
	Kind      Kind      `json:"type"`
	Timestamp time.Time `json:"ts"`

	SessionMeta      *SessionMeta      `json:"session_meta,omitempty"`
	UserMessage      *UserMessage      `json:"user_message,omitempty"`
	AssistantMessage *AssistantMessage `json:"assistant_message,omitempty"`
	ToolUseRequest   *ToolUseRequest   `json:"tool_use_request,omitempty"`
	ToolResult       *ToolResult       `json:"tool_result,omitempty"`
	TurnComplete     *TurnComplete     `json:"turn_complete,omitempty"`
	Error            *ErrorRecord      `json:"error,omitempty"`
	Usage            *UsageRecord      `json:"usage,omitempty"`
	Compact          *CompactRecord    `json:"compact,omitempty"`
}

// CompactRecord is the JSONL record emitted when Engine.Compact runs.
type CompactRecord struct {
	OriginalCount int    `json:"original_count"`
	NewCount      int    `json:"new_count"`
	OriginalBytes int    `json:"original_bytes"`
	NewBytes      int    `json:"new_bytes"`
	Trigger       string `json:"trigger"`
}

type SessionMeta struct {
	SessionID       string    `json:"session_id"`
	Cwd             string    `json:"cwd"`
	Model           string    `json:"model"`
	PermissionMode  string    `json:"permission_mode"`
	AnthrogoVersion string    `json:"anthrogo_version"`
	CreatedAt       time.Time `json:"created_at"`
}

type UserMessage struct {
	Content []message.Block `json:"content"`
}

type AssistantMessage struct {
	Content    []message.Block `json:"content"`
	StopReason string          `json:"stop_reason"`
}

type ToolUseRequest struct {
	ToolUseID string         `json:"tool_use_id"`
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
}

type ToolResult struct {
	ToolUseID string `json:"tool_use_id"`
	Text      string `json:"text"`
	IsError   bool   `json:"is_error,omitempty"`
}

type TurnComplete struct {
	StopReason string `json:"stop_reason"`
}

type ErrorRecord struct {
	Error  string `json:"error"`
	During string `json:"during"`
}

type UsageRecord struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

func (r Record) MarshalJSONLine() ([]byte, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func UnmarshalJSONLine(line []byte) (Record, error) {
	var r Record
	if err := json.Unmarshal(line, &r); err != nil {
		return Record{}, fmt.Errorf("session record: %w", err)
	}
	return r, nil
}
