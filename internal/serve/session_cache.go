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

// get returns the Engine for sessionID and updates its last-access time.
// Returns nil if the session is not cached.
func (c *sessionCache) get(sessionID string) *query.Engine {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[sessionID]
	if !ok {
		return nil
	}
	e.lastAccess = time.Now()
	return e.engine
}

// put stores engine under sessionID. If the cache is at capacity it first
// evicts the entry with the oldest last-access time.
func (c *sessionCache) put(sessionID string, engine *query.Engine) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// If already present just refresh access time.
	if e, ok := c.entries[sessionID]; ok {
		e.engine = engine
		e.lastAccess = time.Now()
		return
	}
	if len(c.entries) >= maxCachedSessions {
		c.evictOldestLocked()
	}
	c.entries[sessionID] = &cacheEntry{engine: engine, lastAccess: time.Now()}
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
