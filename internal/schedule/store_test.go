package schedule

import (
	"context"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// InMemoryStore is an in-memory implementation of Store for testing.
type InMemoryStore struct {
	schedules map[string]*routingv1.Schedule
	overrides map[string][]*routingv1.ScheduleOverride
	counter   int64
}

// NewInMemoryStore creates a new in-memory store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		schedules: make(map[string]*routingv1.Schedule),
		overrides: make(map[string][]*routingv1.ScheduleOverride),
	}
}

// CreateSchedule creates a new schedule in memory.
func (s *InMemoryStore) CreateSchedule(ctx context.Context, schedule *routingv1.Schedule) (*routingv1.Schedule, error) {
	if schedule == nil {
		return nil, ErrInvalidSchedule
	}

	if schedule.Id == "" {
		s.counter++
		schedule.Id = "schedule-" + string(rune(s.counter))
	}

	now := time.Now()
	schedule.CreatedAt = timestamppb.New(now)
	schedule.UpdatedAt = timestamppb.New(now)

	if schedule.Timezone == "" {
		schedule.Timezone = "UTC"
	}

	s.schedules[schedule.Id] = schedule
	s.overrides[schedule.Id] = schedule.Overrides

	return schedule, nil
}

// GetSchedule retrieves a schedule by ID.
func (s *InMemoryStore) GetSchedule(ctx context.Context, id string) (*routingv1.Schedule, error) {
	schedule, ok := s.schedules[id]
	if !ok {
		return nil, ErrNotFound
	}
	return schedule, nil
}

// ListSchedules retrieves schedules with optional filters.
func (s *InMemoryStore) ListSchedules(ctx context.Context, req *routingv1.ListSchedulesRequest) (*routingv1.ListSchedulesResponse, error) {
	var schedules []*routingv1.Schedule

	for _, schedule := range s.schedules {
		if req.TeamId != "" && schedule.TeamId != req.TeamId {
			continue
		}
		schedules = append(schedules, schedule)
	}

	return &routingv1.ListSchedulesResponse{
		Schedules:  schedules,
		TotalCount: int32(len(schedules)),
	}, nil
}

// UpdateSchedule updates an existing schedule.
func (s *InMemoryStore) UpdateSchedule(ctx context.Context, schedule *routingv1.Schedule) (*routingv1.Schedule, error) {
	if schedule == nil || schedule.Id == "" {
		return nil, ErrInvalidSchedule
	}

	existing, ok := s.schedules[schedule.Id]
	if !ok {
		return nil, ErrNotFound
	}

	schedule.CreatedAt = existing.CreatedAt
	schedule.UpdatedAt = timestamppb.Now()

	s.schedules[schedule.Id] = schedule
	return schedule, nil
}

// DeleteSchedule deletes a schedule by ID.
func (s *InMemoryStore) DeleteSchedule(ctx context.Context, id string) error {
	if _, ok := s.schedules[id]; !ok {
		return ErrNotFound
	}
	delete(s.schedules, id)
	delete(s.overrides, id)
	return nil
}

// AddRotation adds a rotation to a schedule.
func (s *InMemoryStore) AddRotation(ctx context.Context, scheduleID string, rotation *routingv1.Rotation) (*routingv1.Schedule, error) {
	schedule, ok := s.schedules[scheduleID]
	if !ok {
		return nil, ErrNotFound
	}

	if rotation.Id == "" {
		s.counter++
		rotation.Id = "rotation-" + string(rune(s.counter))
	}

	schedule.Rotations = append(schedule.Rotations, rotation)
	schedule.UpdatedAt = timestamppb.Now()

	return schedule, nil
}

// UpdateRotation updates a rotation within a schedule.
func (s *InMemoryStore) UpdateRotation(ctx context.Context, scheduleID string, rotation *routingv1.Rotation) (*routingv1.Schedule, error) {
	schedule, ok := s.schedules[scheduleID]
	if !ok {
		return nil, ErrNotFound
	}

	found := false
	for i, r := range schedule.Rotations {
		if r.Id == rotation.Id {
			schedule.Rotations[i] = rotation
			found = true
			break
		}
	}

	if !found {
		return nil, ErrNotFound
	}

	schedule.UpdatedAt = timestamppb.Now()
	return schedule, nil
}

// RemoveRotation removes a rotation from a schedule.
func (s *InMemoryStore) RemoveRotation(ctx context.Context, scheduleID, rotationID string) (*routingv1.Schedule, error) {
	schedule, ok := s.schedules[scheduleID]
	if !ok {
		return nil, ErrNotFound
	}

	found := false
	newRotations := make([]*routingv1.Rotation, 0)
	for _, r := range schedule.Rotations {
		if r.Id == rotationID {
			found = true
			continue
		}
		newRotations = append(newRotations, r)
	}

	if !found {
		return nil, ErrNotFound
	}

	schedule.Rotations = newRotations
	schedule.UpdatedAt = timestamppb.Now()
	return schedule, nil
}

