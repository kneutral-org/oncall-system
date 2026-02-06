package action

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// DefaultExecutor implements the Executor interface with support for
// retries, timeouts, and configurable error handling.
type DefaultExecutor struct {
	config   *ExecutorConfig
	handlers map[routingv1.ActionType]ActionHandler
	mu       sync.RWMutex
	logger   zerolog.Logger
	metrics  *Metrics
}

// NewDefaultExecutor creates a new DefaultExecutor with the provided configuration.
func NewDefaultExecutor(config *ExecutorConfig, logger zerolog.Logger, metrics *Metrics) *DefaultExecutor {
	if config == nil {
		config = DefaultExecutorConfig()
	}

	executor := &DefaultExecutor{
		config:   config,
		handlers: make(map[routingv1.ActionType]ActionHandler),
		logger:   logger.With().Str("component", "action_executor").Logger(),
		metrics:  metrics,
	}

	return executor
}

// RegisterAction registers a handler for a specific action type.
func (e *DefaultExecutor) RegisterAction(actionType routingv1.ActionType, handler ActionHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.handlers[actionType] = handler
	e.logger.Debug().Str("action_type", actionType.String()).Msg("registered action handler")
}

// Execute runs all provided actions for an alert in order.
// It continues on non-fatal errors if configured to do so and logs all results.
func (e *DefaultExecutor) Execute(ctx context.Context, alert *routingv1.Alert, actions []*routingv1.RoutingAction) ([]*Result, error) {
	if alert == nil {
		return nil, fmt.Errorf("%w: alert is nil", ErrInvalidAction)
	}

	results := make([]*Result, 0, len(actions))
	var lastError error

	for i, action := range actions {
		result := e.executeAction(ctx, alert, action, i)
		results = append(results, result)

		// Record metrics
		if e.metrics != nil {
			status := "success"
			if !result.Success {
				status = "failure"
			}
			e.metrics.RecordActionExecution(result.ActionType, status, result.Duration)
		}

		if !result.Success {
			lastError = result.Error
			if !e.config.ContinueOnError {
				e.logger.Warn().
					Str("alert_id", alert.Id).
					Str("action_type", result.ActionType).
					Err(result.Error).
					Msg("action execution failed, stopping")
				break
			}
			e.logger.Warn().
				Str("alert_id", alert.Id).
				Str("action_type", result.ActionType).
				Err(result.Error).
				Msg("action execution failed, continuing")
		}
	}

	return results, lastError
}

// executeAction executes a single action with retry support.
func (e *DefaultExecutor) executeAction(ctx context.Context, alert *routingv1.Alert, action *routingv1.RoutingAction, index int) *Result {
	actionType := action.GetType()
	actionTypeStr := actionType.String()
	startTime := time.Now()

	e.mu.RLock()
	handler, exists := e.handlers[actionType]
	e.mu.RUnlock()

	if !exists {
		return &Result{
			ActionType: actionTypeStr,
			Success:    false,
			Message:    fmt.Sprintf("no handler registered for action type %s", actionTypeStr),
			Error:      fmt.Errorf("%w: %s", ErrActionNotRegistered, actionTypeStr),
			Retryable:  false,
			Duration:   time.Since(startTime),
		}
	}

	// Execute with retries
	var result *Result
	var lastErr error

	for attempt := 0; attempt <= e.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry with exponential backoff
			delay := e.config.RetryDelay * time.Duration(1<<uint(attempt-1))
			select {
			case <-ctx.Done():
				return &Result{
					ActionType: actionTypeStr,
					Success:    false,
					Message:    "context cancelled during retry",
					Error:      ctx.Err(),
					Retryable:  false,
					Duration:   time.Since(startTime),
				}
			case <-time.After(delay):
			}

			e.logger.Debug().
				Str("alert_id", alert.Id).
				Str("action_type", actionTypeStr).
				Int("attempt", attempt+1).
				Msg("retrying action")
		}

		// Create a timeout context for this execution
		execCtx, cancel := context.WithTimeout(ctx, e.config.Timeout)
		result, lastErr = handler(execCtx, alert, action)
		cancel()

		if lastErr == nil && result != nil && result.Success {
			e.logger.Debug().
				Str("alert_id", alert.Id).
				Str("action_type", actionTypeStr).
				Int("action_index", index).
				Dur("duration", result.Duration).
				Msg("action executed successfully")
			return result
		}

		// Check if the error is retryable
		if result != nil && !result.Retryable {
			break
		}
	}

	// Return the last result or create one from the error
	if result == nil {
		result = &Result{
			ActionType: actionTypeStr,
			Success:    false,
			Message:    "action execution failed",
			Error:      lastErr,
			Retryable:  false,
			Duration:   time.Since(startTime),
		}
	}

	return result
}

// GetRegisteredActions returns the list of registered action types.
func (e *DefaultExecutor) GetRegisteredActions() []routingv1.ActionType {
	e.mu.RLock()
	defer e.mu.RUnlock()

	types := make([]routingv1.ActionType, 0, len(e.handlers))
	for t := range e.handlers {
		types = append(types, t)
	}
	return types
}

// Ensure DefaultExecutor satisfies the Executor interface
var _ Executor = (*DefaultExecutor)(nil)
