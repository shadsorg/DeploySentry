package deploysentry

import (
	"sync"
	"time"
)

// cacheEntry wraps a cached Flag with a timestamp for TTL expiry.
type cacheEntry struct {
	flag      Flag
	storedAt  time.Time
}

// flagCache is a thread-safe in-memory cache for feature flags, keyed by
// flag key. Entries are considered stale after the configured TTL, but stale
// entries are still returned when offline mode is active and the API is
// unreachable.
type flagCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
	ttl     time.Duration
}

// newFlagCache creates a cache with the given TTL. A zero TTL means entries
// never expire.
func newFlagCache(ttl time.Duration) *flagCache {
	return &flagCache{
		entries: make(map[string]cacheEntry),
		ttl:     ttl,
	}
}

// get retrieves a flag from the cache. The second return value indicates
// whether the entry was found, and the third indicates whether it is still
// fresh (within TTL).
func (c *flagCache) get(key string) (Flag, bool, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return Flag{}, false, false
	}

	fresh := c.ttl == 0 || time.Since(entry.storedAt) < c.ttl
	return entry.flag, true, fresh
}

// set stores or updates a flag in the cache.
func (c *flagCache) set(flag Flag) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[flag.Key] = cacheEntry{
		flag:     flag,
		storedAt: time.Now(),
	}
}

// clear removes all entries from the cache.
func (c *flagCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]cacheEntry)
}

// setAll replaces the entire cache with the provided flags.
func (c *flagCache) setAll(flags []Flag) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	c.entries = make(map[string]cacheEntry, len(flags))
	for _, f := range flags {
		c.entries[f.Key] = cacheEntry{
			flag:     f,
			storedAt: now,
		}
	}
}

// all returns a snapshot of every cached flag.
func (c *flagCache) all() []Flag {
	c.mu.RLock()
	defer c.mu.RUnlock()

	flags := make([]Flag, 0, len(c.entries))
	for _, entry := range c.entries {
		flags = append(flags, entry.flag)
	}
	return flags
}

// byCategory returns all cached flags matching the given category.
func (c *flagCache) byCategory(cat FlagCategory) []Flag {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var flags []Flag
	for _, entry := range c.entries {
		if entry.flag.Category == cat || entry.flag.Metadata.Category == cat {
			flags = append(flags, entry.flag)
		}
	}
	return flags
}

// expired returns all cached flags whose ExpiresAt is in the past.
func (c *flagCache) expired() []Flag {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	var flags []Flag
	for _, entry := range c.entries {
		if entry.flag.ExpiresAt != nil && entry.flag.ExpiresAt.Before(now) {
			flags = append(flags, entry.flag)
		}
	}
	return flags
}

// owners returns the owners for the given flag key, or nil if not found.
func (c *flagCache) owners(key string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil
	}
	return entry.flag.Owners
}
