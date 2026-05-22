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

	"github.com/ricardo/anthrogo/internal/version"
)

// cacheSchemaVersion is the SQLite user_version this build manages.
// v0 = fresh DB, v1 = pre-M13.4, v2 = M13.4 (anthrogo_version column added).
const cacheSchemaVersion = 2

// maxPersistentCacheRows caps the number of rows retained in the replay_cache
// table. On every Insert we run a trim that keeps the most-recently-cached
// rows up to this cap. Unbounded growth was a real concern for long-lived
// installations that accumulate one row per session file ever touched.
const maxPersistentCacheRows = 2000

// PersistentCache is a SQLite-backed cache of parsed []Record keyed by
// (path, modtime). On miss or stale modtime, it falls through to Replay
// + stores. Falls back to a no-op (in-memory only) if the DB can't open.
type PersistentCache struct {
	mu  sync.Mutex
	db  *sql.DB
	mem *ReplayCache // L1 cache; PersistentCache is L2
}

// openAndMigrate checks PRAGMA user_version and applies schema migrations:
//   - v0 (fresh): create table with anthrogo_version column, set user_version=2
//   - v1 (pre-M13.4): ALTER TABLE to add anthrogo_version column, set user_version=2
//   - v2: no-op
//   - v>2: warn to stderr, proceed (forward-compatible best-effort)
func openAndMigrate(db *sql.DB) error {
	var v int
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return err
	}

	switch {
	case v == 0:
		// Fresh DB: create table with anthrogo_version column.
		if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS replay_cache (
			path TEXT PRIMARY KEY,
			modtime INTEGER NOT NULL,
			records_json BLOB NOT NULL,
			cached_at INTEGER NOT NULL,
			anthrogo_version TEXT
		)`); err != nil {
			return err
		}
		if _, err := db.Exec("PRAGMA user_version = 2"); err != nil {
			return err
		}
	case v == 1:
		// Migrate v1 → v2: add anthrogo_version column.
		if _, err := db.Exec(`ALTER TABLE replay_cache ADD COLUMN anthrogo_version TEXT`); err != nil {
			return err
		}
		if _, err := db.Exec("PRAGMA user_version = 2"); err != nil {
			return err
		}
	case v > cacheSchemaVersion:
		fmt.Fprintf(os.Stderr, "[session-cache] DB user_version=%d unknown (anthrogo supports %d); using read-only access where possible\n", v, cacheSchemaVersion)
	}
	return nil
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
	// WAL mode lets concurrent readers run alongside one writer, and
	// synchronous=NORMAL is the documented sweet spot for WAL: ACI but not D
	// across power loss within ~milliseconds — acceptable for a derived cache
	// we can always rebuild from JSONL.
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		fmt.Fprintf(os.Stderr, "session cache: WAL: %v\n", err)
	}
	if _, err := db.Exec("PRAGMA synchronous = NORMAL"); err != nil {
		fmt.Fprintf(os.Stderr, "session cache: synchronous: %v\n", err)
	}
	if err := openAndMigrate(db); err != nil {
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
			`INSERT INTO replay_cache (path, modtime, records_json, cached_at, anthrogo_version)
			 VALUES (?, ?, ?, ?, ?)
			 ON CONFLICT(path) DO UPDATE SET
			    modtime=excluded.modtime,
			    records_json=excluded.records_json,
			    cached_at=excluded.cached_at,
			    anthrogo_version=excluded.anthrogo_version`,
			path, modtimeUnix, blob, time.Now().Unix(), version.Version,
		)
		// Trim oldest rows when over the cap.
		_, _ = c.db.Exec(
			`DELETE FROM replay_cache WHERE path IN (
				SELECT path FROM replay_cache ORDER BY cached_at ASC
				LIMIT MAX(0, (SELECT COUNT(*) FROM replay_cache) - ?))`,
			maxPersistentCacheRows,
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
