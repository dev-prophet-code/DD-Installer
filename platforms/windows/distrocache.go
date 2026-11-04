package windows

import (
	"sync"
	"time"
)

// listCache remembers the result of an expensive lookup (spawning wsl.exe) for
// a short time. The wizard asks about WSL on every click on the target page;
// without a cache that would start a new process each time.
type listCache struct {
	mu  sync.Mutex
	at  time.Time
	val []string
	ok  bool
	ttl time.Duration
	now func() time.Time
}

func newListCache(ttl time.Duration) *listCache {
	return &listCache{ttl: ttl, now: time.Now}
}

// get returns the cached value or calls load.
func (c *listCache) get(load func() []string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ok && c.now().Sub(c.at) < c.ttl {
		return append([]string(nil), c.val...)
	}
	c.val = load()
	c.at = c.now()
	c.ok = true
	return append([]string(nil), c.val...)
}

// invalidate forces the next get to reload (after WSL was installed).
func (c *listCache) invalidate() {
	c.mu.Lock()
	c.ok = false
	c.mu.Unlock()
}
