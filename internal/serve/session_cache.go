package serve

import (
	"sync"
	"time"

	"github.com/ricardo/anthrogo/pkg/query"
)

const maxCachedSessions = 32

type cacheEntry struct {
	engine     *query.Engine
	lastAccess time.Time
}

// sessionCache holds a bounded map of session-id → Engine, evicting the
// least-recently-used entry when the cap is reached.
type sessionCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
}

func newSessionCache() *sessionCache {
	return &sessionCache{entries: make(map[string]*cacheEntry, maxCachedSessions)}
}

// GetOrCreate returns the cached Engine for sessionID, or calls builder to
// construct one. The entire get→build→put sequence is performed under the
// write lock so that concurrent calls for the same ID always produce exactly
// one Engine (no duplicate construction).
func (c *sessionCache) GetOrCreate(sessionID string, builder func() (*query.Engine, error)) (*query.Engine, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[sessionID]; ok {
		e.lastAccess = time.Now()
		return e.engine, nil
	}
	eng, err := builder()
	if err != nil {
		return nil, err
	}
	if len(c.entries) >= maxCachedSessions {
		c.evictOldestLocked()
	}
	c.entries[sessionID] = &cacheEntry{engine: eng, lastAccess: time.Now()}
	return eng, nil
}

// delete removes sessionID from the cache if present.
func (c *sessionCache) delete(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, sessionID)
}

// evictOldestLocked must be called with c.mu held for writing.
func (c *sessionCache) evictOldestLocked() {
	var oldest string
	var oldestTime time.Time
	for id, e := range c.entries {
		if oldest == "" || e.lastAccess.Before(oldestTime) {
			oldest = id
			oldestTime = e.lastAccess
		}
	}
	if oldest != "" {
		delete(c.entries, oldest)
	}
}
