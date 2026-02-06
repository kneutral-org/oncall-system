// Package grpc provides gRPC service implementations.
package grpc

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kneutral-org/alerting-system/internal/schedule"
	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// ScheduleService implements the ScheduleServiceServer interface.
type ScheduleService struct {
	routingv1.UnimplementedScheduleServiceServer
	store      schedule.Store
	calculator *schedule.Calculator
	logger     zerolog.Logger
}

// NewScheduleService creates a new ScheduleService.
func NewScheduleService(store schedule.Store, logger zerolog.Logger) *ScheduleService {
	return &ScheduleService{
		store:      store,
		calculator: schedule.NewCalculator(),
		logger:     logger.With().Str("service", "schedule").Logger(),
	}
}

// =============================================================================
// Schedule CRUD (5 RPCs)
// =============================================================================

// CreateSchedule creates a new schedule.
func (s *ScheduleService) CreateSchedule(ctx context.Context, req *routingv1.CreateScheduleRequest) (*routingv1.Schedule, error) {
	if req.Schedule == nil {
		return nil, status.Error(codes.InvalidArgument, "schedule is required")
	}

	if req.Schedule.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "schedule name is required")
	}

	s.logger.Info().
		Str("name", req.Schedule.Name).
		Str("team_id", req.Schedule.TeamId).
		Msg("creating schedule")

	sched, err := s.store.CreateSchedule(ctx, req.Schedule)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to create schedule")
		return nil, status.Error(codes.Internal, "failed to create schedule")
	}

	s.logger.Info().
		Str("id", sched.Id).
		Str("name", sched.Name).
		Msg("schedule created")

	return sched, nil
}

// GetSchedule retrieves a schedule by ID.
func (s *ScheduleService) GetSchedule(ctx context.Context, req *routingv1.GetScheduleRequest) (*routingv1.Schedule, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	sched, err := s.store.GetSchedule(ctx, req.Id)
	if err != nil {
		if errors.Is(err, schedule.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "schedule not found")
		}
		s.logger.Error().Err(err).Str("id", req.Id).Msg("failed to get schedule")
		return nil, status.Error(codes.Internal, "failed to get schedule")
	}

	return sched, nil
}

// ListSchedules retrieves schedules with optional filters.
func (s *ScheduleService) ListSchedules(ctx context.Context, req *routingv1.ListSchedulesRequest) (*routingv1.ListSchedulesResponse, error) {
	resp, err := s.store.ListSchedules(ctx, req)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to list schedules")
		return nil, status.Error(codes.Internal, "failed to list schedules")
	}

	return resp, nil
}

// UpdateSchedule updates an existing schedule.
func (s *ScheduleService) UpdateSchedule(ctx context.Context, req *routingv1.UpdateScheduleRequest) (*routingv1.Schedule, error) {
	if req.Schedule == nil || req.Schedule.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "schedule with id is required")
	}

	s.logger.Info().
		Str("id", req.Schedule.Id).
		Str("name", req.Schedule.Name).
		Msg("updating schedule")

	sched, err := s.store.UpdateSchedule(ctx, req.Schedule)
	if err != nil {
		if errors.Is(err, schedule.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "schedule not found")
		}
		s.logger.Error().Err(err).Str("id", req.Schedule.Id).Msg("failed to update schedule")
		return nil, status.Error(codes.Internal, "failed to update schedule")
	}

	s.logger.Info().
		Str("id", sched.Id).
		Msg("schedule updated")

	return sched, nil
}

// DeleteSchedule deletes a schedule by ID.
func (s *ScheduleService) DeleteSchedule(ctx context.Context, req *routingv1.DeleteScheduleRequest) (*routingv1.DeleteScheduleResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	s.logger.Info().Str("id", req.Id).Msg("deleting schedule")

	err := s.store.DeleteSchedule(ctx, req.Id)
	if err != nil {
		if errors.Is(err, schedule.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "schedule not found")
		}
		s.logger.Error().Err(err).Str("id", req.Id).Msg("failed to delete schedule")
		return nil, status.Error(codes.Internal, "failed to delete schedule")
	}

	s.logger.Info().Str("id", req.Id).Msg("schedule deleted")

	return &routingv1.DeleteScheduleResponse{Success: true}, nil
}

// =============================================================================
// Rotation management (3 RPCs)
// =============================================================================

