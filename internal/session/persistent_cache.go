package session

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// PersistentCache is a SQLite-backed cache of parsed []Record keyed by
// (path, modtime). On miss or stale modtime, it falls through to Replay
// + stores. Falls back to a no-op (in-memory only) if the DB can't open.
type PersistentCache struct {
	mu  sync.Mutex
	db  *sql.DB
	mem *ReplayCache // L1 cache; PersistentCache is L2
}

// NewPersistentCache opens (or creates) a SQLite DB at dbPath. memCap is the
// in-memory L1 LRU capacity. If sql.Open fails, the cache degrades to L1-only
// (logs warning to stderr).
func NewPersistentCache(dbPath string, memCap int) *PersistentCache {
	mem := NewReplayCache(memCap)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "session cache: mkdir %s: %v\n", dbPath, err)
		return &PersistentCache{mem: mem}
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "session cache: open %s: %v\n", dbPath, err)
		return &PersistentCache{mem: mem}
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS replay_cache (
			path TEXT PRIMARY KEY,
			modtime INTEGER NOT NULL,
			records_json BLOB NOT NULL,
			cached_at INTEGER NOT NULL
		);
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "session cache: schema: %v\n", err)
		_ = db.Close()
		return &PersistentCache{mem: mem}
	}
	return &PersistentCache{db: db, mem: mem}
}

// Get returns parsed records: L1 → L2 (if modtime matches) → Replay.
func (c *PersistentCache) Get(path string) ([]Record, error) {
	if c == nil {
		return Replay(path)
	}

	// Stat the file for current modtime (needed for both L1 and L2 checks).
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	modtime := info.ModTime()
	modtimeUnix := modtime.Unix()

	// L1: pure in-memory lookup (does NOT call Replay on miss).
	if recs, ok := c.mem.lookup(path, modtime); ok {
		return recs, nil
	}

	// L2: SQLite lookup
	if c.db != nil {
		c.mu.Lock()
		var cachedModtime int64
		var blob []byte
		queryErr := c.db.QueryRow("SELECT modtime, records_json FROM replay_cache WHERE path = ?", path).Scan(&cachedModtime, &blob)
		c.mu.Unlock()
		if queryErr == nil && cachedModtime == modtimeUnix {
			var recs []Record
			if jerr := json.Unmarshal(blob, &recs); jerr == nil {
				_ = c.mem.warm(path, recs) // populate L1
				return recs, nil
			}
		}
	}

	// Fall through to Replay + persist
	records, err := Replay(path)
	if err != nil {
		return nil, err
	}
	if c.db != nil {
		blob, _ := json.Marshal(records)
		c.mu.Lock()
		_, _ = c.db.Exec(
			"INSERT INTO replay_cache (path, modtime, records_json, cached_at) VALUES (?, ?, ?, ?) ON CONFLICT(path) DO UPDATE SET modtime=excluded.modtime, records_json=excluded.records_json, cached_at=excluded.cached_at",
			path, modtimeUnix, blob, time.Now().Unix(),
		)
		c.mu.Unlock()
	}
	_ = c.mem.warm(path, records)
	return records, nil
}

// Invalidate removes a single path from both L1 and L2.
func (c *PersistentCache) Invalidate(path string) {
	if c == nil {
		return
	}
	c.mem.Invalidate(path)
	if c.db != nil {
		c.mu.Lock()
		_, _ = c.db.Exec("DELETE FROM replay_cache WHERE path = ?", path)
		c.mu.Unlock()
	}
}

// Clear empties both L1 and L2.
func (c *PersistentCache) Clear() {
	if c == nil {
		return
	}
	c.mem.Clear()
	if c.db != nil {
		c.mu.Lock()
		_, _ = c.db.Exec("DELETE FROM replay_cache")
		c.mu.Unlock()
	}
}

// Close closes the underlying SQLite database.
func (c *PersistentCache) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.Close()
}

// SizeOnDisk returns the row count in the SQLite table. 0 if no DB.
func (c *PersistentCache) SizeOnDisk() int {
	if c == nil || c.db == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var n int
	_ = c.db.QueryRow("SELECT COUNT(*) FROM replay_cache").Scan(&n)
	return n
}
