package session

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/message"
)

func TestStore_AppendAndReplay(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANTHROGO_HOME", dir)

	s, err := New(NewOptions{Cwd: dir, Model: "claude-x", PermissionMode: "default"})
	require.NoError(t, err)

	require.NoError(t, s.Append(Record{Kind: KindUserMessage, UserMessage: &UserMessage{
		Content: []message.Block{{Type: message.BlockText, Text: "hi"}},
	}}))
	require.NoError(t, s.Append(Record{Kind: KindAssistantMessage, AssistantMessage: &AssistantMessage{
		Content:    []message.Block{{Type: message.BlockText, Text: "hello"}},
		StopReason: "end_turn",
	}}))
	require.NoError(t, s.Append(Record{Kind: KindTurnComplete, TurnComplete: &TurnComplete{StopReason: "end_turn"}}))
	require.NoError(t, s.Close())

	records, err := Replay(s.Path())
	require.NoError(t, err)
	require.Len(t, records, 4)
	require.Equal(t, KindSessionMeta, records[0].Kind)
	require.Equal(t, KindUserMessage, records[1].Kind)
	require.Equal(t, "hello", records[2].AssistantMessage.Content[0].Text)
}

func TestReplay_DropsMessagesBeforeCompact(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANTHROGO_HOME", dir)

	s, err := New(NewOptions{Cwd: dir, Model: "claude-x", PermissionMode: "default"})
	require.NoError(t, err)

	// Pre-compact messages (should be discarded after compact record).
	require.NoError(t, s.Append(Record{Kind: KindUserMessage, UserMessage: &UserMessage{
		Content: []message.Block{{Type: message.BlockText, Text: "old-user"}},
	}}))
	require.NoError(t, s.Append(Record{Kind: KindAssistantMessage, AssistantMessage: &AssistantMessage{
		Content:    []message.Block{{Type: message.BlockText, Text: "old-assistant"}},
		StopReason: "end_turn",
	}}))

	// Compact record — signals discard of prior messages.
	require.NoError(t, s.Append(Record{Kind: KindCompact, Compact: &CompactRecord{
		OriginalCount: 2, NewCount: 1, Trigger: "manual",
	}}))

	// Post-compact messages (the summary + new turn).
	require.NoError(t, s.Append(Record{Kind: KindUserMessage, UserMessage: &UserMessage{
		Content: []message.Block{{Type: message.BlockText, Text: "[Compacted earlier conversation] summary"}},
	}}))
	require.NoError(t, s.Append(Record{Kind: KindAssistantMessage, AssistantMessage: &AssistantMessage{
		Content:    []message.Block{{Type: message.BlockText, Text: "new-response"}},
		StopReason: "end_turn",
	}}))
	require.NoError(t, s.Append(Record{Kind: KindTurnComplete, TurnComplete: &TurnComplete{StopReason: "end_turn"}}))
	require.NoError(t, s.Close())

	records, err := Replay(s.Path())
	require.NoError(t, err)

	msgs := Messages(records)
	// Only the 2 messages after the compact record should appear; pre-compact
	// user+assistant messages must be discarded.
	require.Len(t, msgs, 2)
	require.Equal(t, message.RoleUser, msgs[0].Role)
	require.Contains(t, msgs[0].Content[0].Text, "Compacted earlier conversation")
	require.Equal(t, message.RoleAssistant, msgs[1].Role)
	require.Equal(t, "new-response", msgs[1].Content[0].Text)
}

func TestStore_HookFactory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANTHROGO_HOME", dir)

	s, err := New(NewOptions{Cwd: dir, Model: "m"})
	require.NoError(t, err)
	hook := s.NewRecordHook()
	hook(Record{Kind: KindUserMessage, Timestamp: time.Now(), UserMessage: &UserMessage{
		Content: []message.Block{{Type: message.BlockText, Text: "via-hook"}},
	}})
	require.NoError(t, s.Close())

	raw, err := os.ReadFile(s.Path())
	require.NoError(t, err)
	require.Contains(t, string(raw), "via-hook")
}
