package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kneutral-org/alerting-system/internal/schedule"
	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// TestInMemoryStore is an in-memory implementation for testing.
type TestInMemoryStore struct {
	schedules map[string]*routingv1.Schedule
	overrides map[string][]*routingv1.ScheduleOverride
	counter   int64
}

func NewTestInMemoryStore() *TestInMemoryStore {
	return &TestInMemoryStore{
		schedules: make(map[string]*routingv1.Schedule),
		overrides: make(map[string][]*routingv1.ScheduleOverride),
	}
}

func (s *TestInMemoryStore) CreateSchedule(ctx context.Context, sched *routingv1.Schedule) (*routingv1.Schedule, error) {
	if sched == nil {
		return nil, schedule.ErrInvalidSchedule
	}

	if sched.Id == "" {
		s.counter++
		sched.Id = "schedule-" + string(rune(s.counter+'0'))
	}

	now := time.Now()
	sched.CreatedAt = timestamppb.New(now)
	sched.UpdatedAt = timestamppb.New(now)

	if sched.Timezone == "" {
		sched.Timezone = "UTC"
	}

	s.schedules[sched.Id] = sched
	s.overrides[sched.Id] = sched.Overrides

	return sched, nil
}

func (s *TestInMemoryStore) GetSchedule(ctx context.Context, id string) (*routingv1.Schedule, error) {
	sched, ok := s.schedules[id]
	if !ok {
		return nil, schedule.ErrNotFound
	}
	return sched, nil
}

func (s *TestInMemoryStore) ListSchedules(ctx context.Context, req *routingv1.ListSchedulesRequest) (*routingv1.ListSchedulesResponse, error) {
	var schedules []*routingv1.Schedule

	for _, sched := range s.schedules {
		if req.TeamId != "" && sched.TeamId != req.TeamId {
			continue
		}
		schedules = append(schedules, sched)
	}

	return &routingv1.ListSchedulesResponse{
		Schedules:  schedules,
		TotalCount: int32(len(schedules)),
	}, nil
}

func (s *TestInMemoryStore) UpdateSchedule(ctx context.Context, sched *routingv1.Schedule) (*routingv1.Schedule, error) {
	if sched == nil || sched.Id == "" {
		return nil, schedule.ErrInvalidSchedule
	}

	existing, ok := s.schedules[sched.Id]
	if !ok {
		return nil, schedule.ErrNotFound
	}

	sched.CreatedAt = existing.CreatedAt
	sched.UpdatedAt = timestamppb.Now()

	s.schedules[sched.Id] = sched
	return sched, nil
}

func (s *TestInMemoryStore) DeleteSchedule(ctx context.Context, id string) error {
	if _, ok := s.schedules[id]; !ok {
		return schedule.ErrNotFound
	}
	delete(s.schedules, id)
	delete(s.overrides, id)
	return nil
}

func (s *TestInMemoryStore) AddRotation(ctx context.Context, scheduleID string, rotation *routingv1.Rotation) (*routingv1.Schedule, error) {
	sched, ok := s.schedules[scheduleID]
	if !ok {
		return nil, schedule.ErrNotFound
	}

	if rotation.Id == "" {
		s.counter++
		rotation.Id = "rotation-" + string(rune(s.counter+'0'))
	}

	sched.Rotations = append(sched.Rotations, rotation)
	sched.UpdatedAt = timestamppb.Now()

	return sched, nil
}

func (s *TestInMemoryStore) UpdateRotation(ctx context.Context, scheduleID string, rotation *routingv1.Rotation) (*routingv1.Schedule, error) {
	sched, ok := s.schedules[scheduleID]
	if !ok {
		return nil, schedule.ErrNotFound
	}

	found := false
	for i, r := range sched.Rotations {
		if r.Id == rotation.Id {
			sched.Rotations[i] = rotation
			found = true
			break
		}
	}

	if !found {
		return nil, schedule.ErrNotFound
	}

	sched.UpdatedAt = timestamppb.Now()
	return sched, nil
}

