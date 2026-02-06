// Package grpc provides gRPC service implementations.
package grpc

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/kneutral-org/alerting-system/internal/maintenance"
	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// MaintenanceService implements the MaintenanceServiceServer interface.
type MaintenanceService struct {
	routingv1.UnimplementedMaintenanceServiceServer
	store   maintenance.Store
	checker *maintenance.DefaultChecker
	logger  zerolog.Logger
}

// NewMaintenanceService creates a new MaintenanceService.
func NewMaintenanceService(store maintenance.Store, logger zerolog.Logger) *MaintenanceService {
	return &MaintenanceService{
		store:   store,
		checker: maintenance.NewChecker(store, logger),
		logger:  logger.With().Str("service", "maintenance").Logger(),
	}
}

// CreateMaintenanceWindow creates a new maintenance window.
func (s *MaintenanceService) CreateMaintenanceWindow(ctx context.Context, req *routingv1.CreateMaintenanceWindowRequest) (*routingv1.MaintenanceWindow, error) {
	if req.Window == nil {
		return nil, status.Error(codes.InvalidArgument, "window is required")
	}

	if req.Window.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "window name is required")
	}

	if req.Window.StartTime == nil {
		return nil, status.Error(codes.InvalidArgument, "start_time is required")
	}

	if req.Window.EndTime == nil {
		return nil, status.Error(codes.InvalidArgument, "end_time is required")
	}

	s.logger.Info().
		Str("name", req.Window.Name).
		Time("startTime", req.Window.StartTime.AsTime()).
		Time("endTime", req.Window.EndTime.AsTime()).
		Msg("creating maintenance window")

	window, err := s.store.Create(ctx, req.Window)
	if err != nil {
		if errors.Is(err, maintenance.ErrInvalidWindow) {
			return nil, status.Errorf(codes.InvalidArgument, "invalid window: %v", err)
		}
		s.logger.Error().Err(err).Msg("failed to create maintenance window")
		return nil, status.Error(codes.Internal, "failed to create maintenance window")
	}

	s.logger.Info().
		Str("id", window.Id).
		Str("name", window.Name).
		Msg("maintenance window created")

	return window, nil
}

// GetMaintenanceWindow retrieves a maintenance window by ID.
func (s *MaintenanceService) GetMaintenanceWindow(ctx context.Context, req *routingv1.GetMaintenanceWindowRequest) (*routingv1.MaintenanceWindow, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	window, err := s.store.Get(ctx, req.Id)
	if err != nil {
		if errors.Is(err, maintenance.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "maintenance window not found")
		}
		s.logger.Error().Err(err).Str("id", req.Id).Msg("failed to get maintenance window")
		return nil, status.Error(codes.Internal, "failed to get maintenance window")
	}

	return window, nil
}

// ListMaintenanceWindows retrieves maintenance windows with optional filters.
func (s *MaintenanceService) ListMaintenanceWindows(ctx context.Context, req *routingv1.ListMaintenanceWindowsRequest) (*routingv1.ListMaintenanceWindowsResponse, error) {
	resp, err := s.store.List(ctx, req)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to list maintenance windows")
		return nil, status.Error(codes.Internal, "failed to list maintenance windows")
	}

	return resp, nil
}

// UpdateMaintenanceWindow updates an existing maintenance window.
func (s *MaintenanceService) UpdateMaintenanceWindow(ctx context.Context, req *routingv1.UpdateMaintenanceWindowRequest) (*routingv1.MaintenanceWindow, error) {
	if req.Window == nil || req.Window.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "window with id is required")
	}

	s.logger.Info().
		Str("id", req.Window.Id).
		Str("name", req.Window.Name).
		Msg("updating maintenance window")

	window, err := s.store.Update(ctx, req.Window)
	if err != nil {
		if errors.Is(err, maintenance.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "maintenance window not found")
		}
		if errors.Is(err, maintenance.ErrInvalidWindow) {
			return nil, status.Errorf(codes.InvalidArgument, "invalid window: %v", err)
		}
		s.logger.Error().Err(err).Str("id", req.Window.Id).Msg("failed to update maintenance window")
		return nil, status.Error(codes.Internal, "failed to update maintenance window")
	}

	s.logger.Info().
		Str("id", window.Id).
		Msg("maintenance window updated")

	return window, nil
}

