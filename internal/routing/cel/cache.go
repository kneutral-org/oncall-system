package cel

import (
	"container/list"
	"sync"
	"time"

	"github.com/google/cel-go/cel"
)

// CacheEntry represents a cached compiled expression.
type CacheEntry struct {
	Expression string
	Program    cel.Program
	AST        *cel.Ast
	CreatedAt  time.Time
	LastUsedAt time.Time
	HitCount   int64
}

// Cache provides thread-safe LRU caching for compiled CEL expressions.
type Cache struct {
	mu       sync.RWMutex
	capacity int
	items    map[string]*list.Element
	order    *list.List
	env      *cel.Env
}

type cacheItem struct {
	key   string
	entry *CacheEntry
}

// NewCache creates a new expression cache with the specified capacity.
func NewCache(capacity int) (*Cache, error) {
	env, err := NewStandardEnvironment()
	if err != nil {
		return nil, err
	}

	if capacity <= 0 {
		capacity = 1000 // Default capacity
	}

	return &Cache{
		capacity: capacity,
		items:    make(map[string]*list.Element),
		order:    list.New(),
		env:      env,
	}, nil
}

// NewCacheWithEnv creates a new expression cache with a custom environment.
func NewCacheWithEnv(capacity int, env *cel.Env) *Cache {
	if capacity <= 0 {
		capacity = 1000
	}

	return &Cache{
		capacity: capacity,
		items:    make(map[string]*list.Element),
		order:    list.New(),
		env:      env,
	}
}

// Get retrieves a compiled expression from the cache.
// Returns nil if not found.
func (c *Cache) Get(expression string) *CacheEntry {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[expression]; ok {
		// Move to front (most recently used)
		c.order.MoveToFront(elem)

		item := elem.Value.(*cacheItem)
		item.entry.LastUsedAt = time.Now()
		item.entry.HitCount++

		return item.entry
	}

	return nil
}

// GetOrCompile retrieves a compiled expression from cache, or compiles and caches it.
func (c *Cache) GetOrCompile(expression string) (*CacheEntry, error) {
	// First, try to get from cache with read lock
	c.mu.RLock()
	if elem, ok := c.items[expression]; ok {
		c.mu.RUnlock()

		// Upgrade to write lock to update access info
		c.mu.Lock()
		c.order.MoveToFront(elem)
		item := elem.Value.(*cacheItem)
		item.entry.LastUsedAt = time.Now()
		item.entry.HitCount++
		entry := item.entry
		c.mu.Unlock()

		return entry, nil
	}
	c.mu.RUnlock()

	// Not in cache, compile it
	ast, issues := c.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, issues.Err()
	}

	prg, err := c.env.Program(ast)
	if err != nil {
		return nil, err
	}

	entry := &CacheEntry{
		Expression: expression,
		Program:    prg,
		AST:        ast,
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		HitCount:   0,
	}

	// Add to cache
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check again in case another goroutine added it
	if elem, ok := c.items[expression]; ok {
		c.order.MoveToFront(elem)
		item := elem.Value.(*cacheItem)
		item.entry.LastUsedAt = time.Now()
		item.entry.HitCount++
		return item.entry, nil
	}

	// Evict if at capacity
	if c.order.Len() >= c.capacity {
		c.evictOldest()
	}

	// Add new entry
	item := &cacheItem{
		key:   expression,
		entry: entry,
	}
	elem := c.order.PushFront(item)
	c.items[expression] = elem

	return entry, nil
}

// Put adds or updates a compiled expression in the cache.
func (c *Cache) Put(expression string, entry *CacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[expression]; ok {
		// Update existing entry
		c.order.MoveToFront(elem)
		item := elem.Value.(*cacheItem)
		item.entry = entry
		return
	}

	// Evict if at capacity
	if c.order.Len() >= c.capacity {
		c.evictOldest()
	}

	// Add new entry
	item := &cacheItem{
		key:   expression,
		entry: entry,
	}
	elem := c.order.PushFront(item)
	c.items[expression] = elem
}

// Delete removes an expression from the cache.
func (c *Cache) Delete(expression string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[expression]; ok {
		c.order.Remove(elem)
		delete(c.items, expression)
	}
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.order = list.New()
}

// Size returns the number of entries in the cache.
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.order.Len()
}

// Capacity returns the maximum capacity of the cache.
func (c *Cache) Capacity() int {
	return c.capacity
}

// Stats returns cache statistics.
func (c *Cache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var totalHits int64
	var oldestAccess time.Time
	var newestAccess time.Time

	for elem := c.order.Back(); elem != nil; elem = elem.Prev() {
		item := elem.Value.(*cacheItem)
		totalHits += item.entry.HitCount

		if oldestAccess.IsZero() || item.entry.LastUsedAt.Before(oldestAccess) {
			oldestAccess = item.entry.LastUsedAt
		}
		if item.entry.LastUsedAt.After(newestAccess) {
			newestAccess = item.entry.LastUsedAt
		}
	}

	return CacheStats{
		Size:          c.order.Len(),
		Capacity:      c.capacity,
		TotalHits:     totalHits,
		OldestAccess:  oldestAccess,
		NewestAccess:  newestAccess,
	}
}

// CacheStats contains cache statistics.
type CacheStats struct {
	Size          int
	Capacity      int
	TotalHits     int64
	OldestAccess  time.Time
	NewestAccess  time.Time
}

// evictOldest removes the least recently used entry.
// Must be called with lock held.
func (c *Cache) evictOldest() {
	if elem := c.order.Back(); elem != nil {
		item := elem.Value.(*cacheItem)
		delete(c.items, item.key)
		c.order.Remove(elem)
	}
}

// Env returns the CEL environment used by this cache.
func (c *Cache) Env() *cel.Env {
	return c.env
}

// Compile compiles an expression and returns the compiled entry without caching.
func (c *Cache) Compile(expression string) (*CacheEntry, error) {
	ast, issues := c.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, issues.Err()
	}

	prg, err := c.env.Program(ast)
	if err != nil {
		return nil, err
	}

	return &CacheEntry{
		Expression: expression,
		Program:    prg,
		AST:        ast,
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		HitCount:   0,
	}, nil
}

// Keys returns all expression keys in the cache.
func (c *Cache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, c.order.Len())
	for elem := c.order.Front(); elem != nil; elem = elem.Next() {
		item := elem.Value.(*cacheItem)
		keys = append(keys, item.key)
	}
	return keys
}
