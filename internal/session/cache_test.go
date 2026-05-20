package session

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/message"
)

// writeCacheJSONL writes a minimal JSONL with one user-message record to dir/<name>.jsonl.
func writeCacheJSONL(t *testing.T, dir, name, text string) string {
	t.Helper()
	ts := time.Now()
	records := []Record{{
		Kind:      KindUserMessage,
		Timestamp: ts,
		UserMessage: &UserMessage{
			Content: []message.Block{{Type: message.BlockText, Text: text}},
		},
	}}
	var buf bytes.Buffer
	for _, r := range records {
		line, err := r.MarshalJSONLine()
		require.NoError(t, err)
		buf.Write(line)
	}
	p := filepath.Join(dir, name+".jsonl")
	require.NoError(t, os.WriteFile(p, buf.Bytes(), 0o644))
	return p
}

func TestReplayCache_GetCachesResults(t *testing.T) {
	dir := t.TempDir()
	p := writeCacheJSONL(t, dir, "sess1", "hello cache")

	c := NewReplayCache(8)

	r1, err := c.Get(p)
	require.NoError(t, err)
	require.Len(t, r1, 1)
	require.Equal(t, 1, c.Size())

	r2, err := c.Get(p)
	require.NoError(t, err)
	require.Equal(t, r1, r2, "second Get must return same records")
	require.Equal(t, 1, c.Size(), "cache size must remain 1 after re-hit")
}

func TestReplayCache_InvalidatesOnModtimeChange(t *testing.T) {
	dir := t.TempDir()
	p := writeCacheJSONL(t, dir, "sess2", "original text")

	c := NewReplayCache(8)

	r1, err := c.Get(p)
	require.NoError(t, err)
	require.Len(t, r1, 1)

	// Rewrite the file with different content and advance its modtime by at least 1s.
	time.Sleep(10 * time.Millisecond)
	p2 := writeCacheJSONL(t, dir, "sess2-new", "updated text")
	data, err := os.ReadFile(p2)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(p, data, 0o644))
	// Bump modtime past what the OS may have cached.
	future := time.Now().Add(2 * time.Second)
	require.NoError(t, os.Chtimes(p, future, future))

	r2, err := c.Get(p)
	require.NoError(t, err)
	require.Len(t, r2, 1)
	require.Equal(t, "updated text", r2[0].UserMessage.Content[0].Text)
}

func TestReplayCache_EvictsOldestAtCapacity(t *testing.T) {
	dir := t.TempDir()
	c := NewReplayCache(2)

	p1 := writeCacheJSONL(t, dir, "ev1", "ev1")
	p2 := writeCacheJSONL(t, dir, "ev2", "ev2")
	p3 := writeCacheJSONL(t, dir, "ev3", "ev3")

	_, err := c.Get(p1)
	require.NoError(t, err)
	_, err = c.Get(p2)
	require.NoError(t, err)
	// Adding p3 must evict p1 (oldest).
	_, err = c.Get(p3)
	require.NoError(t, err)

	require.Equal(t, 2, c.Size())

	// p1 should have been evicted — it won't be in items.
	c.mu.Lock()
	_, p1Present := c.items[p1]
	_, p2Present := c.items[p2]
	_, p3Present := c.items[p3]
	c.mu.Unlock()

	require.False(t, p1Present, "p1 should be evicted")
	require.True(t, p2Present, "p2 should still be present")
	require.True(t, p3Present, "p3 should be present")
}

func TestReplayCache_InvalidateRemoves(t *testing.T) {
	dir := t.TempDir()
	p := writeCacheJSONL(t, dir, "inv1", "to be invalidated")

	c := NewReplayCache(8)
	_, err := c.Get(p)
	require.NoError(t, err)
	require.Equal(t, 1, c.Size())

	c.Invalidate(p)
	require.Equal(t, 0, c.Size())

	// Invalidate of non-existent path should be a no-op.
	c.Invalidate("/nonexistent/path.jsonl")
	require.Equal(t, 0, c.Size())
}

func TestReplayCache_Clear(t *testing.T) {
	dir := t.TempDir()
	c := NewReplayCache(8)

	for _, name := range []string{"cl1", "cl2", "cl3"} {
		p := writeCacheJSONL(t, dir, name, name)
		_, err := c.Get(p)
		require.NoError(t, err)
	}
	require.Equal(t, 3, c.Size())

	c.Clear()
	require.Equal(t, 0, c.Size())
}
