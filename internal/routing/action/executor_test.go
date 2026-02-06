package action

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

func TestDefaultExecutor_Execute(t *testing.T) {
	logger := zerolog.Nop()
	metrics := NewMetrics()

	tests := []struct {
		name           string
		config         *ExecutorConfig
		actions        []*routingv1.RoutingAction
		setupHandler   func(e *DefaultExecutor)
		expectedLen    int
		expectedErrors int
		expectError    bool
	}{
		{
			name:   "empty actions list",
			config: DefaultExecutorConfig(),
			actions: []*routingv1.RoutingAction{},
			setupHandler: func(e *DefaultExecutor) {},
			expectedLen:    0,
			expectedErrors: 0,
			expectError:    false,
		},
		{
			name:   "single successful action",
			config: DefaultExecutorConfig(),
			actions: []*routingv1.RoutingAction{
				{Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM},
			},
			setupHandler: func(e *DefaultExecutor) {
				e.RegisterAction(routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM, func(ctx context.Context, alert *routingv1.Alert, action *routingv1.RoutingAction) (*Result, error) {
					return &Result{
						ActionType: "ACTION_TYPE_NOTIFY_TEAM",
						Success:    true,
						Message:    "success",
						Duration:   time.Millisecond,
					}, nil
				})
			},
			expectedLen:    1,
			expectedErrors: 0,
			expectError:    false,
		},
		{
			name:   "unregistered action type",
			config: DefaultExecutorConfig(),
			actions: []*routingv1.RoutingAction{
				{Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM},
			},
			setupHandler:   func(e *DefaultExecutor) {},
			expectedLen:    1,
			expectedErrors: 1,
			expectError:    true,
		},
		{
			name: "continue on error enabled",
			config: &ExecutorConfig{
				MaxRetries:      0,
				ContinueOnError: true,
				Timeout:         time.Second,
			},
			actions: []*routingv1.RoutingAction{
				{Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM},
				{Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_USER},
			},
			setupHandler: func(e *DefaultExecutor) {
				e.RegisterAction(routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM, func(ctx context.Context, alert *routingv1.Alert, action *routingv1.RoutingAction) (*Result, error) {
					return &Result{
						ActionType: "ACTION_TYPE_NOTIFY_TEAM",
						Success:    false,
						Error:      errors.New("team not found"),
						Retryable:  false,
						Duration:   time.Millisecond,
					}, errors.New("team not found")
				})
				e.RegisterAction(routingv1.ActionType_ACTION_TYPE_NOTIFY_USER, func(ctx context.Context, alert *routingv1.Alert, action *routingv1.RoutingAction) (*Result, error) {
					return &Result{
						ActionType: "ACTION_TYPE_NOTIFY_USER",
						Success:    true,
						Message:    "success",
						Duration:   time.Millisecond,
					}, nil
				})
			},
			expectedLen:    2,
			expectedErrors: 1,
			expectError:    true,
		},
		{
			name: "stop on error when disabled",
			config: &ExecutorConfig{
				MaxRetries:      0,
				ContinueOnError: false,
				Timeout:         time.Second,
			},
			actions: []*routingv1.RoutingAction{
				{Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM},
				{Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_USER},
			},
			setupHandler: func(e *DefaultExecutor) {
				e.RegisterAction(routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM, func(ctx context.Context, alert *routingv1.Alert, action *routingv1.RoutingAction) (*Result, error) {
					return &Result{
						ActionType: "ACTION_TYPE_NOTIFY_TEAM",
						Success:    false,
						Error:      errors.New("team not found"),
						Retryable:  false,
						Duration:   time.Millisecond,
					}, errors.New("team not found")
				})
				e.RegisterAction(routingv1.ActionType_ACTION_TYPE_NOTIFY_USER, func(ctx context.Context, alert *routingv1.Alert, action *routingv1.RoutingAction) (*Result, error) {
					return &Result{
						ActionType: "ACTION_TYPE_NOTIFY_USER",
						Success:    true,
						Message:    "success",
						Duration:   time.Millisecond,
					}, nil
				})
			},
			expectedLen:    1, // Only first action executed
			expectedErrors: 1,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewDefaultExecutor(tt.config, logger, metrics)
			tt.setupHandler(executor)

			alert := &routingv1.Alert{Id: "test-alert"}
			results, err := executor.Execute(context.Background(), alert, tt.actions)

			if (err != nil) != tt.expectError {
				t.Errorf("Execute() error = %v, expectError %v", err, tt.expectError)
			}

			if len(results) != tt.expectedLen {
				t.Errorf("Execute() returned %d results, expected %d", len(results), tt.expectedLen)
			}

			errorCount := 0
			for _, r := range results {
				if !r.Success {
					errorCount++
				}
			}
			if errorCount != tt.expectedErrors {
				t.Errorf("Execute() had %d errors, expected %d", errorCount, tt.expectedErrors)
			}
		})
	}
}

