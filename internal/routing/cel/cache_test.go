package cel

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCache(t *testing.T) {
	cache, err := NewCache(100)
	require.NoError(t, err)
	require.NotNil(t, cache)
	assert.Equal(t, 100, cache.Capacity())
	assert.Equal(t, 0, cache.Size())
}

func TestNewCache_DefaultCapacity(t *testing.T) {
	cache, err := NewCache(0)
	require.NoError(t, err)
	assert.Equal(t, 1000, cache.Capacity())
}

func TestCache_GetOrCompile(t *testing.T) {
	cache, err := NewCache(10)
	require.NoError(t, err)

	expression := `alert_labels["severity"] == "critical"`

	// First call - should compile
	entry1, err := cache.GetOrCompile(expression)
	require.NoError(t, err)
	require.NotNil(t, entry1)
	assert.Equal(t, expression, entry1.Expression)
	assert.Equal(t, int64(0), entry1.HitCount)

	// Second call - should get from cache
	entry2, err := cache.GetOrCompile(expression)
	require.NoError(t, err)
	require.NotNil(t, entry2)
	assert.Equal(t, int64(1), entry2.HitCount)

	// Should be the same program
	assert.Equal(t, entry1.Expression, entry2.Expression)
}

func TestCache_Get(t *testing.T) {
	cache, err := NewCache(10)
	require.NoError(t, err)

	expression := `alert_labels["severity"] == "critical"`

	// Get non-existent
	entry := cache.Get(expression)
	assert.Nil(t, entry)

	// Add entry
	_, err = cache.GetOrCompile(expression)
	require.NoError(t, err)

	// Get existing
	entry = cache.Get(expression)
	assert.NotNil(t, entry)
	assert.Equal(t, expression, entry.Expression)
}

func TestCache_Put(t *testing.T) {
	cache, err := NewCache(10)
	require.NoError(t, err)

	expression := `alert_labels["severity"] == "critical"`

	// Compile manually
	entry, err := cache.Compile(expression)
	require.NoError(t, err)

	// Put in cache
	cache.Put(expression, entry)

	// Verify it's in cache
	cached := cache.Get(expression)
	assert.NotNil(t, cached)
	assert.Equal(t, 1, cache.Size())
}

func TestCache_Delete(t *testing.T) {
	cache, err := NewCache(10)
	require.NoError(t, err)

	expression := `alert_labels["severity"] == "critical"`

	// Add entry
	_, err = cache.GetOrCompile(expression)
	require.NoError(t, err)
	assert.Equal(t, 1, cache.Size())

	// Delete entry
	cache.Delete(expression)
	assert.Equal(t, 0, cache.Size())

	// Verify it's gone
	entry := cache.Get(expression)
	assert.Nil(t, entry)
}

func TestCache_Clear(t *testing.T) {
	cache, err := NewCache(10)
	require.NoError(t, err)

	// Add multiple entries
	expressions := []string{
		`alert_labels["severity"] == "critical"`,
		`alert_labels["environment"] == "production"`,
		`hasLabel(alert_labels, "team")`,
	}

	for _, expr := range expressions {
		_, err := cache.GetOrCompile(expr)
		require.NoError(t, err)
	}

	assert.Equal(t, 3, cache.Size())

	// Clear cache
	cache.Clear()
	assert.Equal(t, 0, cache.Size())
}

