package session

import (
	"container/list"
	"os"
	"sync"
	"time"
)

// ReplayCache caches parsed records per file path. Entries are invalidated
// when the file's modtime changes. Capped via LRU eviction.
type ReplayCache struct {
	mu       sync.Mutex
	items    map[string]*list.Element
	order    *list.List
	capacity int
}

type replayEntry struct {
	path    string
	modtime time.Time
	records []Record
}

// NewReplayCache returns a cache with the given capacity (0 → 64 default).
func NewReplayCache(capacity int) *ReplayCache {
	if capacity <= 0 {
		capacity = 64
	}
	return &ReplayCache{
		items:    map[string]*list.Element{},
		order:    list.New(),
		capacity: capacity,
	}
}

// Get returns the cached records if present and the file's modtime hasn't
// changed; otherwise reads + parses the file, stores, evicts oldest if at cap.
func (c *ReplayCache) Get(path string) ([]Record, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	modtime := info.ModTime()

	c.mu.Lock()
	if el, ok := c.items[path]; ok {
		entry := el.Value.(*replayEntry)
		if entry.modtime.Equal(modtime) {
			c.order.MoveToFront(el)
			c.mu.Unlock()
			return entry.records, nil
		}
		// modtime drifted; evict and refetch below
		c.order.Remove(el)
		delete(c.items, path)
	}
	c.mu.Unlock()

	records, err := Replay(path)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	// Evict oldest if at cap (re-check after possibly racing acquires)
	for c.order.Len() >= c.capacity {
		oldest := c.order.Back()
		if oldest == nil {
			break
		}
		c.order.Remove(oldest)
		delete(c.items, oldest.Value.(*replayEntry).path)
	}
	entry := &replayEntry{path: path, modtime: modtime, records: records}
	el := c.order.PushFront(entry)
	c.items[path] = el
	return records, nil
}

// Invalidate removes a single path from the cache (used by /sessions delete).
func (c *ReplayCache) Invalidate(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[path]; ok {
		c.order.Remove(el)
		delete(c.items, path)
	}
}

// Clear empties the cache (used by /sessions search-rebuild-index).
func (c *ReplayCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = map[string]*list.Element{}
	c.order = list.New()
}

// Size returns the current entry count.
func (c *ReplayCache) Size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}
