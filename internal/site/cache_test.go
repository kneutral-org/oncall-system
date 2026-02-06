package site

import (
	"testing"
	"time"
)

func TestCache_GetSet(t *testing.T) {
	config := CacheConfig{
		TTL:             1 * time.Minute,
		CleanupInterval: 0, // Disable cleanup for testing
		MaxSize:         100,
	}
	cache := NewCache(config)
	defer cache.Stop()

	site := &Site{
		ID:   "site-1",
		Code: "dfw1",
		Name: "Dallas DC 1",
	}

	// Initially should not be in cache
	if got := cache.Get("dfw1"); got != nil {
		t.Errorf("Expected nil for uncached site, got %v", got)
	}

	// Set the site
	cache.Set(site)

	// Should now be retrievable
	got := cache.Get("dfw1")
	if got == nil {
		t.Fatal("Expected site to be cached")
	}
	if got.ID != site.ID {
		t.Errorf("Got ID %v, want %v", got.ID, site.ID)
	}
	if got.Name != site.Name {
		t.Errorf("Got Name %v, want %v", got.Name, site.Name)
	}
}

func TestCache_GetByID(t *testing.T) {
	config := CacheConfig{
		TTL:             1 * time.Minute,
		CleanupInterval: 0,
		MaxSize:         100,
	}
	cache := NewCache(config)
	defer cache.Stop()

	site := &Site{
		ID:   "site-1",
		Code: "dfw1",
		Name: "Dallas DC 1",
	}

	cache.Set(site)

	// Should find by ID
	got := cache.GetByID("site-1")
	if got == nil {
		t.Fatal("Expected site to be found by ID")
	}
	if got.Code != site.Code {
		t.Errorf("Got Code %v, want %v", got.Code, site.Code)
	}

	// Should not find non-existent ID
	if got := cache.GetByID("non-existent"); got != nil {
		t.Errorf("Expected nil for non-existent ID, got %v", got)
	}
}

func TestCache_Invalidate(t *testing.T) {
	config := CacheConfig{
		TTL:             1 * time.Minute,
		CleanupInterval: 0,
		MaxSize:         100,
	}
	cache := NewCache(config)
	defer cache.Stop()

	site := &Site{
		ID:   "site-1",
		Code: "dfw1",
		Name: "Dallas DC 1",
	}

	cache.Set(site)

	// Verify it's cached
	if got := cache.Get("dfw1"); got == nil {
		t.Fatal("Expected site to be cached")
	}

	// Invalidate by code
	cache.Invalidate("dfw1")

	// Should no longer be cached
	if got := cache.Get("dfw1"); got != nil {
		t.Errorf("Expected nil after invalidation, got %v", got)
	}
}

func TestCache_InvalidateByID(t *testing.T) {
	config := CacheConfig{
		TTL:             1 * time.Minute,
		CleanupInterval: 0,
		MaxSize:         100,
	}
	cache := NewCache(config)
	defer cache.Stop()

	site := &Site{
		ID:   "site-1",
		Code: "dfw1",
		Name: "Dallas DC 1",
	}

	cache.Set(site)

	// Invalidate by ID
	cache.InvalidateByID("site-1")

	// Should no longer be cached
	if got := cache.Get("dfw1"); got != nil {
		t.Errorf("Expected nil after invalidation by ID, got %v", got)
	}
}

func TestCache_InvalidateAll(t *testing.T) {
	config := CacheConfig{
		TTL:             1 * time.Minute,
		CleanupInterval: 0,
		MaxSize:         100,
	}
	cache := NewCache(config)
	defer cache.Stop()

	sites := []*Site{
		{ID: "site-1", Code: "dfw1", Name: "Dallas DC 1"},
		{ID: "site-2", Code: "nyc2", Name: "New York DC 2"},
		{ID: "site-3", Code: "lax1", Name: "Los Angeles DC 1"},
	}

	for _, s := range sites {
		cache.Set(s)
	}

	// Verify all are cached
	if cache.Size() != 3 {
		t.Errorf("Expected size 3, got %d", cache.Size())
	}

	// Invalidate all
	cache.InvalidateAll()

	// Should be empty
	if cache.Size() != 0 {
		t.Errorf("Expected size 0 after InvalidateAll, got %d", cache.Size())
	}

	// None should be retrievable
	for _, s := range sites {
		if got := cache.Get(s.Code); got != nil {
			t.Errorf("Expected nil for %s after InvalidateAll", s.Code)
		}
	}
}