// AddRotation adds a rotation to a schedule.
func (s *ScheduleService) AddRotation(ctx context.Context, req *routingv1.AddRotationRequest) (*routingv1.Schedule, error) {
	if req.ScheduleId == "" {
		return nil, status.Error(codes.InvalidArgument, "schedule_id is required")
	}

	if req.Rotation == nil {
		return nil, status.Error(codes.InvalidArgument, "rotation is required")
	}

	s.logger.Info().
		Str("schedule_id", req.ScheduleId).
		Str("rotation_name", req.Rotation.Name).
		Msg("adding rotation to schedule")

	sched, err := s.store.AddRotation(ctx, req.ScheduleId, req.Rotation)
	if err != nil {
		if errors.Is(err, schedule.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "schedule not found")
		}
		s.logger.Error().Err(err).Str("schedule_id", req.ScheduleId).Msg("failed to add rotation")
		return nil, status.Error(codes.Internal, "failed to add rotation")
	}

	s.logger.Info().
		Str("schedule_id", req.ScheduleId).
		Msg("rotation added")

	return sched, nil
}

// UpdateRotation updates a rotation within a schedule.
func (s *ScheduleService) UpdateRotation(ctx context.Context, req *routingv1.UpdateRotationRequest) (*routingv1.Schedule, error) {
	if req.ScheduleId == "" {
		return nil, status.Error(codes.InvalidArgument, "schedule_id is required")
	}

	if req.Rotation == nil || req.Rotation.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "rotation with id is required")
	}

	s.logger.Info().
		Str("schedule_id", req.ScheduleId).
		Str("rotation_id", req.Rotation.Id).
		Msg("updating rotation")

	sched, err := s.store.UpdateRotation(ctx, req.ScheduleId, req.Rotation)
	if err != nil {
		if errors.Is(err, schedule.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "schedule or rotation not found")
		}
		s.logger.Error().Err(err).Str("rotation_id", req.Rotation.Id).Msg("failed to update rotation")
		return nil, status.Error(codes.Internal, "failed to update rotation")
	}

	s.logger.Info().
		Str("schedule_id", req.ScheduleId).
		Str("rotation_id", req.Rotation.Id).
		Msg("rotation updated")

	return sched, nil
}

// RemoveRotation removes a rotation from a schedule.
func (s *ScheduleService) RemoveRotation(ctx context.Context, req *routingv1.RemoveRotationRequest) (*routingv1.Schedule, error) {
	if req.ScheduleId == "" {
		return nil, status.Error(codes.InvalidArgument, "schedule_id is required")
	}

	if req.RotationId == "" {
		return nil, status.Error(codes.InvalidArgument, "rotation_id is required")
	}

	s.logger.Info().
		Str("schedule_id", req.ScheduleId).
		Str("rotation_id", req.RotationId).
		Msg("removing rotation from schedule")

	sched, err := s.store.RemoveRotation(ctx, req.ScheduleId, req.RotationId)
	if err != nil {
		if errors.Is(err, schedule.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "schedule or rotation not found")
		}
		s.logger.Error().Err(err).Str("rotation_id", req.RotationId).Msg("failed to remove rotation")
		return nil, status.Error(codes.Internal, "failed to remove rotation")
	}

	s.logger.Info().
		Str("schedule_id", req.ScheduleId).
		Str("rotation_id", req.RotationId).
		Msg("rotation removed")

	return sched, nil
}

// =============================================================================
// Override management (3 RPCs)
// =============================================================================

