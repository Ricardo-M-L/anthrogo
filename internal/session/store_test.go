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
