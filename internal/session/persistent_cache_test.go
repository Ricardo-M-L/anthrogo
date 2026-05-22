package session

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"

	"github.com/ricardo/anthrogo/pkg/message"
)

// writeJSONL writes a minimal JSONL with the given records to a temp file and
// returns its path. It reuses writeCacheJSONL's approach but accepts records
// directly so tests can vary content.
func writeJSONL(t *testing.T, dir string, records []Record) string {
	t.Helper()
	var buf bytes.Buffer
	for _, r := range records {
		line, err := r.MarshalJSONLine()
		require.NoError(t, err)
		buf.Write(line)
	}
	p := filepath.Join(dir, "sess-"+time.Now().Format("150405.000000000")+".jsonl")
	require.NoError(t, os.WriteFile(p, buf.Bytes(), 0o644))
	return p
}

func TestPersistentCache_GetThenRestart(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "cache.db")

	// Phase 1: create cache, fill with one file.
	jsonlPath := writeJSONL(t, tmpDir, []Record{
		{
			Kind:      KindUserMessage,
			Timestamp: time.Now(),
			UserMessage: &UserMessage{
				Content: []message.Block{{Type: message.BlockText, Text: "hello"}},
			},
		},
	})
	c1 := NewPersistentCache(dbPath, 8)
	recs1, err := c1.Get(jsonlPath)
	require.NoError(t, err)
	require.Len(t, recs1, 1)
	require.NoError(t, c1.Close())

	// Phase 2: new cache instance from same DB file.
	c2 := NewPersistentCache(dbPath, 8)
	require.Equal(t, 1, c2.SizeOnDisk())
	recs2, err := c2.Get(jsonlPath)
	require.NoError(t, err)
	require.Len(t, recs2, 1)
	require.NoError(t, c2.Close())
}

func TestPersistentCache_InvalidatesOnModtime(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "cache.db")

	jsonlPath := writeJSONL(t, tmpDir, []Record{
		{
			Kind:      KindUserMessage,
			Timestamp: time.Now(),
			UserMessage: &UserMessage{
				Content: []message.Block{{Type: message.BlockText, Text: "original"}},
			},
		},
	})
	c := NewPersistentCache(dbPath, 8)
	recs1, err := c.Get(jsonlPath)
	require.NoError(t, err)
	require.Len(t, recs1, 1)
	require.Equal(t, "original", recs1[0].UserMessage.Content[0].Text)

	// Rewrite file with new content + bump modtime so it differs by at least 1s.
	newRecords := []Record{
		{
			Kind:      KindUserMessage,
			Timestamp: time.Now(),
			UserMessage: &UserMessage{
				Content: []message.Block{{Type: message.BlockText, Text: "updated"}},
			},
		},
		{
			Kind:      KindUserMessage,
			Timestamp: time.Now(),
			UserMessage: &UserMessage{
				Content: []message.Block{{Type: message.BlockText, Text: "second"}},
			},
		},
	}
	var buf bytes.Buffer
	for _, r := range newRecords {
		line, err2 := r.MarshalJSONLine()
		require.NoError(t, err2)
		buf.Write(line)
	}
	require.NoError(t, os.WriteFile(jsonlPath, buf.Bytes(), 0o644))
	future := time.Now().Add(2 * time.Second)
	require.NoError(t, os.Chtimes(jsonlPath, future, future))

	recs2, err := c.Get(jsonlPath)
	require.NoError(t, err)
	require.Len(t, recs2, 2)
	require.Equal(t, "updated", recs2[0].UserMessage.Content[0].Text)
	require.NoError(t, c.Close())
}

func TestPersistentCache_Invalidate(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "cache.db")

	jsonlPath := writeJSONL(t, tmpDir, []Record{
		{Kind: KindUserMessage, Timestamp: time.Now(), UserMessage: &UserMessage{
			Content: []message.Block{{Type: message.BlockText, Text: "inv"}},
		}},
	})
	c := NewPersistentCache(dbPath, 8)
	_, err := c.Get(jsonlPath)
	require.NoError(t, err)
	require.Equal(t, 1, c.SizeOnDisk())
	require.Equal(t, 1, c.mem.Size())

	c.Invalidate(jsonlPath)
	require.Equal(t, 0, c.SizeOnDisk(), "L2 should be empty after Invalidate")
	require.Equal(t, 0, c.mem.Size(), "L1 should be empty after Invalidate")

	// No-op on missing path.
	c.Invalidate("/no/such/path.jsonl")
	require.Equal(t, 0, c.SizeOnDisk())
	require.NoError(t, c.Close())
}

func TestPersistentCache_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "cache.db")

	c := NewPersistentCache(dbPath, 8)
	for i := 0; i < 3; i++ {
		p := writeJSONL(t, tmpDir, []Record{
			{Kind: KindUserMessage, Timestamp: time.Now(), UserMessage: &UserMessage{
				Content: []message.Block{{Type: message.BlockText, Text: "x"}},
			}},
		})
		_, err := c.Get(p)
		require.NoError(t, err)
		time.Sleep(time.Millisecond) // ensure unique filenames
	}
	require.Equal(t, 3, c.SizeOnDisk())
	require.Equal(t, 3, c.mem.Size())

	c.Clear()
	require.Equal(t, 0, c.SizeOnDisk(), "L2 should be empty after Clear")
	require.Equal(t, 0, c.mem.Size(), "L1 should be empty after Clear")
	require.NoError(t, c.Close())
}

