// Package store provides interfaces and implementations for data persistence.
package store

import (
	"context"

	alertingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/v1"
)

// AlertStore defines the interface for alert persistence operations.
type AlertStore interface {
	// Create creates a new alert and returns the created alert with generated ID.
	Create(ctx context.Context, alert *alertingv1.Alert) (*alertingv1.Alert, error)

	// GetByID retrieves an alert by its ID.
	GetByID(ctx context.Context, id string) (*alertingv1.Alert, error)

	// GetByFingerprint retrieves an alert by its fingerprint for deduplication.
	GetByFingerprint(ctx context.Context, fingerprint string) (*alertingv1.Alert, error)

	// Update updates an existing alert.
	Update(ctx context.Context, alert *alertingv1.Alert) (*alertingv1.Alert, error)

	// CreateOrUpdate creates a new alert or updates an existing one based on fingerprint.
	// Returns the alert and a boolean indicating if it was created (true) or updated (false).
	CreateOrUpdate(ctx context.Context, alert *alertingv1.Alert) (*alertingv1.Alert, bool, error)

	// List retrieves alerts based on filter criteria.
	List(ctx context.Context, req *alertingv1.ListAlertsRequest) (*alertingv1.ListAlertsResponse, error)
}
