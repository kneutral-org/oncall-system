package lock

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Common errors for distributed locking operations.
var (
	// ErrLockNotHeld is returned when trying to release or extend a lock that is not held.
	ErrLockNotHeld = errors.New("lock not held by this instance")
)

// RedisLock implements DistributedLock using Redis.
// It uses SET NX with expiration for atomic lock acquisition.
type RedisLock struct {
	client *redis.Client
	key    string
	value  string // unique identifier for this lock holder
	ttl    time.Duration

	mu   sync.RWMutex
	held bool
}

// RedisLockOption configures a RedisLock.
type RedisLockOption func(*RedisLock)

// WithLockValue sets a custom value for the lock.
// This should be unique per instance to prevent accidental release by other instances.
func WithLockValue(value string) RedisLockOption {
	return func(l *RedisLock) {
		l.value = value
	}
}

// NewRedisLock creates a new Redis-backed distributed lock.
// The key identifies the lock, and ttl sets the automatic expiration time.
// If no value is provided via WithLockValue, a default timestamp-based value is used.
func NewRedisLock(client *redis.Client, key string, ttl time.Duration, opts ...RedisLockOption) *RedisLock {
	l := &RedisLock{
		client: client,
		key:    key,
		ttl:    ttl,
		value:  generateDefaultValue(),
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// generateDefaultValue creates a default unique value for the lock.
// In production, this should include hostname/pod ID for better debugging.
func generateDefaultValue() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Acquire attempts to acquire the lock using Redis SET NX.
// Returns true if the lock was acquired, false if already held by another instance.
func (l *RedisLock) Acquire(ctx context.Context) (bool, error) {
	// Use SET NX PX for atomic check-and-set with expiration
	// SET key value NX PX milliseconds
	result, err := l.client.SetNX(ctx, l.key, l.value, l.ttl).Result()
	if err != nil {
		return false, err
	}

	if result {
		l.mu.Lock()
		l.held = true
		l.mu.Unlock()
	}

	return result, nil
}

// Release releases the lock if it's held by this instance.
// Uses a Lua script to ensure atomic check-and-delete.
func (l *RedisLock) Release(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.held {
		return nil // Already released, no-op
	}

	// Lua script for atomic check-and-delete
	// Only delete if the value matches (we own the lock)
	script := redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		end
		return 0
	`)

	result, err := script.Run(ctx, l.client, []string{l.key}, l.value).Int64()
	if err != nil {
		return err
	}

	if result == 1 {
		l.held = false
	}

	return nil
}

// Extend extends the lock's TTL if held by this instance.
// Uses a Lua script to ensure atomic check-and-extend.
func (l *RedisLock) Extend(ctx context.Context) error {
	l.mu.RLock()
	held := l.held
	l.mu.RUnlock()

	if !held {
		return ErrLockNotHeld
	}

	// Lua script for atomic check-and-extend
	// Only extend if the value matches (we own the lock)
	script := redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("PEXPIRE", KEYS[1], ARGV[2])
		end
		return 0
	`)

	result, err := script.Run(ctx, l.client, []string{l.key}, l.value, l.ttl.Milliseconds()).Int64()
	if err != nil {
		return err
	}

	if result == 0 {
		l.mu.Lock()
		l.held = false
		l.mu.Unlock()
		return ErrLockNotHeld
	}

	return nil
}

// IsHeld returns true if this instance currently holds the lock.
func (l *RedisLock) IsHeld() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.held
}

// Key returns the lock's Redis key.
func (l *RedisLock) Key() string {
	return l.key
}

// TTL returns the lock's TTL duration.
func (l *RedisLock) TTL() time.Duration {
	return l.ttl
}
