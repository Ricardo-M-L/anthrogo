package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/message"
)

func TestResolveContinue_PicksMostRecent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANTHROGO_HOME", dir)

	older, err := New(NewOptions{Cwd: dir, Model: "m"})
	require.NoError(t, err)
	require.NoError(t, older.Close())
	time.Sleep(10 * time.Millisecond)

	newer, err := New(NewOptions{Cwd: dir, Model: "m"})
	require.NoError(t, err)
	require.NoError(t, newer.Close())

	got, err := ResolveContinue(dir)
	require.NoError(t, err)
	require.Equal(t, newer.ID(), got)
}

func TestResolveResume_ExactIDMatch(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANTHROGO_HOME", dir)

	s, err := New(NewOptions{Cwd: dir, Model: "m"})
	require.NoError(t, err)
	require.NoError(t, s.Append(Record{Kind: KindUserMessage, UserMessage: &UserMessage{
		Content: []message.Block{{Type: message.BlockText, Text: "x"}},
	}}))
	require.NoError(t, s.Close())

	id, err := ResolveResume(dir, s.ID()[:8])
	require.NoError(t, err)
	require.Equal(t, s.ID(), id)
}

func TestResolveContinue_NoSessions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANTHROGO_HOME", dir)
	_, err := ResolveContinue(filepath.Join(dir, "nope"))
	require.Error(t, err)
}

func TestResolveResume_InvalidPrefix(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANTHROGO_HOME", dir)

	s, err := New(NewOptions{Cwd: dir, Model: "m"})
	require.NoError(t, err)
	require.NoError(t, s.Close())

	_, err = ResolveResume(dir, "zzzz")
	require.Error(t, err)

	emptyDir := filepath.Join(t.TempDir(), "empty")
	require.NoError(t, os.MkdirAll(emptyDir, 0o755))
	_, err = ResolveResume(emptyDir, "abc")
	require.Error(t, err)
}