func TestCache_LRUEviction(t *testing.T) {
	cache, err := NewCache(3)
	require.NoError(t, err)

	expressions := []string{
		`alert_labels["a"] == "1"`,
		`alert_labels["b"] == "2"`,
		`alert_labels["c"] == "3"`,
		`alert_labels["d"] == "4"`,
	}

	// Add first 3 entries
	for i := 0; i < 3; i++ {
		_, err := cache.GetOrCompile(expressions[i])
		require.NoError(t, err)
	}
	assert.Equal(t, 3, cache.Size())

	// Access first entry to make it recently used
	cache.Get(expressions[0])

	// Add 4th entry - should evict second entry (LRU)
	_, err = cache.GetOrCompile(expressions[3])
	require.NoError(t, err)
	assert.Equal(t, 3, cache.Size())

	// First entry should still be there (was accessed)
	assert.NotNil(t, cache.Get(expressions[0]))

	// Second entry should be evicted (was LRU)
	assert.Nil(t, cache.Get(expressions[1]))

	// Third and fourth entries should be there
	assert.NotNil(t, cache.Get(expressions[2]))
	assert.NotNil(t, cache.Get(expressions[3]))
}

func TestCache_Stats(t *testing.T) {
	cache, err := NewCache(10)
	require.NoError(t, err)

	expression := `alert_labels["severity"] == "critical"`

	// Add and access entry
	_, err = cache.GetOrCompile(expression)
	require.NoError(t, err)

	// Access multiple times
	for i := 0; i < 5; i++ {
		cache.Get(expression)
	}

	stats := cache.Stats()
	assert.Equal(t, 1, stats.Size)
	assert.Equal(t, 10, stats.Capacity)
	assert.Equal(t, int64(5), stats.TotalHits)
	assert.False(t, stats.OldestAccess.IsZero())
	assert.False(t, stats.NewestAccess.IsZero())
}

func TestCache_Keys(t *testing.T) {
	cache, err := NewCache(10)
	require.NoError(t, err)

	expressions := []string{
		`alert_labels["a"] == "1"`,
		`alert_labels["b"] == "2"`,
		`alert_labels["c"] == "3"`,
	}

	for _, expr := range expressions {
		_, err := cache.GetOrCompile(expr)
		require.NoError(t, err)
	}

	keys := cache.Keys()
	assert.Len(t, keys, 3)

	// Keys should be in MRU order (most recent first)
	assert.Equal(t, expressions[2], keys[0])
	assert.Equal(t, expressions[1], keys[1])
	assert.Equal(t, expressions[0], keys[2])
}

func TestCache_CompileError(t *testing.T) {
	cache, err := NewCache(10)
	require.NoError(t, err)

	invalidExpr := `alert_labels[`

	entry, err := cache.GetOrCompile(invalidExpr)
	assert.Error(t, err)
	assert.Nil(t, entry)

	// Should not be in cache
	assert.Equal(t, 0, cache.Size())
}

func TestCache_ConcurrentAccess(t *testing.T) {
	cache, err := NewCache(100)
	require.NoError(t, err)

	expressions := []string{
		`alert_labels["a"] == "1"`,
		`alert_labels["b"] == "2"`,
		`alert_labels["c"] == "3"`,
		`alert_labels["d"] == "4"`,
		`alert_labels["e"] == "5"`,
	}

	var wg sync.WaitGroup
	concurrency := 50

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			expr := expressions[idx%len(expressions)]

			// GetOrCompile
			entry, err := cache.GetOrCompile(expr)
			assert.NoError(t, err)
			assert.NotNil(t, entry)

			// Get
			cache.Get(expr)

			// Small delay
			time.Sleep(time.Millisecond)
		}(i)
	}

	wg.Wait()

	// All expressions should be in cache
	assert.Equal(t, len(expressions), cache.Size())
}

func TestCache_Compile(t *testing.T) {
	cache, err := NewCache(10)
	require.NoError(t, err)

	expression := `alert_labels["severity"] == "critical"`

	// Compile without caching
	entry, err := cache.Compile(expression)
	require.NoError(t, err)
	require.NotNil(t, entry)

	// Should not be in cache
	assert.Equal(t, 0, cache.Size())

	// Entry should be usable
	assert.NotNil(t, entry.Program)
	assert.NotNil(t, entry.AST)
}

func TestCache_Env(t *testing.T) {
	cache, err := NewCache(10)
	require.NoError(t, err)

	env := cache.Env()
	assert.NotNil(t, env)
}
