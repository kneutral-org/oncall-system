// Package idempotency provides mechanisms to prevent duplicate webhook processing.
package idempotency

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore is a Redis implementation of Store.
type RedisStore struct {
	client *redis.Client
	prefix string
}

// RedisOption configures a RedisStore.
type RedisOption func(*RedisStore)

// WithKeyPrefix sets a prefix for all idempotency keys in Redis.
func WithKeyPrefix(prefix string) RedisOption {
	return func(s *RedisStore) {
		s.prefix = prefix
	}
}

// NewRedisStore creates a new Redis-backed idempotency store.
func NewRedisStore(client *redis.Client, opts ...RedisOption) *RedisStore {
	s := &RedisStore{
		client: client,
		prefix: "idempotency:",
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// CheckAndSet implements Store.CheckAndSet for Redis storage.
// Uses SET NX (set if not exists) with expiration for atomic check-and-set.
func (s *RedisStore) CheckAndSet(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	fullKey := s.prefix + key

	// SET NX returns true if the key was set (didn't exist), false if it already existed
	wasSet, err := s.client.SetNX(ctx, fullKey, "1", ttl).Result()
	if err != nil {
		return false, err
	}

	return wasSet, nil
}

// Delete implements Store.Delete for Redis storage.
func (s *RedisStore) Delete(ctx context.Context, key string) error {
	fullKey := s.prefix + key
	return s.client.Del(ctx, fullKey).Err()
}

// Ping checks if the Redis connection is healthy.
func (s *RedisStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}