func TestDefaultExecutor_Retry(t *testing.T) {
	logger := zerolog.Nop()
	metrics := NewMetrics()

	attempts := 0
	config := &ExecutorConfig{
		MaxRetries:      2,
		RetryDelay:      time.Millisecond,
		ContinueOnError: true,
		Timeout:         time.Second,
	}

	executor := NewDefaultExecutor(config, logger, metrics)
	executor.RegisterAction(routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM, func(ctx context.Context, alert *routingv1.Alert, action *routingv1.RoutingAction) (*Result, error) {
		attempts++
		if attempts < 3 {
			return &Result{
				ActionType: "ACTION_TYPE_NOTIFY_TEAM",
				Success:    false,
				Error:      errors.New("transient error"),
				Retryable:  true,
				Duration:   time.Millisecond,
			}, errors.New("transient error")
		}
		return &Result{
			ActionType: "ACTION_TYPE_NOTIFY_TEAM",
			Success:    true,
			Message:    "success after retry",
			Duration:   time.Millisecond,
		}, nil
	})

	alert := &routingv1.Alert{Id: "test-alert"}
	actions := []*routingv1.RoutingAction{
		{Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM},
	}

	results, err := executor.Execute(context.Background(), alert, actions)

	if err != nil {
		t.Errorf("Execute() error = %v, expected nil after retry success", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}

	if len(results) != 1 || !results[0].Success {
		t.Errorf("Expected 1 successful result, got %d results with success=%v", len(results), results[0].Success)
	}
}

func TestDefaultExecutor_NilAlert(t *testing.T) {
	logger := zerolog.Nop()
	metrics := NewMetrics()
	executor := NewDefaultExecutor(nil, logger, metrics)

	_, err := executor.Execute(context.Background(), nil, []*routingv1.RoutingAction{})

	if err == nil {
		t.Error("Execute() expected error for nil alert, got nil")
	}

	if !errors.Is(err, ErrInvalidAction) {
		t.Errorf("Execute() error = %v, expected ErrInvalidAction", err)
	}
}

func TestDefaultExecutor_ContextCancellation(t *testing.T) {
	logger := zerolog.Nop()
	metrics := NewMetrics()

	config := &ExecutorConfig{
		MaxRetries:      5,
		RetryDelay:      100 * time.Millisecond,
		ContinueOnError: true,
		Timeout:         time.Second,
	}

	executor := NewDefaultExecutor(config, logger, metrics)
	executor.RegisterAction(routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM, func(ctx context.Context, alert *routingv1.Alert, action *routingv1.RoutingAction) (*Result, error) {
		return &Result{
			ActionType: "ACTION_TYPE_NOTIFY_TEAM",
			Success:    false,
			Error:      errors.New("keep failing"),
			Retryable:  true,
			Duration:   time.Millisecond,
		}, errors.New("keep failing")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	alert := &routingv1.Alert{Id: "test-alert"}
	actions := []*routingv1.RoutingAction{
		{Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM},
	}

	results, _ := executor.Execute(ctx, alert, actions)

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	// The result should indicate failure (either from context cancellation or the action error)
	if results[0].Success {
		t.Error("Expected result to indicate failure due to context cancellation or action error")
	}
}

func TestDefaultExecutor_GetRegisteredActions(t *testing.T) {
	logger := zerolog.Nop()
	metrics := NewMetrics()
	executor := NewDefaultExecutor(nil, logger, metrics)

	// Register some handlers
	executor.RegisterAction(routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM, func(ctx context.Context, alert *routingv1.Alert, action *routingv1.RoutingAction) (*Result, error) {
		return nil, nil
	})
	executor.RegisterAction(routingv1.ActionType_ACTION_TYPE_SUPPRESS, func(ctx context.Context, alert *routingv1.Alert, action *routingv1.RoutingAction) (*Result, error) {
		return nil, nil
	})

	registered := executor.GetRegisteredActions()

	if len(registered) != 2 {
		t.Errorf("Expected 2 registered actions, got %d", len(registered))
	}
}