func (s *TestInMemoryStore) RemoveRotation(ctx context.Context, scheduleID, rotationID string) (*routingv1.Schedule, error) {
	sched, ok := s.schedules[scheduleID]
	if !ok {
		return nil, schedule.ErrNotFound
	}

	found := false
	newRotations := make([]*routingv1.Rotation, 0)
	for _, r := range sched.Rotations {
		if r.Id == rotationID {
			found = true
			continue
		}
		newRotations = append(newRotations, r)
	}

	if !found {
		return nil, schedule.ErrNotFound
	}

	sched.Rotations = newRotations
	sched.UpdatedAt = timestamppb.Now()
	return sched, nil
}

func (s *TestInMemoryStore) CreateOverride(ctx context.Context, scheduleID string, override *routingv1.ScheduleOverride) (*routingv1.ScheduleOverride, error) {
	if _, ok := s.schedules[scheduleID]; !ok {
		return nil, schedule.ErrNotFound
	}

	if override.Id == "" {
		s.counter++
		override.Id = "override-" + string(rune(s.counter+'0'))
	}

	override.CreatedAt = timestamppb.Now()
	s.overrides[scheduleID] = append(s.overrides[scheduleID], override)

	return override, nil
}

func (s *TestInMemoryStore) DeleteOverride(ctx context.Context, scheduleID, overrideID string) error {
	overrides, ok := s.overrides[scheduleID]
	if !ok {
		return schedule.ErrNotFound
	}

	found := false
	newOverrides := make([]*routingv1.ScheduleOverride, 0)
	for _, o := range overrides {
		if o.Id == overrideID {
			found = true
			continue
		}
		newOverrides = append(newOverrides, o)
	}

	if !found {
		return schedule.ErrNotFound
	}

	s.overrides[scheduleID] = newOverrides
	return nil
}

func (s *TestInMemoryStore) ListOverrides(ctx context.Context, scheduleID string, startTime, endTime *timestamppb.Timestamp, pageSize int, pageToken string) (*routingv1.ListOverridesResponse, error) {
	overrides := s.overrides[scheduleID]

	var filtered []*routingv1.ScheduleOverride
	for _, o := range overrides {
		if startTime != nil && o.EndTime.AsTime().Before(startTime.AsTime()) {
			continue
		}
		if endTime != nil && o.StartTime.AsTime().After(endTime.AsTime()) {
			continue
		}
		filtered = append(filtered, o)
	}

	return &routingv1.ListOverridesResponse{
		Overrides: filtered,
	}, nil
}

func (s *TestInMemoryStore) GetActiveOverrides(ctx context.Context, scheduleID string, at time.Time) ([]*routingv1.ScheduleOverride, error) {
	overrides := s.overrides[scheduleID]

	var active []*routingv1.ScheduleOverride
	for _, o := range overrides {
		if !at.Before(o.StartTime.AsTime()) && at.Before(o.EndTime.AsTime()) {
			active = append(active, o)
		}
	}

	return active, nil
}

func (s *TestInMemoryStore) RecordHandoffAck(ctx context.Context, scheduleID, userID string) error {
	if _, ok := s.schedules[scheduleID]; !ok {
		return schedule.ErrNotFound
	}
	return nil
}

// Ensure TestInMemoryStore implements schedule.Store
var _ schedule.Store = (*TestInMemoryStore)(nil)

// =============================================================================
// Tests
// =============================================================================

func newTestScheduleService() *ScheduleService {
	store := NewTestInMemoryStore()
	logger := zerolog.Nop()
	return NewScheduleService(store, logger)
}