// CreateOverride creates a schedule override.
func (s *ScheduleService) CreateOverride(ctx context.Context, req *routingv1.CreateOverrideRequest) (*routingv1.ScheduleOverride, error) {
	if req.ScheduleId == "" {
		return nil, status.Error(codes.InvalidArgument, "schedule_id is required")
	}

	if req.Override == nil {
		return nil, status.Error(codes.InvalidArgument, "override is required")
	}

	if req.Override.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "override user_id is required")
	}

	if req.Override.StartTime == nil || req.Override.EndTime == nil {
		return nil, status.Error(codes.InvalidArgument, "override start_time and end_time are required")
	}

	if req.Override.StartTime.AsTime().After(req.Override.EndTime.AsTime()) {
		return nil, status.Error(codes.InvalidArgument, "start_time must be before end_time")
	}

	s.logger.Info().
		Str("schedule_id", req.ScheduleId).
		Str("user_id", req.Override.UserId).
		Time("start_time", req.Override.StartTime.AsTime()).
		Time("end_time", req.Override.EndTime.AsTime()).
		Msg("creating override")

	override, err := s.store.CreateOverride(ctx, req.ScheduleId, req.Override)
	if err != nil {
		if errors.Is(err, schedule.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "schedule not found")
		}
		s.logger.Error().Err(err).Str("schedule_id", req.ScheduleId).Msg("failed to create override")
		return nil, status.Error(codes.Internal, "failed to create override")
	}

	s.logger.Info().
		Str("schedule_id", req.ScheduleId).
		Str("override_id", override.Id).
		Msg("override created")

	return override, nil
}

// DeleteOverride deletes a schedule override.
func (s *ScheduleService) DeleteOverride(ctx context.Context, req *routingv1.DeleteOverrideRequest) (*routingv1.DeleteOverrideResponse, error) {
	if req.ScheduleId == "" {
		return nil, status.Error(codes.InvalidArgument, "schedule_id is required")
	}

	if req.OverrideId == "" {
		return nil, status.Error(codes.InvalidArgument, "override_id is required")
	}

	s.logger.Info().
		Str("schedule_id", req.ScheduleId).
		Str("override_id", req.OverrideId).
		Msg("deleting override")

	err := s.store.DeleteOverride(ctx, req.ScheduleId, req.OverrideId)
	if err != nil {
		if errors.Is(err, schedule.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "override not found")
		}
		s.logger.Error().Err(err).Str("override_id", req.OverrideId).Msg("failed to delete override")
		return nil, status.Error(codes.Internal, "failed to delete override")
	}

	s.logger.Info().
		Str("schedule_id", req.ScheduleId).
		Str("override_id", req.OverrideId).
		Msg("override deleted")

	return &routingv1.DeleteOverrideResponse{Success: true}, nil
}

// ListOverrides lists overrides for a schedule.
func (s *ScheduleService) ListOverrides(ctx context.Context, req *routingv1.ListOverridesRequest) (*routingv1.ListOverridesResponse, error) {
	if req.ScheduleId == "" {
		return nil, status.Error(codes.InvalidArgument, "schedule_id is required")
	}

	resp, err := s.store.ListOverrides(ctx, req.ScheduleId, req.StartTime, req.EndTime, int(req.PageSize), req.PageToken)
	if err != nil {
		s.logger.Error().Err(err).Str("schedule_id", req.ScheduleId).Msg("failed to list overrides")
		return nil, status.Error(codes.Internal, "failed to list overrides")
	}

	return resp, nil
}

// =============================================================================
// On-call queries (3 RPCs)
// =============================================================================

// GetCurrentOnCall returns the current on-call users for a schedule.
func (s *ScheduleService) GetCurrentOnCall(ctx context.Context, req *routingv1.GetCurrentOnCallRequest) (*routingv1.GetCurrentOnCallResponse, error) {
	if req.ScheduleId == "" {
		return nil, status.Error(codes.InvalidArgument, "schedule_id is required")
	}

	// Get schedule
	sched, err := s.store.GetSchedule(ctx, req.ScheduleId)
	if err != nil {
		if errors.Is(err, schedule.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "schedule not found")
		}
		s.logger.Error().Err(err).Str("schedule_id", req.ScheduleId).Msg("failed to get schedule")
		return nil, status.Error(codes.Internal, "failed to get schedule")
	}

	// Get active overrides
	now := time.Now()
	overrides, err := s.store.GetActiveOverrides(ctx, req.ScheduleId, now)
	if err != nil {
		s.logger.Warn().Err(err).Msg("failed to get active overrides, continuing without")
		overrides = nil
	}

	// Calculate who is on-call
	result := s.calculator.GetOnCallAt(sched, overrides, now)

	resp := &routingv1.GetCurrentOnCallResponse{
		PrimaryUserId:   result.PrimaryUserID,
		SecondaryUserId: result.SecondaryUserID,
		CurrentShift:    result.CurrentShift,
	}

	if !result.NextHandoff.IsZero() {
		resp.NextHandoff = timestamppb.New(result.NextHandoff)
	}

	return resp, nil
}