func TestPersistentCache_NoDBFallback(t *testing.T) {
	// Pass a bad path (/dev/null/cache.db); cache degrades to L1-only
	// and still returns records via Replay.
	tmpDir := t.TempDir()
	badDBPath := "/dev/null/cache.db"

	jsonlPath := writeJSONL(t, tmpDir, []Record{
		{Kind: KindUserMessage, Timestamp: time.Now(), UserMessage: &UserMessage{
			Content: []message.Block{{Type: message.BlockText, Text: "fallback"}},
		}},
	})

	c := NewPersistentCache(badDBPath, 8)
	require.Nil(t, c.db, "DB should be nil after bad path")

	recs, err := c.Get(jsonlPath)
	require.NoError(t, err)
	require.Len(t, recs, 1)
	require.Equal(t, "fallback", recs[0].UserMessage.Content[0].Text)
	require.Equal(t, 0, c.SizeOnDisk())
	require.NoError(t, c.Close())
}

func TestPersistentCache_FreshDB_SetsUserVersion(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "cache.db")
	c := NewPersistentCache(dbPath, 4)
	require.NotNil(t, c.db)
	var v int
	require.NoError(t, c.db.QueryRow("PRAGMA user_version").Scan(&v))
	require.Equal(t, cacheSchemaVersion, v)
	require.NoError(t, c.Close())
}

func TestPersistentCache_MigrateV1ToV2(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "cache.db")

	// Create a v1 DB manually (no anthrogo_version column).
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE replay_cache (
		path TEXT PRIMARY KEY,
		modtime INTEGER NOT NULL,
		records_json BLOB NOT NULL,
		cached_at INTEGER NOT NULL
	)`)
	require.NoError(t, err)
	_, err = db.Exec("PRAGMA user_version = 1")
	require.NoError(t, err)
	require.NoError(t, db.Close())

	// Open via PersistentCache — should migrate to v2.
	c := NewPersistentCache(dbPath, 4)
	require.NotNil(t, c.db)
	var v int
	require.NoError(t, c.db.QueryRow("PRAGMA user_version").Scan(&v))
	require.Equal(t, 2, v)

	// Verify anthrogo_version column was added.
	rows, err := c.db.Query("PRAGMA table_info(replay_cache)")
	require.NoError(t, err)
	defer rows.Close()
	found := false
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt any
		require.NoError(t, rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk))
		if name == "anthrogo_version" {
			found = true
		}
	}
	require.True(t, found, "anthrogo_version column should exist after migration")
	require.NoError(t, c.Close())
}

func TestPersistentCache_GetWritesAnthrogoVersion(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "cache.db")

	jsonlPath := writeJSONL(t, tmpDir, []Record{
		{Kind: KindUserMessage, Timestamp: time.Now(), UserMessage: &UserMessage{
			Content: []message.Block{{Type: message.BlockText, Text: "hello"}},
		}},
	})

	c := NewPersistentCache(dbPath, 4)
	require.NotNil(t, c.db)

	_, err := c.Get(jsonlPath)
	require.NoError(t, err)

	// Verify anthrogo_version was written.
	var av sql.NullString
	require.NoError(t, c.db.QueryRow("SELECT anthrogo_version FROM replay_cache WHERE path = ?", jsonlPath).Scan(&av))
	require.True(t, av.Valid, "anthrogo_version should be set")
	require.NotEmpty(t, av.String)
	require.NoError(t, c.Close())
}

func TestPersistentCache_TrimsBeyondCap(t *testing.T) {
	tmpDir := t.TempDir()
	c := NewPersistentCache(filepath.Join(tmpDir, "cache.db"), 64)
	require.NotNil(t, c.db, "db should be open for this test")
	defer c.Close()

	// Populate the table directly with maxPersistentCacheRows+5 rows.
	for i := 0; i < maxPersistentCacheRows+5; i++ {
		_, err := c.db.Exec(
			`INSERT INTO replay_cache (path, modtime, records_json, cached_at, anthrogo_version)
			 VALUES (?, ?, ?, ?, ?)`,
			filepath.Join(tmpDir, "session-"+string(rune('a'+i%26))+"-x"+string(rune('a'+i/26))+".jsonl"),
			int64(i), []byte("[]"), int64(i), "test",
		)
		require.NoError(t, err)
	}
	// Insert one more via the public path to trigger trim.
	// Use a synthetic session file.
	sessionPath := filepath.Join(tmpDir, "session-fresh.jsonl")
	require.NoError(t, os.WriteFile(sessionPath, []byte(""), 0o600))
	_, _ = c.Get(sessionPath)

	require.LessOrEqual(t, c.SizeOnDisk(), maxPersistentCacheRows,
		"row count must not exceed cap after trim")
}

func TestStore_New_RefusesSecondOpenOnSameFile(t *testing.T) {
	tmpDir := t.TempDir()
	sessionID := "test-flock-1"
	a, err := New(NewOptions{
		Cwd:       tmpDir,
		SessionID: sessionID,
		Model:     "x",
	})
	require.NoError(t, err)
	defer a.Close()
	// Second open with same Cwd + SessionID should hit the flock and fail.
	b, err := New(NewOptions{
		Cwd:       tmpDir,
		SessionID: sessionID,
		Model:     "x",
	})
	if b != nil {
		_ = b.Close()
	}
	require.Error(t, err, "second open of the same session JSONL should be refused")
}
