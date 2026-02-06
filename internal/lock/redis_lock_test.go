package lock

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// getTestRedisClient returns a Redis client for testing.
// Skips the test if Redis is not available.
func getTestRedisClient(t *testing.T) *redis.Client {
	t.Helper()

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use a separate DB for testing
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	// Clean up test keys
	t.Cleanup(func() {
		_ = client.FlushDB(context.Background())
		_ = client.Close()
	})

	return client
}

func TestRedisLock_Acquire(t *testing.T) {
	client := getTestRedisClient(t)
	ctx := context.Background()

	lock := NewRedisLock(client, "test:lock:acquire", 10*time.Second)

	// First acquire should succeed
	acquired, err := lock.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if !acquired {
		t.Error("Expected to acquire lock on first attempt")
	}
	if !lock.IsHeld() {
		t.Error("Expected IsHeld to return true after acquiring")
	}

	// Second acquire should fail (lock already held)
	lock2 := NewRedisLock(client, "test:lock:acquire", 10*time.Second, WithLockValue("different-value"))
	acquired2, err := lock2.Acquire(ctx)
	if err != nil {
		t.Fatalf("Second Acquire failed: %v", err)
	}
	if acquired2 {
		t.Error("Expected second acquire to fail")
	}
	if lock2.IsHeld() {
		t.Error("Expected IsHeld to return false for second lock")
	}
}

func TestRedisLock_Release(t *testing.T) {
	client := getTestRedisClient(t)
	ctx := context.Background()

	lock := NewRedisLock(client, "test:lock:release", 10*time.Second)

	// Acquire the lock
	acquired, err := lock.Acquire(ctx)
	if err != nil || !acquired {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Release the lock
	if err := lock.Release(ctx); err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	if lock.IsHeld() {
		t.Error("Expected IsHeld to return false after release")
	}

	// Another instance should now be able to acquire
	lock2 := NewRedisLock(client, "test:lock:release", 10*time.Second)
	acquired2, err := lock2.Acquire(ctx)
	if err != nil {
		t.Fatalf("Second Acquire failed: %v", err)
	}
	if !acquired2 {
		t.Error("Expected to acquire lock after release")
	}
}

func TestRedisLock_ReleaseOnlyOwnLock(t *testing.T) {
	client := getTestRedisClient(t)
	ctx := context.Background()

	lock1 := NewRedisLock(client, "test:lock:own", 10*time.Second, WithLockValue("instance-1"))
	lock2 := NewRedisLock(client, "test:lock:own", 10*time.Second, WithLockValue("instance-2"))

	// Lock1 acquires
	acquired, _ := lock1.Acquire(ctx)
	if !acquired {
		t.Fatal("Failed to acquire lock1")
	}

	// Lock2 tries to release lock1's lock (should be a no-op since lock2 doesn't hold it)
	// Note: lock2.held is false, so Release returns early
	if err := lock2.Release(ctx); err != nil {
		t.Fatalf("Release should not error: %v", err)
	}

	// Lock1 should still hold the lock
	if !lock1.IsHeld() {
		t.Error("Lock1 should still hold the lock")
	}

	// Verify by checking Redis directly
	val, err := client.Get(ctx, "test:lock:own").Result()
	if err != nil {
		t.Fatalf("Failed to get key from Redis: %v", err)
	}
	if val != "instance-1" {
		t.Errorf("Expected lock value 'instance-1', got '%s'", val)
	}
}

func TestRedisLock_Extend(t *testing.T) {
	client := getTestRedisClient(t)
	ctx := context.Background()

	lock := NewRedisLock(client, "test:lock:extend", 5*time.Second)

	// Acquire the lock
	acquired, err := lock.Acquire(ctx)
	if err != nil || !acquired {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Wait a bit
	time.Sleep(1 * time.Second)

	// Extend the lock
	if err := lock.Extend(ctx); err != nil {
		t.Fatalf("Extend failed: %v", err)
	}

	// Check TTL is refreshed (should be close to 5 seconds again)
	ttl, err := client.PTTL(ctx, "test:lock:extend").Result()
	if err != nil {
		t.Fatalf("Failed to get TTL: %v", err)
	}
	if ttl < 4*time.Second {
		t.Errorf("Expected TTL > 4s after extend, got %v", ttl)
	}
}

func TestRedisLock_ExtendNotHeld(t *testing.T) {
	client := getTestRedisClient(t)
	ctx := context.Background()

	lock := NewRedisLock(client, "test:lock:extend-notheld", 5*time.Second)

	// Try to extend without acquiring
	err := lock.Extend(ctx)
	if err != ErrLockNotHeld {
		t.Errorf("Expected ErrLockNotHeld, got: %v", err)
	}
}

func TestRedisLock_AutoExpire(t *testing.T) {
	client := getTestRedisClient(t)
	ctx := context.Background()

	// Use a very short TTL for testing
	lock := NewRedisLock(client, "test:lock:expire", 100*time.Millisecond)

	acquired, err := lock.Acquire(ctx)
	if err != nil || !acquired {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Wait for expiration
	time.Sleep(200 * time.Millisecond)

	// Another instance should be able to acquire now
	lock2 := NewRedisLock(client, "test:lock:expire", 10*time.Second)
	acquired2, err := lock2.Acquire(ctx)
	if err != nil {
		t.Fatalf("Second Acquire failed: %v", err)
	}
	if !acquired2 {
		t.Error("Expected to acquire lock after expiration")
	}
}

func TestRedisLock_ConcurrentAcquire(t *testing.T) {
	client := getTestRedisClient(t)
	ctx := context.Background()

	const numGoroutines = 10
	acquired := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			lock := NewRedisLock(client, "test:lock:concurrent", 10*time.Second,
				WithLockValue(time.Now().String()+string(rune(id))))
			got, _ := lock.Acquire(ctx)
			acquired <- got
		}(i)
	}

	// Only one goroutine should acquire the lock
	successCount := 0
	for i := 0; i < numGoroutines; i++ {
		if <-acquired {
			successCount++
		}
	}

	if successCount != 1 {
		t.Errorf("Expected exactly 1 goroutine to acquire lock, got %d", successCount)
	}
}

func TestRedisLock_KeyAndTTL(t *testing.T) {
	client := getTestRedisClient(t)

	key := "test:lock:getters"
	ttl := 30 * time.Second
	lock := NewRedisLock(client, key, ttl)

	if lock.Key() != key {
		t.Errorf("Expected key %q, got %q", key, lock.Key())
	}
	if lock.TTL() != ttl {
		t.Errorf("Expected TTL %v, got %v", ttl, lock.TTL())
	}
}
