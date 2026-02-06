package idempotency

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStore_CheckAndSet(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	key := "test-key-1"
	ttl := time.Hour

	// First call should succeed (key is new)
	isNew, err := store.CheckAndSet(ctx, key, ttl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isNew {
		t.Error("expected isNew=true for first call")
	}

	// Second call with same key should fail (duplicate)
	isNew, err = store.CheckAndSet(ctx, key, ttl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isNew {
		t.Error("expected isNew=false for duplicate call")
	}

	// Different key should succeed
	key2 := "test-key-2"
	isNew, err = store.CheckAndSet(ctx, key2, ttl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isNew {
		t.Error("expected isNew=true for different key")
	}
}

func TestMemoryStore_CheckAndSet_ExpiredKey(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	key := "test-key-expired"
	shortTTL := 10 * time.Millisecond

	// Set key with short TTL
	isNew, err := store.CheckAndSet(ctx, key, shortTTL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isNew {
		t.Error("expected isNew=true for first call")
	}

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Key should be usable again after expiration
	isNew, err = store.CheckAndSet(ctx, key, shortTTL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isNew {
		t.Error("expected isNew=true after expiration")
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	key := "test-key-delete"
	ttl := time.Hour

	// Set key
	_, _ = store.CheckAndSet(ctx, key, ttl)

	// Delete key
	err := store.Delete(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Key should be usable again after deletion
	isNew, err := store.CheckAndSet(ctx, key, ttl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isNew {
		t.Error("expected isNew=true after deletion")
	}
}

func TestMemoryStore_Cleanup(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	shortTTL := 10 * time.Millisecond
	longTTL := time.Hour

	// Add keys with different TTLs
	_, _ = store.CheckAndSet(ctx, "short-1", shortTTL)
	_, _ = store.CheckAndSet(ctx, "short-2", shortTTL)
	_, _ = store.CheckAndSet(ctx, "long-1", longTTL)

	if store.Len() != 3 {
		t.Errorf("expected 3 entries, got %d", store.Len())
	}

	// Wait for short TTL keys to expire
	time.Sleep(20 * time.Millisecond)

	// Manual cleanup
	store.cleanup()

	if store.Len() != 1 {
		t.Errorf("expected 1 entry after cleanup, got %d", store.Len())
	}
}

func TestMemoryStore_Concurrent(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	ttl := time.Hour
	key := "concurrent-key"

	// Run multiple goroutines trying to set the same key
	results := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			isNew, err := store.CheckAndSet(ctx, key, ttl)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				results <- false
				return
			}
			results <- isNew
		}()
	}

	// Count how many succeeded
	newCount := 0
	for i := 0; i < 10; i++ {
		if <-results {
			newCount++
		}
	}

	// Exactly one goroutine should have succeeded
	if newCount != 1 {
		t.Errorf("expected exactly 1 success, got %d", newCount)
	}
}

func TestMemoryStore_DeleteNonExistent(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()

	// Deleting a non-existent key should not error
	err := store.Delete(ctx, "non-existent")
	if err != nil {
		t.Errorf("unexpected error deleting non-existent key: %v", err)
	}
}

func TestMemoryStore_ContextCanceled(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	// Create an already-canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Operations should still work (memory store doesn't check context)
	isNew, err := store.CheckAndSet(ctx, "key", time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isNew {
		t.Error("expected isNew=true")
	}
}

func BenchmarkMemoryStore_CheckAndSet(b *testing.B) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	ttl := time.Hour

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "benchmark-key"
		store.CheckAndSet(ctx, key, ttl)
		store.Delete(ctx, key)
	}
}

func BenchmarkMemoryStore_CheckAndSet_Parallel(b *testing.B) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	ttl := time.Hour

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := "benchmark-key"
			store.CheckAndSet(ctx, key, ttl)
			i++
		}
	})
}
