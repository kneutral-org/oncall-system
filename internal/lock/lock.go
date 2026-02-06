// Package lock provides distributed locking mechanisms for coordinating
// work across multiple service instances.
package lock

import (
	"context"
)

// DistributedLock defines the interface for a distributed lock.
// Implementations must be safe for concurrent use.
type DistributedLock interface {
	// Acquire attempts to acquire the lock.
	// Returns true if the lock was successfully acquired, false if it's already held.
	// The lock will automatically expire after the configured TTL.
	Acquire(ctx context.Context) (bool, error)

	// Release releases the lock if it's held by this instance.
	// It's safe to call Release even if the lock is not held.
	Release(ctx context.Context) error

	// Extend extends the lock's TTL.
	// Returns an error if the lock is not held by this instance.
	Extend(ctx context.Context) error

	// IsHeld returns true if this instance currently holds the lock.
	IsHeld() bool
}
