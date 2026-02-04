// Package store provides interfaces and implementations for data persistence.
package store

import (
	"context"
)

// Service represents a service/integration that can send alerts.
type Service struct {
	ID             string
	Name           string
	IntegrationKey string
	Description    string
}

// ServiceStore defines the interface for service/integration persistence operations.
type ServiceStore interface {
	// GetByIntegrationKey retrieves a service by its integration key.
	// Returns the service and its ID if found, or an error if not found.
	GetByIntegrationKey(ctx context.Context, integrationKey string) (*Service, error)

	// Create creates a new service.
	Create(ctx context.Context, service *Service) (*Service, error)

	// GetByID retrieves a service by its ID.
	GetByID(ctx context.Context, id string) (*Service, error)
}