// DeleteMaintenanceWindow deletes a maintenance window by ID.
func (s *MaintenanceService) DeleteMaintenanceWindow(ctx context.Context, req *routingv1.DeleteMaintenanceWindowRequest) (*routingv1.DeleteMaintenanceWindowResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	s.logger.Info().Str("id", req.Id).Msg("deleting maintenance window")

	err := s.store.Delete(ctx, req.Id)
	if err != nil {
		if errors.Is(err, maintenance.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "maintenance window not found")
		}
		s.logger.Error().Err(err).Str("id", req.Id).Msg("failed to delete maintenance window")
		return nil, status.Error(codes.Internal, "failed to delete maintenance window")
	}

	s.logger.Info().Str("id", req.Id).Msg("maintenance window deleted")

	return &routingv1.DeleteMaintenanceWindowResponse{Success: true}, nil
}

// ListActiveMaintenanceWindows retrieves currently active maintenance windows.
func (s *MaintenanceService) ListActiveMaintenanceWindows(ctx context.Context, req *routingv1.ListActiveMaintenanceWindowsRequest) (*routingv1.ListMaintenanceWindowsResponse, error) {
	// First, refresh statuses to ensure windows are in correct state
	if err := s.checker.RefreshStatuses(ctx); err != nil {
		s.logger.Warn().Err(err).Msg("failed to refresh maintenance window statuses")
		// Continue anyway, statuses might be slightly stale
	}

	windows, err := s.store.ListActive(ctx, req.SiteIds, req.ServiceIds)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to list active maintenance windows")
		return nil, status.Error(codes.Internal, "failed to list active maintenance windows")
	}

	return &routingv1.ListMaintenanceWindowsResponse{
		Windows:    windows,
		TotalCount: int32(len(windows)),
	}, nil
}

// CheckAlertMaintenance checks if an alert is in maintenance.
func (s *MaintenanceService) CheckAlertMaintenance(ctx context.Context, req *routingv1.CheckAlertMaintenanceRequest) (*routingv1.CheckAlertMaintenanceResponse, error) {
	if req.Alert == nil {
		return nil, status.Error(codes.InvalidArgument, "alert is required")
	}

	s.logger.Debug().
		Str("alertId", req.Alert.Id).
		Str("fingerprint", req.Alert.Fingerprint).
		Msg("checking alert maintenance")

	// First, refresh statuses
	if err := s.checker.RefreshStatuses(ctx); err != nil {
		s.logger.Warn().Err(err).Msg("failed to refresh maintenance window statuses")
	}

	result, err := s.checker.CheckForGRPC(ctx, req.Alert)
	if err != nil {
		s.logger.Error().Err(err).Str("alertId", req.Alert.Id).Msg("failed to check alert maintenance")
		return nil, status.Error(codes.Internal, "failed to check alert maintenance")
	}

	s.logger.Debug().
		Str("alertId", req.Alert.Id).
		Bool("inMaintenance", result.InMaintenance).
		Int("matchingWindows", len(result.MatchingWindows)).
		Msg("alert maintenance check complete")

	return &routingv1.CheckAlertMaintenanceResponse{
		InMaintenance:     result.InMaintenance,
		MatchingWindows:   result.MatchingWindows,
		RecommendedAction: result.RecommendedAction,
	}, nil
}

// CancelMaintenanceWindow cancels an active or scheduled maintenance window.
func (s *MaintenanceService) CancelMaintenanceWindow(ctx context.Context, id string) error {
	s.logger.Info().Str("id", id).Msg("cancelling maintenance window")

	// Get current window to verify it can be cancelled
	window, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}

	// Only scheduled or active windows can be cancelled
	if window.Status != routingv1.MaintenanceStatus_MAINTENANCE_STATUS_SCHEDULED &&
		window.Status != routingv1.MaintenanceStatus_MAINTENANCE_STATUS_IN_PROGRESS {
		return maintenance.ErrInvalidStatus
	}

	err = s.store.UpdateStatus(ctx, id, routingv1.MaintenanceStatus_MAINTENANCE_STATUS_CANCELLED)
	if err != nil {
		s.logger.Error().Err(err).Str("id", id).Msg("failed to cancel maintenance window")
		return err
	}

	s.logger.Info().Str("id", id).Msg("maintenance window cancelled")
	return nil
}

// ListUpcomingMaintenanceWindows lists maintenance windows starting within the given duration.
func (s *MaintenanceService) ListUpcomingMaintenanceWindows(ctx context.Context, duration time.Duration) ([]*routingv1.MaintenanceWindow, error) {
	return s.checker.ListUpcoming(ctx, duration)
}

// Ensure MaintenanceService implements the interface
var _ routingv1.MaintenanceServiceServer = (*MaintenanceService)(nil)
