// Package site provides site resolution and enrichment for alerts.
package site

import (
	"sync"
	"time"
)

// CacheConfig holds configuration for the site cache.
type CacheConfig struct {
	TTL            time.Duration // Time-to-live for cached entries
	CleanupInterval time.Duration // Interval for cleaning expired entries
	MaxSize        int           // Maximum number of entries (0 = unlimited)
}

// DefaultCacheConfig returns the default cache configuration.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		TTL:            5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
		MaxSize:        1000,
	}
}

// cacheEntry represents a cached site entry.
type cacheEntry struct {
	site      *Site
	expiresAt time.Time
}

// Cache provides an in-memory cache for site lookups.
type Cache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	config  CacheConfig
	stopCh  chan struct{}
}

// NewCache creates a new site cache with the given configuration.
func NewCache(config CacheConfig) *Cache {
	c := &Cache{
		entries: make(map[string]*cacheEntry),
		config:  config,
		stopCh:  make(chan struct{}),
	}

	// Start cleanup goroutine
	if config.CleanupInterval > 0 {
		go c.cleanupLoop()
	}

	return c
}

// Get retrieves a site from the cache by code.
// Returns nil if not found or expired.
func (c *Cache) Get(code string) *Site {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[code]
	if !ok {
		return nil
	}

	if time.Now().After(entry.expiresAt) {
		return nil
	}

	return entry.site
}

// GetByID retrieves a site from the cache by ID.
// This is a slower operation as it requires iterating over all entries.
func (c *Cache) GetByID(id string) *Site {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	for _, entry := range c.entries {
		if entry.site.ID == id && now.Before(entry.expiresAt) {
			return entry.site
		}
	}

	return nil
}

// Set stores a site in the cache.
func (c *Cache) Set(site *Site) {
	if site == nil || site.Code == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check max size limit
	if c.config.MaxSize > 0 && len(c.entries) >= c.config.MaxSize {
		// Evict oldest entry
		c.evictOldest()
	}

	c.entries[site.Code] = &cacheEntry{
		site:      site,
		expiresAt: time.Now().Add(c.config.TTL),
	}
}

// SetMultiple stores multiple sites in the cache.
func (c *Cache) SetMultiple(sites []*Site) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expiresAt := now.Add(c.config.TTL)

	for _, site := range sites {
		if site == nil || site.Code == "" {
			continue
		}

		// Check max size limit
		if c.config.MaxSize > 0 && len(c.entries) >= c.config.MaxSize {
			c.evictOldest()
		}

		c.entries[site.Code] = &cacheEntry{
			site:      site,
			expiresAt: expiresAt,
		}
	}
}

// Invalidate removes a site from the cache by code.
func (c *Cache) Invalidate(code string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, code)
}

// InvalidateByID removes a site from the cache by ID.
func (c *Cache) InvalidateByID(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for code, entry := range c.entries {
		if entry.site.ID == id {
			delete(c.entries, code)
			return
		}
	}
}

// InvalidateAll clears all entries from the cache.
func (c *Cache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*cacheEntry)
}

// Size returns the current number of entries in the cache.
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.entries)
}

// Stop stops the cache cleanup goroutine.
func (c *Cache) Stop() {
	close(c.stopCh)
}

// evictOldest removes the oldest entry from the cache.
// Must be called with the lock held.
func (c *Cache) evictOldest() {
	var oldestCode string
	var oldestTime time.Time

	for code, entry := range c.entries {
		if oldestCode == "" || entry.expiresAt.Before(oldestTime) {
			oldestCode = code
			oldestTime = entry.expiresAt
		}
	}

	if oldestCode != "" {
		delete(c.entries, oldestCode)
	}
}

// cleanupLoop periodically removes expired entries.
func (c *Cache) cleanupLoop() {
	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopCh:
			return
		}
	}
}

// cleanup removes all expired entries from the cache.
func (c *Cache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for code, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, code)
		}
	}
}
