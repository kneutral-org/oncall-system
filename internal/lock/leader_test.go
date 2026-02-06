package lock

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// mockLock is a mock implementation of DistributedLock for testing.
type mockLock struct {
	acquireResult bool
	acquireErr    error
	releaseErr    error
	extendErr     error
	held          atomic.Bool
	acquireCalls  atomic.Int32
	releaseCalls  atomic.Int32
	extendCalls   atomic.Int32
}

func (m *mockLock) Acquire(ctx context.Context) (bool, error) {
	m.acquireCalls.Add(1)
	if m.acquireErr != nil {
		return false, m.acquireErr
	}
	if m.acquireResult {
		m.held.Store(true)
	}
	return m.acquireResult, nil
}

func (m *mockLock) Release(ctx context.Context) error {
	m.releaseCalls.Add(1)
	m.held.Store(false)
	return m.releaseErr
}

func (m *mockLock) Extend(ctx context.Context) error {
	m.extendCalls.Add(1)
	if m.extendErr != nil {
		return m.extendErr
	}
	return nil
}

func (m *mockLock) IsHeld() bool {
	return m.held.Load()
}

func TestLeaderElector_BecomeLeader(t *testing.T) {
	mock := &mockLock{acquireResult: true}
	logger := zerolog.Nop()

	var becameLeader atomic.Bool
	elector := NewLeaderElector(mock, logger,
		WithRenewalRate(50*time.Millisecond),
		WithOnBecomeLeader(func() {
			becameLeader.Store(true)
		}),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	elector.Start(ctx)

	// Wait for leader acquisition
	time.Sleep(100 * time.Millisecond)

	if !elector.IsLeader() {
		t.Error("Expected to be leader")
	}
	if !becameLeader.Load() {
		t.Error("Expected onBecomeLeader callback to be called")
	}

	elector.Stop(context.Background())

	if elector.IsLeader() {
		t.Error("Expected to not be leader after stop")
	}
}

func TestLeaderElector_LoseLeadership(t *testing.T) {
	mock := &mockLock{acquireResult: true}
	logger := zerolog.Nop()

	var lostLeader atomic.Bool
	elector := NewLeaderElector(mock, logger,
		WithRenewalRate(50*time.Millisecond),
		WithOnLoseLeader(func() {
			lostLeader.Store(true)
		}),
	)

	ctx, cancel := context.WithCancel(context.Background())

	elector.Start(ctx)

	// Wait for leader acquisition
	time.Sleep(100 * time.Millisecond)

	if !elector.IsLeader() {
		t.Error("Expected to be leader")
	}

	// Simulate losing leadership by making extend fail
	mock.extendErr = ErrLockNotHeld

	// Wait for renewal attempt
	time.Sleep(100 * time.Millisecond)

	if !lostLeader.Load() {
		t.Error("Expected onLoseLeader callback to be called")
	}

	cancel()
	elector.Stop(context.Background())
}

func TestLeaderElector_RetryAcquisition(t *testing.T) {
	mock := &mockLock{acquireResult: false}
	logger := zerolog.Nop()

	elector := NewLeaderElector(mock, logger,
		WithRenewalRate(50*time.Millisecond),
		WithRetryBackoff(10*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	elector.Start(ctx)

	// Wait for a few acquisition attempts
	time.Sleep(200 * time.Millisecond)

	// Should have tried multiple times
	calls := mock.acquireCalls.Load()
	if calls < 2 {
		t.Errorf("Expected multiple acquire attempts, got %d", calls)
	}

	// Should not be leader
	if elector.IsLeader() {
		t.Error("Expected to not be leader when acquisition fails")
	}

	elector.Stop(context.Background())
}

func TestLeaderElector_RenewalCalls(t *testing.T) {
	mock := &mockLock{acquireResult: true}
	logger := zerolog.Nop()

	elector := NewLeaderElector(mock, logger,
		WithRenewalRate(50*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	elector.Start(ctx)

	// Wait for acquisition and some renewals
	time.Sleep(200 * time.Millisecond)

	elector.Stop(context.Background())

	// Should have extended multiple times
	extendCalls := mock.extendCalls.Load()
	if extendCalls < 2 {
		t.Errorf("Expected multiple extend calls, got %d", extendCalls)
	}
}

func TestLeaderElector_StopReleasesLock(t *testing.T) {
	mock := &mockLock{acquireResult: true}
	logger := zerolog.Nop()

	elector := NewLeaderElector(mock, logger,
		WithRenewalRate(50*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	elector.Start(ctx)

	// Wait for leader acquisition
	time.Sleep(100 * time.Millisecond)

	elector.Stop(context.Background())

	// Should have called release
	if mock.releaseCalls.Load() == 0 {
		t.Error("Expected release to be called on stop")
	}
}

func TestLeaderElector_ContextCancellation(t *testing.T) {
	mock := &mockLock{acquireResult: true}
	logger := zerolog.Nop()

	elector := NewLeaderElector(mock, logger,
		WithRenewalRate(50*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())

	elector.Start(ctx)

	// Wait for leader acquisition
	time.Sleep(100 * time.Millisecond)

	// Cancel context
	cancel()

	// Give time for goroutine to exit
	time.Sleep(100 * time.Millisecond)

	// Stop should still work cleanly
	elector.Stop(context.Background())
}