func TestScheduleService_CreateSchedule(t *testing.T) {
	svc := newTestScheduleService()
	ctx := context.Background()

	req := &routingv1.CreateScheduleRequest{
		Schedule: &routingv1.Schedule{
			Name:        "Test Schedule",
			Description: "Test description",
			TeamId:      "team-1",
			Timezone:    "America/New_York",
		},
	}

	resp, err := svc.CreateSchedule(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Id == "" {
		t.Error("expected schedule ID to be generated")
	}

	if resp.Name != "Test Schedule" {
		t.Errorf("expected name 'Test Schedule', got '%s'", resp.Name)
	}
}

func TestScheduleService_CreateSchedule_InvalidInput(t *testing.T) {
	svc := newTestScheduleService()
	ctx := context.Background()

	tests := []struct {
		name string
		req  *routingv1.CreateScheduleRequest
	}{
		{
			name: "nil schedule",
			req:  &routingv1.CreateScheduleRequest{},
		},
		{
			name: "empty name",
			req: &routingv1.CreateScheduleRequest{
				Schedule: &routingv1.Schedule{Name: ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateSchedule(ctx, tt.req)
			if err == nil {
				t.Error("expected error for invalid input")
			}

			st, ok := status.FromError(err)
			if !ok {
				t.Errorf("expected gRPC status error, got %v", err)
			}
			if st.Code() != codes.InvalidArgument {
				t.Errorf("expected InvalidArgument, got %v", st.Code())
			}
		})
	}
}

func TestScheduleService_GetSchedule(t *testing.T) {
	svc := newTestScheduleService()
	ctx := context.Background()

	// Create a schedule
	created, _ := svc.CreateSchedule(ctx, &routingv1.CreateScheduleRequest{
		Schedule: &routingv1.Schedule{Name: "Test Schedule"},
	})

	// Get the schedule
	resp, err := svc.GetSchedule(ctx, &routingv1.GetScheduleRequest{Id: created.Id})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Name != "Test Schedule" {
		t.Errorf("expected name 'Test Schedule', got '%s'", resp.Name)
	}
}

func TestScheduleService_GetSchedule_NotFound(t *testing.T) {
	svc := newTestScheduleService()
	ctx := context.Background()

	_, err := svc.GetSchedule(ctx, &routingv1.GetScheduleRequest{Id: "nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent schedule")
	}

	st, _ := status.FromError(err)
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", st.Code())
	}
}

func TestScheduleService_ListSchedules(t *testing.T) {
	svc := newTestScheduleService()
	ctx := context.Background()

	// Create schedules
	for i := 0; i < 3; i++ {
		_, _ = svc.CreateSchedule(ctx, &routingv1.CreateScheduleRequest{
			Schedule: &routingv1.Schedule{
				Name:   "Schedule " + string(rune('A'+i)),
				TeamId: "team-1",
			},
		})
	}

	resp, err := svc.ListSchedules(ctx, &routingv1.ListSchedulesRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Schedules) != 3 {
		t.Errorf("expected 3 schedules, got %d", len(resp.Schedules))
	}
}

func TestScheduleService_UpdateSchedule(t *testing.T) {
	svc := newTestScheduleService()
	ctx := context.Background()

	// Create a schedule
	created, _ := svc.CreateSchedule(ctx, &routingv1.CreateScheduleRequest{
		Schedule: &routingv1.Schedule{Name: "Original Name"},
	})

	// Update the schedule
	created.Name = "Updated Name"
	resp, err := svc.UpdateSchedule(ctx, &routingv1.UpdateScheduleRequest{Schedule: created})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got '%s'", resp.Name)
	}
}

