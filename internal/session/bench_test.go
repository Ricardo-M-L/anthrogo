package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/message"
)

func BenchmarkReplayCache_HitAfterWarm(b *testing.B) {
	tmp := b.TempDir()
	p := filepath.Join(tmp, "x.jsonl")
	// Write a small JSONL session file.
	rec := Record{
		Kind:      KindUserMessage,
		Timestamp: time.Now(),
		UserMessage: &UserMessage{
			Content: []message.Block{{Type: message.BlockText, Text: "hi"}},
		},
	}
	line, err := rec.MarshalJSONLine()
	require.NoError(b, err)
	require.NoError(b, os.WriteFile(p, line, 0o644))

	c := NewReplayCache(8)
	_, _ = c.Get(p) // warm
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Get(p)
	}
}
