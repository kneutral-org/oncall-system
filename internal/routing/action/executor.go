// Package action provides the action executor framework for the alerting system.
// It handles execution of routing actions when alerts match routing rules.
package action

import (
	"context"
	"errors"
	"time"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

var (
	// ErrActionNotRegistered is returned when an action type has no registered handler.
	ErrActionNotRegistered = errors.New("action type not registered")
	// ErrInvalidAction is returned when an action is invalid or missing required params.
	ErrInvalidAction = errors.New("invalid action configuration")
	// ErrActionFailed is returned when an action execution fails.
	ErrActionFailed = errors.New("action execution failed")
)

// Result represents the outcome of executing a single action.
type Result struct {
	// ActionType identifies the type of action that was executed.
	ActionType string `json:"actionType"`
	// Success indicates whether the action completed successfully.
	Success bool `json:"success"`
	// Message provides additional context about the execution result.
	Message string `json:"message"`
	// Error contains the error if the action failed.
	Error error `json:"error,omitempty"`
	// Retryable indicates whether this action can be retried on failure.
	Retryable bool `json:"retryable"`
	// Duration is the time taken to execute the action.
	Duration time.Duration `json:"duration"`
}

// Action defines the interface for executable routing actions.
type Action interface {
	// Type returns the action type identifier.
	Type() string
	// Execute performs the action for the given alert.
	Execute(ctx context.Context, alert *routingv1.Alert) (*Result, error)
}

// ActionHandler is a function type for handling specific action types.
type ActionHandler func(ctx context.Context, alert *routingv1.Alert, action *routingv1.RoutingAction) (*Result, error)

// Executor defines the interface for action execution.
type Executor interface {
	// Execute runs all provided actions for an alert.
	Execute(ctx context.Context, alert *routingv1.Alert, actions []*routingv1.RoutingAction) ([]*Result, error)
	// RegisterAction registers a handler for a specific action type.
	RegisterAction(actionType routingv1.ActionType, handler ActionHandler)
}

// ExecutorConfig holds configuration for the action executor.
type ExecutorConfig struct {
	// MaxRetries is the maximum number of retries for retryable actions.
	MaxRetries int
	// RetryDelay is the base delay between retries.
	RetryDelay time.Duration
	// ContinueOnError determines whether to continue executing actions after an error.
	ContinueOnError bool
	// Timeout is the maximum time allowed for a single action execution.
	Timeout time.Duration
}

// DefaultExecutorConfig returns the default executor configuration.
func DefaultExecutorConfig() *ExecutorConfig {
	return &ExecutorConfig{
		MaxRetries:      3,
		RetryDelay:      time.Second,
		ContinueOnError: true,
		Timeout:         30 * time.Second,
	}
}