// GetOnCallAtTime returns who is on-call at a specific time.
func (s *ScheduleService) GetOnCallAtTime(ctx context.Context, req *routingv1.GetOnCallAtTimeRequest) (*routingv1.GetOnCallAtTimeResponse, error) {
	if req.ScheduleId == "" {
		return nil, status.Error(codes.InvalidArgument, "schedule_id is required")
	}

	if req.Time == nil {
		return nil, status.Error(codes.InvalidArgument, "time is required")
	}

	// Get schedule
	sched, err := s.store.GetSchedule(ctx, req.ScheduleId)
	if err != nil {
		if errors.Is(err, schedule.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "schedule not found")
		}
		s.logger.Error().Err(err).Str("schedule_id", req.ScheduleId).Msg("failed to get schedule")
		return nil, status.Error(codes.Internal, "failed to get schedule")
	}

	// Get active overrides for the specified time
	at := req.Time.AsTime()
	overrides, err := s.store.GetActiveOverrides(ctx, req.ScheduleId, at)
	if err != nil {
		s.logger.Warn().Err(err).Msg("failed to get active overrides, continuing without")
		overrides = nil
	}

	// Calculate who is on-call
	result := s.calculator.GetOnCallAt(sched, overrides, at)

	return &routingv1.GetOnCallAtTimeResponse{
		PrimaryUserId:   result.PrimaryUserID,
		SecondaryUserId: result.SecondaryUserID,
		Shift:           result.CurrentShift,
	}, nil
}

// ListUpcomingShifts lists upcoming shifts for a schedule.
func (s *ScheduleService) ListUpcomingShifts(ctx context.Context, req *routingv1.ListUpcomingShiftsRequest) (*routingv1.ListUpcomingShiftsResponse, error) {
	if req.ScheduleId == "" {
		return nil, status.Error(codes.InvalidArgument, "schedule_id is required")
	}

	// Get schedule
	sched, err := s.store.GetSchedule(ctx, req.ScheduleId)
	if err != nil {
		if errors.Is(err, schedule.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "schedule not found")
		}
		s.logger.Error().Err(err).Str("schedule_id", req.ScheduleId).Msg("failed to get schedule")
		return nil, status.Error(codes.Internal, "failed to get schedule")
	}

	// Determine time range
	from := time.Now()
	until := from.Add(30 * 24 * time.Hour) // Default: 30 days

	if req.Until != nil {
		until = req.Until.AsTime()
	}

	// Get overrides for the time range
	overridesResp, err := s.store.ListOverrides(ctx, req.ScheduleId, timestamppb.New(from), timestamppb.New(until), 100, "")
	if err != nil {
		s.logger.Warn().Err(err).Msg("failed to get overrides, continuing without")
		overridesResp = &routingv1.ListOverridesResponse{}
	}

	// Generate shifts
	shifts := s.calculator.ListUpcomingShifts(sched, overridesResp.Overrides, from, until, req.UserId)

	// Apply pagination
	pageSize := int(req.PageSize)
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}

	offset := 0
	if req.PageToken != "" {
		var scanOffset int
		if _, err := time.Parse(time.RFC3339, req.PageToken); err == nil {
			// Page token is a timestamp - find the offset
			for i, shift := range shifts {
				if shift.StartTime.AsTime().Format(time.RFC3339) >= req.PageToken {
					offset = i
					break
				}
			}
		} else if n, err := time.ParseDuration(req.PageToken); err == nil {
			offset = int(n.Hours())
		} else if _, err := time.Parse("", req.PageToken); err != nil {
			// Try to parse as integer offset
			if _, err := time.ParseDuration(req.PageToken); err == nil {
				offset = scanOffset
			}
		}
	}

	// Slice shifts for pagination
	if offset >= len(shifts) {
		return &routingv1.ListUpcomingShiftsResponse{
			Shifts:        []*routingv1.Shift{},
			NextPageToken: "",
		}, nil
	}

	end := offset + pageSize
	nextPageToken := ""
	if end < len(shifts) {
		nextPageToken = shifts[end].StartTime.AsTime().Format(time.RFC3339)
	}
	if end > len(shifts) {
		end = len(shifts)
	}

	return &routingv1.ListUpcomingShiftsResponse{
		Shifts:        shifts[offset:end],
		NextPageToken: nextPageToken,
	}, nil
}

