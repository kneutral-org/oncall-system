// Package idempotency provides mechanisms to prevent duplicate webhook processing.
package idempotency

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// Cleaner defines the interface for stores that support cleanup operations.
type Cleaner interface {
	// Cleanup removes expired entries and returns the number of entries removed.
	Cleanup(ctx context.Context) (int64, error)
}

// CleanupJob periodically cleans up expired idempotency keys.
type CleanupJob struct {
	store    Cleaner
	interval time.Duration
	logger   zerolog.Logger
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// NewCleanupJob creates a new cleanup job that runs at the specified interval.
func NewCleanupJob(store Cleaner, interval time.Duration, logger zerolog.Logger) *CleanupJob {
	return &CleanupJob{
		store:    store,
		interval: interval,
		logger:   logger.With().Str("component", "idempotency-cleanup").Logger(),
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// Start begins the cleanup job in a background goroutine.
func (j *CleanupJob) Start() {
	go j.run()
}

// Stop signals the cleanup job to stop and waits for it to finish.
func (j *CleanupJob) Stop() {
	close(j.stopCh)
	<-j.doneCh
}

func (j *CleanupJob) run() {
	defer close(j.doneCh)

	// Run an initial cleanup
	j.runCleanup()

	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-j.stopCh:
			j.logger.Info().Msg("cleanup job stopped")
			return
		case <-ticker.C:
			j.runCleanup()
		}
	}
}

func (j *CleanupJob) runCleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	count, err := j.store.Cleanup(ctx)
	if err != nil {
		j.logger.Error().Err(err).Msg("failed to cleanup expired idempotency keys")
		return
	}

	if count > 0 {
		j.logger.Info().
			Int64("removedCount", count).
			Msg("cleaned up expired idempotency keys")
	}
}
