package idempotency

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// mockCleaner is a mock implementation of Cleaner for testing.
type mockCleaner struct {
	cleanupCount atomic.Int64
	removedCount int64
	err          error
}

func (m *mockCleaner) Cleanup(ctx context.Context) (int64, error) {
	m.cleanupCount.Add(1)
	return m.removedCount, m.err
}

func TestCleanupJob_RunsAtInterval(t *testing.T) {
	cleaner := &mockCleaner{removedCount: 5}

	job := NewCleanupJob(cleaner, 50*time.Millisecond, zerolog.Nop())
	job.Start()

	// Wait for initial cleanup + at least one interval
	time.Sleep(120 * time.Millisecond)

	job.Stop()

	// Should have run at least twice (initial + interval)
	count := cleaner.cleanupCount.Load()
	if count < 2 {
		t.Errorf("expected at least 2 cleanups, got %d", count)
	}
}

func TestCleanupJob_Stop(t *testing.T) {
	cleaner := &mockCleaner{removedCount: 0}

	job := NewCleanupJob(cleaner, time.Hour, zerolog.Nop())
	job.Start()

	// Should complete quickly
	done := make(chan struct{})
	go func() {
		job.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Error("Stop did not return in time")
	}
}

func TestCleanupJob_ContinuesOnError(t *testing.T) {
	cleaner := &mockCleaner{
		removedCount: 0,
		err:          context.DeadlineExceeded,
	}

	job := NewCleanupJob(cleaner, 30*time.Millisecond, zerolog.Nop())
	job.Start()

	// Wait for multiple cleanup attempts
	time.Sleep(100 * time.Millisecond)

	job.Stop()

	// Should have attempted multiple cleanups despite errors
	count := cleaner.cleanupCount.Load()
	if count < 2 {
		t.Errorf("expected at least 2 cleanup attempts, got %d", count)
	}
}