func TestCache_TTLExpiration(t *testing.T) {
	config := CacheConfig{
		TTL:             50 * time.Millisecond, // Very short TTL for testing
		CleanupInterval: 0,
		MaxSize:         100,
	}
	cache := NewCache(config)
	defer cache.Stop()

	site := &Site{
		ID:   "site-1",
		Code: "dfw1",
		Name: "Dallas DC 1",
	}

	cache.Set(site)

	// Should be cached immediately
	if got := cache.Get("dfw1"); got == nil {
		t.Fatal("Expected site to be cached immediately")
	}

	// Wait for TTL to expire
	time.Sleep(60 * time.Millisecond)

	// Should return nil for expired entry
	if got := cache.Get("dfw1"); got != nil {
		t.Errorf("Expected nil for expired entry, got %v", got)
	}
}

func TestCache_MaxSize(t *testing.T) {
	config := CacheConfig{
		TTL:             1 * time.Minute,
		CleanupInterval: 0,
		MaxSize:         3, // Small max size for testing
	}
	cache := NewCache(config)
	defer cache.Stop()

	// Add 4 sites (exceeds max)
	sites := []*Site{
		{ID: "site-1", Code: "dfw1", Name: "Dallas DC 1"},
		{ID: "site-2", Code: "nyc2", Name: "New York DC 2"},
		{ID: "site-3", Code: "lax1", Name: "Los Angeles DC 1"},
		{ID: "site-4", Code: "ord3", Name: "Chicago DC 3"},
	}

	for _, s := range sites {
		cache.Set(s)
	}

	// Size should not exceed max
	if cache.Size() > config.MaxSize {
		t.Errorf("Cache size %d exceeds max %d", cache.Size(), config.MaxSize)
	}
}

func TestCache_SetMultiple(t *testing.T) {
	config := CacheConfig{
		TTL:             1 * time.Minute,
		CleanupInterval: 0,
		MaxSize:         100,
	}
	cache := NewCache(config)
	defer cache.Stop()

	sites := []*Site{
		{ID: "site-1", Code: "dfw1", Name: "Dallas DC 1"},
		{ID: "site-2", Code: "nyc2", Name: "New York DC 2"},
		{ID: "site-3", Code: "lax1", Name: "Los Angeles DC 1"},
	}

	cache.SetMultiple(sites)

	// All should be cached
	for _, s := range sites {
		got := cache.Get(s.Code)
		if got == nil {
			t.Errorf("Expected %s to be cached", s.Code)
		}
	}

	if cache.Size() != 3 {
		t.Errorf("Expected size 3, got %d", cache.Size())
	}
}

func TestCache_NilSite(t *testing.T) {
	config := CacheConfig{
		TTL:             1 * time.Minute,
		CleanupInterval: 0,
		MaxSize:         100,
	}
	cache := NewCache(config)
	defer cache.Stop()

	// Setting nil should not panic or add entry
	cache.Set(nil)

	if cache.Size() != 0 {
		t.Errorf("Expected size 0 after setting nil, got %d", cache.Size())
	}
}

func TestCache_EmptyCode(t *testing.T) {
	config := CacheConfig{
		TTL:             1 * time.Minute,
		CleanupInterval: 0,
		MaxSize:         100,
	}
	cache := NewCache(config)
	defer cache.Stop()

	// Setting site with empty code should not add entry
	site := &Site{
		ID:   "site-1",
		Code: "", // Empty code
		Name: "Test Site",
	}

	cache.Set(site)

	if cache.Size() != 0 {
		t.Errorf("Expected size 0 after setting site with empty code, got %d", cache.Size())
	}
}