func TestScheduleService_DeleteSchedule(t *testing.T) {
	svc := newTestScheduleService()
	ctx := context.Background()

	// Create a schedule
	created, _ := svc.CreateSchedule(ctx, &routingv1.CreateScheduleRequest{
		Schedule: &routingv1.Schedule{Name: "Test Schedule"},
	})

	// Delete the schedule
	resp, err := svc.DeleteSchedule(ctx, &routingv1.DeleteScheduleRequest{Id: created.Id})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Success {
		t.Error("expected success to be true")
	}

	// Verify it's deleted
	_, err = svc.GetSchedule(ctx, &routingv1.GetScheduleRequest{Id: created.Id})
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestScheduleService_AddRotation(t *testing.T) {
	svc := newTestScheduleService()
	ctx := context.Background()

	// Create a schedule
	created, _ := svc.CreateSchedule(ctx, &routingv1.CreateScheduleRequest{
		Schedule: &routingv1.Schedule{Name: "Test Schedule"},
	})

	// Add a rotation
	rotation := &routingv1.Rotation{
		Name: "Primary",
		Type: routingv1.RotationType_ROTATION_TYPE_WEEKLY,
		Members: []*routingv1.RotationMember{
			{UserId: "user-1", Position: 0},
		},
		StartTime: timestamppb.Now(),
		ShiftConfig: &routingv1.ShiftConfig{
			ShiftLength: durationpb.New(7 * 24 * time.Hour),
		},
	}

	resp, err := svc.AddRotation(ctx, &routingv1.AddRotationRequest{
		ScheduleId: created.Id,
		Rotation:   rotation,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Rotations) != 1 {
		t.Errorf("expected 1 rotation, got %d", len(resp.Rotations))
	}
}

func TestScheduleService_RemoveRotation(t *testing.T) {
	svc := newTestScheduleService()
	ctx := context.Background()

	// Create a schedule with a rotation
	created, _ := svc.CreateSchedule(ctx, &routingv1.CreateScheduleRequest{
		Schedule: &routingv1.Schedule{
			Name: "Test Schedule",
			Rotations: []*routingv1.Rotation{
				{Id: "rotation-1", Name: "Primary"},
			},
		},
	})

	// Remove the rotation
	resp, err := svc.RemoveRotation(ctx, &routingv1.RemoveRotationRequest{
		ScheduleId: created.Id,
		RotationId: "rotation-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Rotations) != 0 {
		t.Errorf("expected 0 rotations, got %d", len(resp.Rotations))
	}
}

func TestScheduleService_CreateOverride(t *testing.T) {
	svc := newTestScheduleService()
	ctx := context.Background()

	// Create a schedule
	created, _ := svc.CreateSchedule(ctx, &routingv1.CreateScheduleRequest{
		Schedule: &routingv1.Schedule{Name: "Test Schedule"},
	})

	now := time.Now()
	override := &routingv1.ScheduleOverride{
		UserId:    "user-1",
		StartTime: timestamppb.New(now),
		EndTime:   timestamppb.New(now.Add(8 * time.Hour)),
		Reason:    "Covering for vacation",
	}

	resp, err := svc.CreateOverride(ctx, &routingv1.CreateOverrideRequest{
		ScheduleId: created.Id,
		Override:   override,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Id == "" {
		t.Error("expected override ID to be generated")
	}

	if resp.UserId != "user-1" {
		t.Errorf("expected user_id 'user-1', got '%s'", resp.UserId)
	}
}

func TestScheduleService_CreateOverride_InvalidInput(t *testing.T) {
	svc := newTestScheduleService()
	ctx := context.Background()

	// Create a schedule
	created, _ := svc.CreateSchedule(ctx, &routingv1.CreateScheduleRequest{
		Schedule: &routingv1.Schedule{Name: "Test Schedule"},
	})

	now := time.Now()

	tests := []struct {
		name string
		req  *routingv1.CreateOverrideRequest
	}{
		{
			name: "missing schedule_id",
			req:  &routingv1.CreateOverrideRequest{},
		},
		{
			name: "missing override",
			req:  &routingv1.CreateOverrideRequest{ScheduleId: created.Id},
		},
		{
			name: "missing user_id",
			req: &routingv1.CreateOverrideRequest{
				ScheduleId: created.Id,
				Override: &routingv1.ScheduleOverride{
					StartTime: timestamppb.New(now),
					EndTime:   timestamppb.New(now.Add(time.Hour)),
				},
			},
		},
		{
			name: "start after end",
			req: &routingv1.CreateOverrideRequest{
				ScheduleId: created.Id,
				Override: &routingv1.ScheduleOverride{
					UserId:    "user-1",
					StartTime: timestamppb.New(now.Add(time.Hour)),
					EndTime:   timestamppb.New(now),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateOverride(ctx, tt.req)
			if err == nil {
				t.Error("expected error for invalid input")
			}

			st, ok := status.FromError(err)
			if !ok {
				t.Errorf("expected gRPC status error, got %v", err)
			}
			if st.Code() != codes.InvalidArgument {
				t.Errorf("expected InvalidArgument, got %v", st.Code())
			}
		})
	}
}

func TestScheduleService_DeleteOverride(t *testing.T) {
	svc := newTestScheduleService()
	ctx := context.Background()

	// Create a schedule
	created, _ := svc.CreateSchedule(ctx, &routingv1.CreateScheduleRequest{
		Schedule: &routingv1.Schedule{Name: "Test Schedule"},
	})

	now := time.Now()
	override, _ := svc.CreateOverride(ctx, &routingv1.CreateOverrideRequest{
		ScheduleId: created.Id,
		Override: &routingv1.ScheduleOverride{
			UserId:    "user-1",
			StartTime: timestamppb.New(now),
			EndTime:   timestamppb.New(now.Add(time.Hour)),
		},
	})

	// Delete the override
	resp, err := svc.DeleteOverride(ctx, &routingv1.DeleteOverrideRequest{
		ScheduleId: created.Id,
		OverrideId: override.Id,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Success {
		t.Error("expected success to be true")
	}
}

func TestScheduleService_GetCurrentOnCall(t *testing.T) {
	svc := newTestScheduleService()
	ctx := context.Background()

	// Create a schedule with a rotation
	now := time.Now()
	rotationStart := now.Add(-24 * time.Hour)

	created, _ := svc.CreateSchedule(ctx, &routingv1.CreateScheduleRequest{
		Schedule: &routingv1.Schedule{
			Name:     "Test Schedule",
			Timezone: "UTC",
			Rotations: []*routingv1.Rotation{
				{
					Id:        "rotation-1",
					Name:      "Primary",
					Type:      routingv1.RotationType_ROTATION_TYPE_DAILY,
					Layer:     1,
					StartTime: timestamppb.New(rotationStart),
					ShiftConfig: &routingv1.ShiftConfig{
						ShiftLength: durationpb.New(24 * time.Hour),
					},
					Members: []*routingv1.RotationMember{
						{UserId: "user-1", Position: 0},
						{UserId: "user-2", Position: 1},
					},
				},
			},
		},
	})

	resp, err := svc.GetCurrentOnCall(ctx, &routingv1.GetCurrentOnCallRequest{
		ScheduleId: created.Id,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.PrimaryUserId == "" {
		t.Error("expected primary user to be set")
	}

	if resp.CurrentShift == nil {
		t.Error("expected current shift to be set")
	}
}

func TestScheduleService_GetOnCallAtTime(t *testing.T) {
	svc := newTestScheduleService()
	ctx := context.Background()

	// Create a schedule with a rotation
	now := time.Now()
	rotationStart := now.Add(-48 * time.Hour)

	created, _ := svc.CreateSchedule(ctx, &routingv1.CreateScheduleRequest{
		Schedule: &routingv1.Schedule{
			Name:     "Test Schedule",
			Timezone: "UTC",
			Rotations: []*routingv1.Rotation{
				{
					Id:        "rotation-1",
					Name:      "Primary",
					Type:      routingv1.RotationType_ROTATION_TYPE_DAILY,
					Layer:     1,
					StartTime: timestamppb.New(rotationStart),
					ShiftConfig: &routingv1.ShiftConfig{
						ShiftLength: durationpb.New(24 * time.Hour),
					},
					Members: []*routingv1.RotationMember{
						{UserId: "user-1", Position: 0},
						{UserId: "user-2", Position: 1},
					},
				},
			},
		},
	})

	// Query at rotation start (should be user-1)
	resp, err := svc.GetOnCallAtTime(ctx, &routingv1.GetOnCallAtTimeRequest{
		ScheduleId: created.Id,
		Time:       timestamppb.New(rotationStart.Add(time.Hour)),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.PrimaryUserId != "user-1" {
		t.Errorf("expected user-1 at rotation start, got '%s'", resp.PrimaryUserId)
	}
}

func TestScheduleService_ListUpcomingShifts(t *testing.T) {
	svc := newTestScheduleService()
	ctx := context.Background()

	// Create a schedule with a rotation
	now := time.Now().Truncate(24 * time.Hour)

	created, _ := svc.CreateSchedule(ctx, &routingv1.CreateScheduleRequest{
		Schedule: &routingv1.Schedule{
			Name:     "Test Schedule",
			Timezone: "UTC",
			Rotations: []*routingv1.Rotation{
				{
					Id:        "rotation-1",
					Name:      "Primary",
					Type:      routingv1.RotationType_ROTATION_TYPE_DAILY,
					Layer:     1,
					StartTime: timestamppb.New(now),
					ShiftConfig: &routingv1.ShiftConfig{
						ShiftLength: durationpb.New(24 * time.Hour),
					},
					Members: []*routingv1.RotationMember{
						{UserId: "user-1", Position: 0},
						{UserId: "user-2", Position: 1},
					},
				},
			},
		},
	})

	resp, err := svc.ListUpcomingShifts(ctx, &routingv1.ListUpcomingShiftsRequest{
		ScheduleId: created.Id,
		Until:      timestamppb.New(now.Add(7 * 24 * time.Hour)),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Shifts) < 7 {
		t.Errorf("expected at least 7 shifts, got %d", len(resp.Shifts))
	}
}

func TestScheduleService_AcknowledgeHandoff(t *testing.T) {
	svc := newTestScheduleService()
	ctx := context.Background()

	// Create a schedule with a rotation
	now := time.Now()
	rotationStart := now.Add(-12 * time.Hour)

	created, _ := svc.CreateSchedule(ctx, &routingv1.CreateScheduleRequest{
		Schedule: &routingv1.Schedule{
			Name:     "Test Schedule",
			Timezone: "UTC",
			Rotations: []*routingv1.Rotation{
				{
					Id:        "rotation-1",
					Name:      "Primary",
					Type:      routingv1.RotationType_ROTATION_TYPE_DAILY,
					Layer:     1,
					StartTime: timestamppb.New(rotationStart),
					ShiftConfig: &routingv1.ShiftConfig{
						ShiftLength: durationpb.New(24 * time.Hour),
					},
					Members: []*routingv1.RotationMember{
						{UserId: "user-1", Position: 0},
					},
				},
			},
		},
	})

	// Acknowledge handoff as the on-call user
	resp, err := svc.AcknowledgeHandoff(ctx, &routingv1.AcknowledgeHandoffRequest{
		ScheduleId: created.Id,
		UserId:     "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Success {
		t.Error("expected success to be true")
	}
}

func TestScheduleService_AcknowledgeHandoff_NotOnCall(t *testing.T) {
	svc := newTestScheduleService()
	ctx := context.Background()

	// Create a schedule with a rotation
	now := time.Now()
	rotationStart := now.Add(-12 * time.Hour)

	created, _ := svc.CreateSchedule(ctx, &routingv1.CreateScheduleRequest{
		Schedule: &routingv1.Schedule{
			Name:     "Test Schedule",
			Timezone: "UTC",
			Rotations: []*routingv1.Rotation{
				{
					Id:        "rotation-1",
					Name:      "Primary",
					Type:      routingv1.RotationType_ROTATION_TYPE_DAILY,
					Layer:     1,
					StartTime: timestamppb.New(rotationStart),
					ShiftConfig: &routingv1.ShiftConfig{
						ShiftLength: durationpb.New(24 * time.Hour),
					},
					Members: []*routingv1.RotationMember{
						{UserId: "user-1", Position: 0},
					},
				},
			},
		},
	})

	// Try to acknowledge handoff as a user who is NOT on-call
	_, err := svc.AcknowledgeHandoff(ctx, &routingv1.AcknowledgeHandoffRequest{
		ScheduleId: created.Id,
		UserId:     "user-not-oncall",
	})
	if err == nil {
		t.Error("expected error when user is not on-call")
	}

	st, _ := status.FromError(err)
	if st.Code() != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition, got %v", st.Code())
	}
}

func TestScheduleService_GetHandoffSummary(t *testing.T) {
	svc := newTestScheduleService()
	ctx := context.Background()

	// Create a schedule with a rotation
	now := time.Now()
	rotationStart := now.Add(-12 * time.Hour)

	created, _ := svc.CreateSchedule(ctx, &routingv1.CreateScheduleRequest{
		Schedule: &routingv1.Schedule{
			Name:     "Test Schedule",
			Timezone: "UTC",
			Rotations: []*routingv1.Rotation{
				{
					Id:        "rotation-1",
					Name:      "Primary",
					Type:      routingv1.RotationType_ROTATION_TYPE_DAILY,
					Layer:     1,
					StartTime: timestamppb.New(rotationStart),
					ShiftConfig: &routingv1.ShiftConfig{
						ShiftLength: durationpb.New(24 * time.Hour),
					},
					Members: []*routingv1.RotationMember{
						{UserId: "user-1", Position: 0},
						{UserId: "user-2", Position: 1},
					},
				},
			},
		},
	})

	resp, err := svc.GetHandoffSummary(ctx, &routingv1.GetHandoffSummaryRequest{
		ScheduleId: created.Id,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.ScheduleId != created.Id {
		t.Errorf("expected schedule_id '%s', got '%s'", created.Id, resp.ScheduleId)
	}

	if resp.OutgoingUserId == "" {
		t.Error("expected outgoing user to be set")
	}

	if resp.HandoffTime == nil {
		t.Error("expected handoff time to be set")
	}
}
