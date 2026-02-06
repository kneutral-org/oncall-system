// Package maintenance provides maintenance window management for the alerting system.
package maintenance

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// Match represents the result of checking an alert against maintenance windows.
type Match struct {
	// Window is the matching maintenance window.
	Window *routingv1.MaintenanceWindow

	// Action is the recommended action (suppress, annotate, reduce_severity).
	Action string

	// Reason explains why the alert matched.
	Reason string

	// MatchType indicates how the alert matched (site, service, label, global).
	MatchType MatchType
}

// Checker provides functionality to check if alerts match active maintenance windows.
type Checker interface {
	// Check checks if an alert matches any active maintenance window.
	Check(ctx context.Context, alert *routingv1.Alert) (*Match, error)

	// CheckAll checks an alert against all active windows and returns all matches.
	CheckAll(ctx context.Context, alert *routingv1.Alert) ([]*Match, error)

	// ListActive lists currently active maintenance windows.
	ListActive(ctx context.Context) ([]*routingv1.MaintenanceWindow, error)

	// ListUpcoming lists maintenance windows starting within the given duration.
	ListUpcoming(ctx context.Context, duration time.Duration) ([]*routingv1.MaintenanceWindow, error)

	// RefreshStatuses transitions maintenance window statuses based on current time.
	RefreshStatuses(ctx context.Context) error
}

// DefaultChecker implements the Checker interface.
type DefaultChecker struct {
	store   Store
	matcher *Matcher
	logger  zerolog.Logger
}

// NewChecker creates a new DefaultChecker.
func NewChecker(store Store, logger zerolog.Logger) *DefaultChecker {
	return &DefaultChecker{
		store:   store,
		matcher: NewMatcher(),
		logger:  logger.With().Str("component", "maintenance_checker").Logger(),
	}
}

// Check checks if an alert matches any active maintenance window.
// Returns the first matching window, or nil if no window matches.
func (c *DefaultChecker) Check(ctx context.Context, alert *routingv1.Alert) (*Match, error) {
	if alert == nil {
		return nil, fmt.Errorf("alert is required")
	}

	c.logger.Debug().
		Str("alertId", alert.Id).
		Str("fingerprint", alert.Fingerprint).
		Msg("checking alert against maintenance windows")

	// Get all active maintenance windows
	windows, err := c.store.ListActive(ctx, nil, nil)
	if err != nil {
		c.logger.Error().Err(err).Msg("failed to list active maintenance windows")
		return nil, fmt.Errorf("list active windows: %w", err)
	}

	if len(windows) == 0 {
		c.logger.Debug().Msg("no active maintenance windows")
		return nil, nil
	}

	// Check each window
	for _, window := range windows {
		result := c.matcher.Match(alert, window)
		if result.Matched {
			match := &Match{
				Window:    window,
				Action:    actionToReadableString(window.Action),
				Reason:    result.Reason,
				MatchType: result.MatchType,
			}

			c.logger.Info().
				Str("alertId", alert.Id).
				Str("windowId", window.Id).
				Str("windowName", window.Name).
				Str("matchType", string(result.MatchType)).
				Str("action", match.Action).
				Msg("alert matches maintenance window")

			return match, nil
		}
	}

	c.logger.Debug().
		Str("alertId", alert.Id).
		Int("windowsChecked", len(windows)).
		Msg("alert does not match any maintenance window")

	return nil, nil
}

// CheckAll checks an alert against all active windows and returns all matches.
func (c *DefaultChecker) CheckAll(ctx context.Context, alert *routingv1.Alert) ([]*Match, error) {
	if alert == nil {
		return nil, fmt.Errorf("alert is required")
	}

	// Get all active maintenance windows
	windows, err := c.store.ListActive(ctx, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("list active windows: %w", err)
	}

	var matches []*Match

	for _, window := range windows {
		result := c.matcher.Match(alert, window)
		if result.Matched {
			matches = append(matches, &Match{
				Window:    window,
				Action:    actionToReadableString(window.Action),
				Reason:    result.Reason,
				MatchType: result.MatchType,
			})
		}
	}

	return matches, nil
}

// ListActive lists currently active maintenance windows.
func (c *DefaultChecker) ListActive(ctx context.Context) ([]*routingv1.MaintenanceWindow, error) {
	return c.store.ListActive(ctx, nil, nil)
}

// ListUpcoming lists maintenance windows starting within the given duration.
func (c *DefaultChecker) ListUpcoming(ctx context.Context, duration time.Duration) ([]*routingv1.MaintenanceWindow, error) {
	return c.store.ListUpcoming(ctx, duration)
}

// RefreshStatuses transitions maintenance window statuses based on current time.
func (c *DefaultChecker) RefreshStatuses(ctx context.Context) error {
	c.logger.Debug().Msg("refreshing maintenance window statuses")

	if err := c.store.TransitionStatuses(ctx); err != nil {
		c.logger.Error().Err(err).Msg("failed to transition maintenance window statuses")
		return err
	}

	c.logger.Debug().Msg("maintenance window statuses refreshed")
	return nil
}

// CheckResult represents the result of checking an alert, used for gRPC responses.
type CheckResult struct {
	InMaintenance     bool
	MatchingWindows   []*routingv1.MaintenanceWindow
	RecommendedAction routingv1.MaintenanceAction
}

// CheckForGRPC checks an alert and returns a result suitable for gRPC responses.
func (c *DefaultChecker) CheckForGRPC(ctx context.Context, alert *routingv1.Alert) (*CheckResult, error) {
	matches, err := c.CheckAll(ctx, alert)
	if err != nil {
		return nil, err
	}

	result := &CheckResult{
		InMaintenance:     len(matches) > 0,
		MatchingWindows:   make([]*routingv1.MaintenanceWindow, 0, len(matches)),
		RecommendedAction: routingv1.MaintenanceAction_MAINTENANCE_ACTION_UNSPECIFIED,
	}

	for _, match := range matches {
		result.MatchingWindows = append(result.MatchingWindows, match.Window)
	}

	// Determine recommended action based on priority:
	// suppress > annotate > reduce_severity
	if len(matches) > 0 {
		result.RecommendedAction = routingv1.MaintenanceAction_MAINTENANCE_ACTION_ANNOTATE

		for _, match := range matches {
			if match.Window.Action == routingv1.MaintenanceAction_MAINTENANCE_ACTION_SUPPRESS {
				result.RecommendedAction = routingv1.MaintenanceAction_MAINTENANCE_ACTION_SUPPRESS
				break
			}
		}
	}

	return result, nil
}

// Helper functions

func actionToReadableString(action routingv1.MaintenanceAction) string {
	switch action {
	case routingv1.MaintenanceAction_MAINTENANCE_ACTION_SUPPRESS:
		return "suppress"
	case routingv1.MaintenanceAction_MAINTENANCE_ACTION_ANNOTATE:
		return "annotate"
	case routingv1.MaintenanceAction_MAINTENANCE_ACTION_REDUCE_SEVERITY:
		return "reduce_severity"
	default:
		return "unknown"
	}
}

// Ensure DefaultChecker implements Checker
var _ Checker = (*DefaultChecker)(nil)
