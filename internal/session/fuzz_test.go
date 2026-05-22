package session

import "testing"

// FuzzUnmarshalJSONLine exercises the JSONL record parser on arbitrary byte
// slices. The parser MUST never panic; malformed lines should be returned
// as an error.
func FuzzUnmarshalJSONLine(f *testing.F) {
	// Seed corpus: a few well-formed records + obvious malformed cases.
	seeds := [][]byte{
		[]byte(`{"type":"user_message","ts":"2026-05-22T00:00:00Z","user_message":{"content":[]}}`),
		[]byte(`{"type":"session_meta","ts":"2026-05-22T00:00:00Z","session_meta":{"session_id":"x","cwd":"/tmp","schema_version":2}}`),
		[]byte(`{"type":"turn_complete","ts":"2026-05-22T00:00:00Z","turn_complete":{"stop_reason":"end_turn"}}`),
		[]byte(``),
		[]byte(`{`),
		[]byte(`{"type":}`),
		[]byte(`{"type":"???","ts":"not-a-date"}`),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Cap input to keep the corpus small.
		if len(data) > 64*1024 {
			t.Skip()
		}
		// We don't care about the return value — only that no panic / OOM
		// fires. An error return is the expected behaviour for bad input.
		_, _ = UnmarshalJSONLine(data)
	})
}
