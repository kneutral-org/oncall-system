package lock

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// LeaderElector manages leader election using a distributed lock.
// It continuously tries to acquire and maintain leadership.
type LeaderElector struct {
	lock   DistributedLock
	logger zerolog.Logger

	isLeader     atomic.Bool
	renewalRate  time.Duration
	retryBackoff time.Duration

	onBecomeLeader func()
	onLoseLeader   func()

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// LeaderElectorOption configures a LeaderElector.
type LeaderElectorOption func(*LeaderElector)

// WithRenewalRate sets how often the leader renews its lock.
// Should be significantly less than the lock TTL (e.g., TTL/3).
func WithRenewalRate(d time.Duration) LeaderElectorOption {
	return func(e *LeaderElector) {
		e.renewalRate = d
	}
}

// WithRetryBackoff sets how long to wait before retrying to acquire leadership.
func WithRetryBackoff(d time.Duration) LeaderElectorOption {
	return func(e *LeaderElector) {
		e.retryBackoff = d
	}
}

// WithOnBecomeLeader sets a callback that's called when this instance becomes leader.
func WithOnBecomeLeader(fn func()) LeaderElectorOption {
	return func(e *LeaderElector) {
		e.onBecomeLeader = fn
	}
}

// WithOnLoseLeader sets a callback that's called when this instance loses leadership.
func WithOnLoseLeader(fn func()) LeaderElectorOption {
	return func(e *LeaderElector) {
		e.onLoseLeader = fn
	}
}

// NewLeaderElector creates a new leader elector with the given lock.
func NewLeaderElector(lock DistributedLock, logger zerolog.Logger, opts ...LeaderElectorOption) *LeaderElector {
	e := &LeaderElector{
		lock:         lock,
		logger:       logger,
		renewalRate:  20 * time.Second, // Default: renew every 20s for 60s TTL
		retryBackoff: 5 * time.Second,
		stopCh:       make(chan struct{}),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Start begins the leader election loop.
// It will continuously try to acquire and maintain leadership until Stop is called.
func (e *LeaderElector) Start(ctx context.Context) {
	e.wg.Add(1)
	go e.run(ctx)
}

// Stop stops the leader election loop and releases leadership if held.
func (e *LeaderElector) Stop(ctx context.Context) {
	close(e.stopCh)
	e.wg.Wait()

	// Release the lock if we're the leader
	if e.isLeader.Load() {
		if err := e.lock.Release(ctx); err != nil {
			e.logger.Error().Err(err).Msg("failed to release lock on shutdown")
		} else {
			e.logger.Info().Msg("released leadership on shutdown")
		}
		e.isLeader.Store(false)
		if e.onLoseLeader != nil {
			e.onLoseLeader()
		}
	}
}

// IsLeader returns true if this instance is currently the leader.
func (e *LeaderElector) IsLeader() bool {
	return e.isLeader.Load()
}

func (e *LeaderElector) run(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(e.renewalRate)
	defer ticker.Stop()

	// Try to acquire immediately on start
	e.tryAcquireOrRenew(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.tryAcquireOrRenew(ctx)
		}
	}
}

func (e *LeaderElector) tryAcquireOrRenew(ctx context.Context) {
	if e.isLeader.Load() {
		// We're the leader, try to extend
		if err := e.lock.Extend(ctx); err != nil {
			e.logger.Warn().Err(err).Msg("failed to extend leadership, lost leader status")
			e.isLeader.Store(false)
			if e.onLoseLeader != nil {
				e.onLoseLeader()
			}
			// Immediately try to reacquire
			e.tryAcquire(ctx)
		} else {
			e.logger.Debug().Msg("successfully renewed leadership")
		}
	} else {
		e.tryAcquire(ctx)
	}
}

func (e *LeaderElector) tryAcquire(ctx context.Context) {
	acquired, err := e.lock.Acquire(ctx)
	if err != nil {
		e.logger.Error().Err(err).Msg("failed to acquire leadership")
		return
	}

	if acquired {
		e.logger.Info().Msg("acquired leadership")
		e.isLeader.Store(true)
		if e.onBecomeLeader != nil {
			e.onBecomeLeader()
		}
	} else {
		e.logger.Debug().Msg("another instance is leader")
	}
}
