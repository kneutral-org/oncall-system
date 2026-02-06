// Package idempotency provides mechanisms to prevent duplicate webhook processing.
package idempotency

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrKeyExists is returned when an idempotency key already exists.
var ErrKeyExists = errors.New("idempotency key already exists")

// Store defines the interface for idempotency key storage.
type Store interface {
	// CheckAndSet atomically checks if a key exists and sets it if not.
	// Returns true if the key was set (new request), false if it already existed (duplicate).
	// The key will automatically expire after the specified TTL.
	CheckAndSet(ctx context.Context, key string, ttl time.Duration) (bool, error)

	// Delete removes an idempotency key, typically used on request failure
	// to allow retries.
	Delete(ctx context.Context, key string) error
}

// Entry represents an idempotency key entry with expiration.
type Entry struct {
	Key       string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// MemoryStore is an in-memory implementation of Store for testing and development.
type MemoryStore struct {
	mu      sync.RWMutex
	entries map[string]*Entry
	stopCh  chan struct{}
}

// NewMemoryStore creates a new in-memory idempotency store.
// It starts a background goroutine to clean up expired entries.
func NewMemoryStore() *MemoryStore {
	s := &MemoryStore{
		entries: make(map[string]*Entry),
		stopCh:  make(chan struct{}),
	}
	go s.cleanupLoop()
	return s
}

// CheckAndSet implements Store.CheckAndSet for in-memory storage.
func (s *MemoryStore) CheckAndSet(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// Check if key exists and is not expired
	if entry, exists := s.entries[key]; exists {
		if entry.ExpiresAt.After(now) {
			return false, nil // Key exists and is valid, this is a duplicate
		}
		// Key exists but expired, we can reuse it
	}

	// Set the key
	s.entries[key] = &Entry{
		Key:       key,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}

	return true, nil
}

// Delete implements Store.Delete for in-memory storage.
func (s *MemoryStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.entries, key)
	return nil
}

// Close stops the cleanup goroutine.
func (s *MemoryStore) Close() {
	close(s.stopCh)
}

// cleanupLoop periodically removes expired entries.
func (s *MemoryStore) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.cleanup()
		}
	}
}

// cleanup removes all expired entries.
func (s *MemoryStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for key, entry := range s.entries {
		if entry.ExpiresAt.Before(now) {
			delete(s.entries, key)
		}
	}
}

// Len returns the number of entries in the store (for testing).
func (s *MemoryStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}