// =============================================================================
// Handoff (2 RPCs)
// =============================================================================

// AcknowledgeHandoff acknowledges a handoff.
func (s *ScheduleService) AcknowledgeHandoff(ctx context.Context, req *routingv1.AcknowledgeHandoffRequest) (*routingv1.AcknowledgeHandoffResponse, error) {
	if req.ScheduleId == "" {
		return nil, status.Error(codes.InvalidArgument, "schedule_id is required")
	}

	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	s.logger.Info().
		Str("schedule_id", req.ScheduleId).
		Str("user_id", req.UserId).
		Msg("acknowledging handoff")

	// Get schedule
	sched, err := s.store.GetSchedule(ctx, req.ScheduleId)
	if err != nil {
		if errors.Is(err, schedule.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "schedule not found")
		}
		s.logger.Error().Err(err).Str("schedule_id", req.ScheduleId).Msg("failed to get schedule")
		return nil, status.Error(codes.Internal, "failed to get schedule")
	}

	// Get current on-call info
	now := time.Now()
	overrides, err := s.store.GetActiveOverrides(ctx, req.ScheduleId, now)
	if err != nil {
		overrides = nil
	}

	result := s.calculator.GetOnCallAt(sched, overrides, now)

	// Verify user is actually on-call
	if result.PrimaryUserID != req.UserId && result.SecondaryUserID != req.UserId {
		return nil, status.Error(codes.FailedPrecondition, "user is not currently on-call")
	}

	// Record the acknowledgment
	err = s.store.RecordHandoffAck(ctx, req.ScheduleId, req.UserId)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to record handoff acknowledgment")
		return nil, status.Error(codes.Internal, "failed to record handoff acknowledgment")
	}

	s.logger.Info().
		Str("schedule_id", req.ScheduleId).
		Str("user_id", req.UserId).
		Msg("handoff acknowledged")

	return &routingv1.AcknowledgeHandoffResponse{
		Success: true,
		Shift:   result.CurrentShift,
	}, nil
}

// GetHandoffSummary returns a summary of the upcoming handoff.
func (s *ScheduleService) GetHandoffSummary(ctx context.Context, req *routingv1.GetHandoffSummaryRequest) (*routingv1.HandoffSummary, error) {
	if req.ScheduleId == "" {
		return nil, status.Error(codes.InvalidArgument, "schedule_id is required")
	}

	// Get schedule
	sched, err := s.store.GetSchedule(ctx, req.ScheduleId)
	if err != nil {
		if errors.Is(err, schedule.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "schedule not found")
		}
		s.logger.Error().Err(err).Str("schedule_id", req.ScheduleId).Msg("failed to get schedule")
		return nil, status.Error(codes.Internal, "failed to get schedule")
	}

	// Get current on-call
	now := time.Now()
	overrides, err := s.store.GetActiveOverrides(ctx, req.ScheduleId, now)
	if err != nil {
		overrides = nil
	}

	currentResult := s.calculator.GetOnCallAt(sched, overrides, now)

	// Calculate next handoff time
	nextHandoff := s.calculator.CalculateNextHandoff(sched, overrides, now)

	// Get who will be on-call after handoff
	var incomingUserID string
	if !nextHandoff.IsZero() {
		// Add a small buffer to get the next on-call
		nextResult := s.calculator.GetOnCallAt(sched, nil, nextHandoff.Add(time.Minute))
		incomingUserID = nextResult.PrimaryUserID
	}

	summary := &routingv1.HandoffSummary{
		ScheduleId:     req.ScheduleId,
		OutgoingUserId: currentResult.PrimaryUserID,
		IncomingUserId: incomingUserID,
		ActiveAlerts:   []*routingv1.Alert{},   // Would be populated from alert service
		OpenTickets:    []*routingv1.TicketSummary{}, // Would be populated from ticket service
		RecentEvents:   []*routingv1.Event{},   // Would be populated from event service
		HandoffNotes:   "",                     // Would be populated from handoff notes storage
	}

	if !nextHandoff.IsZero() {
		summary.HandoffTime = timestamppb.New(nextHandoff)
	}

	return summary, nil
}

// Ensure ScheduleService implements the interface
var _ routingv1.ScheduleServiceServer = (*ScheduleService)(nil)