// CreateOverride creates a schedule override.
func (s *InMemoryStore) CreateOverride(ctx context.Context, scheduleID string, override *routingv1.ScheduleOverride) (*routingv1.ScheduleOverride, error) {
	if _, ok := s.schedules[scheduleID]; !ok {
		return nil, ErrNotFound
	}

	if override.Id == "" {
		s.counter++
		override.Id = "override-" + string(rune(s.counter))
	}

	override.CreatedAt = timestamppb.Now()
	s.overrides[scheduleID] = append(s.overrides[scheduleID], override)

	return override, nil
}

// DeleteOverride deletes a schedule override.
func (s *InMemoryStore) DeleteOverride(ctx context.Context, scheduleID, overrideID string) error {
	overrides, ok := s.overrides[scheduleID]
	if !ok {
		return ErrNotFound
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
		return ErrNotFound
	}

	s.overrides[scheduleID] = newOverrides
	return nil
}

// ListOverrides lists overrides for a schedule within a time range.
func (s *InMemoryStore) ListOverrides(ctx context.Context, scheduleID string, startTime, endTime *timestamppb.Timestamp, pageSize int, pageToken string) (*routingv1.ListOverridesResponse, error) {
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

// GetActiveOverrides returns overrides active at a given time.
func (s *InMemoryStore) GetActiveOverrides(ctx context.Context, scheduleID string, at time.Time) ([]*routingv1.ScheduleOverride, error) {
	overrides := s.overrides[scheduleID]

	var active []*routingv1.ScheduleOverride
	for _, o := range overrides {
		if !at.Before(o.StartTime.AsTime()) && at.Before(o.EndTime.AsTime()) {
			active = append(active, o)
		}
	}

	return active, nil
}

// RecordHandoffAck records a handoff acknowledgment.
func (s *InMemoryStore) RecordHandoffAck(ctx context.Context, scheduleID, userID string) error {
	if _, ok := s.schedules[scheduleID]; !ok {
		return ErrNotFound
	}
	return nil
}

// Ensure InMemoryStore implements Store
var _ Store = (*InMemoryStore)(nil)

// =============================================================================
// Tests
// =============================================================================

func TestInMemoryStore_CreateSchedule(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	schedule := &routingv1.Schedule{
		Name:        "Test Schedule",
		Description: "Test description",
		TeamId:      "team-1",
		Timezone:    "America/New_York",
	}

	created, err := store.CreateSchedule(ctx, schedule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if created.Id == "" {
		t.Error("expected schedule ID to be generated")
	}

	if created.Name != "Test Schedule" {
		t.Errorf("expected name 'Test Schedule', got '%s'", created.Name)
	}

	if created.CreatedAt == nil {
		t.Error("expected CreatedAt to be set")
	}
}

func TestInMemoryStore_GetSchedule(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create a schedule
	schedule := &routingv1.Schedule{
		Id:   "test-schedule-1",
		Name: "Test Schedule",
	}
	_, err := store.CreateSchedule(ctx, schedule)
	if err != nil {
		t.Fatalf("unexpected error creating schedule: %v", err)
	}

	// Get the schedule
	retrieved, err := store.GetSchedule(ctx, "test-schedule-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.Name != "Test Schedule" {
		t.Errorf("expected name 'Test Schedule', got '%s'", retrieved.Name)
	}
}

func TestInMemoryStore_GetSchedule_NotFound(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_, err := store.GetSchedule(ctx, "nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestInMemoryStore_ListSchedules(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create schedules
	for i := 0; i < 3; i++ {
		schedule := &routingv1.Schedule{
			Name:   "Schedule " + string(rune('A'+i)),
			TeamId: "team-1",
		}
		_, _ = store.CreateSchedule(ctx, schedule)
	}

	// List all schedules
	resp, err := store.ListSchedules(ctx, &routingv1.ListSchedulesRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Schedules) != 3 {
		t.Errorf("expected 3 schedules, got %d", len(resp.Schedules))
	}
}

func TestInMemoryStore_ListSchedules_ByTeam(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create schedules for different teams
	schedule1 := &routingv1.Schedule{Name: "Schedule A", TeamId: "team-1"}
	schedule2 := &routingv1.Schedule{Name: "Schedule B", TeamId: "team-2"}
	schedule3 := &routingv1.Schedule{Name: "Schedule C", TeamId: "team-1"}

	_, _ = store.CreateSchedule(ctx, schedule1)
	_, _ = store.CreateSchedule(ctx, schedule2)
	_, _ = store.CreateSchedule(ctx, schedule3)

	// List schedules for team-1
	resp, err := store.ListSchedules(ctx, &routingv1.ListSchedulesRequest{TeamId: "team-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Schedules) != 2 {
		t.Errorf("expected 2 schedules for team-1, got %d", len(resp.Schedules))
	}
}

func TestInMemoryStore_UpdateSchedule(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create a schedule
	schedule := &routingv1.Schedule{
		Id:   "test-schedule",
		Name: "Original Name",
	}
	_, _ = store.CreateSchedule(ctx, schedule)

	// Update the schedule
	schedule.Name = "Updated Name"
	updated, err := store.UpdateSchedule(ctx, schedule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got '%s'", updated.Name)
	}
}

func TestInMemoryStore_DeleteSchedule(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create a schedule
	schedule := &routingv1.Schedule{
		Id:   "test-schedule",
		Name: "Test Schedule",
	}
	_, _ = store.CreateSchedule(ctx, schedule)

	// Delete the schedule
	err := store.DeleteSchedule(ctx, "test-schedule")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it's deleted
	_, err = store.GetSchedule(ctx, "test-schedule")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after deletion, got %v", err)
	}
}

func TestInMemoryStore_AddRotation(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create a schedule
	schedule := &routingv1.Schedule{
		Id:   "test-schedule",
		Name: "Test Schedule",
	}
	_, _ = store.CreateSchedule(ctx, schedule)

	// Add a rotation
	rotation := &routingv1.Rotation{
		Name: "Primary",
		Type: routingv1.RotationType_ROTATION_TYPE_WEEKLY,
		Members: []*routingv1.RotationMember{
			{UserId: "user-1", Position: 0},
			{UserId: "user-2", Position: 1},
		},
		StartTime: timestamppb.Now(),
		ShiftConfig: &routingv1.ShiftConfig{
			ShiftLength: durationpb.New(7 * 24 * time.Hour),
			HandoffTime: "09:00",
		},
	}

	updated, err := store.AddRotation(ctx, "test-schedule", rotation)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(updated.Rotations) != 1 {
		t.Errorf("expected 1 rotation, got %d", len(updated.Rotations))
	}

	if updated.Rotations[0].Name != "Primary" {
		t.Errorf("expected rotation name 'Primary', got '%s'", updated.Rotations[0].Name)
	}
}

func TestInMemoryStore_RemoveRotation(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create a schedule with a rotation
	schedule := &routingv1.Schedule{
		Id:   "test-schedule",
		Name: "Test Schedule",
		Rotations: []*routingv1.Rotation{
			{Id: "rotation-1", Name: "Primary"},
		},
	}
	_, _ = store.CreateSchedule(ctx, schedule)

	// Remove the rotation
	updated, err := store.RemoveRotation(ctx, "test-schedule", "rotation-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(updated.Rotations) != 0 {
		t.Errorf("expected 0 rotations after removal, got %d", len(updated.Rotations))
	}
}

func TestInMemoryStore_CreateOverride(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create a schedule
	schedule := &routingv1.Schedule{
		Id:   "test-schedule",
		Name: "Test Schedule",
	}
	_, _ = store.CreateSchedule(ctx, schedule)

	// Create an override
	now := time.Now()
	override := &routingv1.ScheduleOverride{
		UserId:    "user-1",
		StartTime: timestamppb.New(now),
		EndTime:   timestamppb.New(now.Add(8 * time.Hour)),
		Reason:    "Covering for vacation",
	}

	created, err := store.CreateOverride(ctx, "test-schedule", override)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if created.Id == "" {
		t.Error("expected override ID to be generated")
	}

	if created.UserId != "user-1" {
		t.Errorf("expected user_id 'user-1', got '%s'", created.UserId)
	}
}

func TestInMemoryStore_GetActiveOverrides(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create a schedule
	schedule := &routingv1.Schedule{
		Id:   "test-schedule",
		Name: "Test Schedule",
	}
	_, _ = store.CreateSchedule(ctx, schedule)

	// Create overrides
	now := time.Now()

	// Active override
	activeOverride := &routingv1.ScheduleOverride{
		Id:        "override-active",
		UserId:    "user-1",
		StartTime: timestamppb.New(now.Add(-1 * time.Hour)),
		EndTime:   timestamppb.New(now.Add(1 * time.Hour)),
	}
	_, _ = store.CreateOverride(ctx, "test-schedule", activeOverride)

	// Inactive override (in the past)
	pastOverride := &routingv1.ScheduleOverride{
		Id:        "override-past",
		UserId:    "user-2",
		StartTime: timestamppb.New(now.Add(-3 * time.Hour)),
		EndTime:   timestamppb.New(now.Add(-2 * time.Hour)),
	}
	_, _ = store.CreateOverride(ctx, "test-schedule", pastOverride)

	// Get active overrides
	active, err := store.GetActiveOverrides(ctx, "test-schedule", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(active) != 1 {
		t.Errorf("expected 1 active override, got %d", len(active))
	}

	if active[0].UserId != "user-1" {
		t.Errorf("expected active override for user-1, got '%s'", active[0].UserId)
	}
}
