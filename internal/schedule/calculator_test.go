package schedule

import (
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

func TestCalculator_GetOnCallAt_EmptySchedule(t *testing.T) {
	calc := NewCalculator()

	// Empty schedule
	result := calc.GetOnCallAt(nil, nil, time.Now())
	if result.PrimaryUserID != "" {
		t.Errorf("expected empty primary user, got '%s'", result.PrimaryUserID)
	}

	// Schedule with no rotations
	schedule := &routingv1.Schedule{
		Id:   "test-schedule",
		Name: "Test",
	}
	result = calc.GetOnCallAt(schedule, nil, time.Now())
	if result.PrimaryUserID != "" {
		t.Errorf("expected empty primary user for schedule with no rotations, got '%s'", result.PrimaryUserID)
	}
}

func TestCalculator_GetOnCallAt_SimpleRotation(t *testing.T) {
	calc := NewCalculator()

	// Create a schedule with a simple weekly rotation
	rotationStart := time.Now().Add(-7 * 24 * time.Hour) // Started a week ago

	schedule := &routingv1.Schedule{
		Id:       "test-schedule",
		Name:     "Test Schedule",
		Timezone: "UTC",
		Rotations: []*routingv1.Rotation{
			{
				Id:        "rotation-1",
				Name:      "Primary",
				Type:      routingv1.RotationType_ROTATION_TYPE_WEEKLY,
				Layer:     1,
				StartTime: timestamppb.New(rotationStart),
				ShiftConfig: &routingv1.ShiftConfig{
					ShiftLength: durationpb.New(7 * 24 * time.Hour),
				},
				Members: []*routingv1.RotationMember{
					{UserId: "user-1", Position: 0},
					{UserId: "user-2", Position: 1},
					{UserId: "user-3", Position: 2},
				},
			},
		},
	}

	result := calc.GetOnCallAt(schedule, nil, time.Now())

	if result.PrimaryUserID == "" {
		t.Error("expected a primary user to be on-call")
	}

	if result.CurrentShift == nil {
		t.Error("expected current shift to be set")
	}

	if result.NextHandoff.IsZero() {
		t.Error("expected next handoff time to be set")
	}
}

func TestCalculator_GetOnCallAt_DailyRotation(t *testing.T) {
	calc := NewCalculator()

	// Create a schedule with a daily rotation that started 3 days ago
	rotationStart := time.Now().Add(-3 * 24 * time.Hour)

	schedule := &routingv1.Schedule{
		Id:       "test-schedule",
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
	}

	// Test at rotation start (user-1)
	result := calc.GetOnCallAt(schedule, nil, rotationStart.Add(time.Hour))
	if result.PrimaryUserID != "user-1" {
		t.Errorf("expected user-1 at rotation start, got '%s'", result.PrimaryUserID)
	}

	// Test after 1 day (user-2)
	result = calc.GetOnCallAt(schedule, nil, rotationStart.Add(25*time.Hour))
	if result.PrimaryUserID != "user-2" {
		t.Errorf("expected user-2 after 1 day, got '%s'", result.PrimaryUserID)
	}

	// Test after 2 days (user-1 again)
	result = calc.GetOnCallAt(schedule, nil, rotationStart.Add(49*time.Hour))
	if result.PrimaryUserID != "user-1" {
		t.Errorf("expected user-1 after 2 days, got '%s'", result.PrimaryUserID)
	}
}

func TestCalculator_GetOnCallAt_OverridesPriority(t *testing.T) {
	calc := NewCalculator()

	rotationStart := time.Now().Add(-24 * time.Hour)

	schedule := &routingv1.Schedule{
		Id:       "test-schedule",
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
	}

	now := time.Now()

	// Create an override that takes precedence
	overrides := []*routingv1.ScheduleOverride{
		{
			Id:        "override-1",
			UserId:    "user-override",
			StartTime: timestamppb.New(now.Add(-1 * time.Hour)),
			EndTime:   timestamppb.New(now.Add(1 * time.Hour)),
		},
	}

	result := calc.GetOnCallAt(schedule, overrides, now)

	if result.PrimaryUserID != "user-override" {
		t.Errorf("expected override user 'user-override', got '%s'", result.PrimaryUserID)
	}

	if result.CurrentShift.Type != routingv1.ShiftType_SHIFT_TYPE_OVERRIDE {
		t.Errorf("expected shift type OVERRIDE, got %v", result.CurrentShift.Type)
	}
}

func TestCalculator_GetOnCallAt_MultipleRotationLayers(t *testing.T) {
	calc := NewCalculator()

	rotationStart := time.Now().Add(-24 * time.Hour)

	schedule := &routingv1.Schedule{
		Id:       "test-schedule",
		Name:     "Test Schedule",
		Timezone: "UTC",
		Rotations: []*routingv1.Rotation{
			{
				Id:        "rotation-primary",
				Name:      "Primary",
				Type:      routingv1.RotationType_ROTATION_TYPE_DAILY,
				Layer:     2, // Higher layer = higher priority
				StartTime: timestamppb.New(rotationStart),
				ShiftConfig: &routingv1.ShiftConfig{
					ShiftLength: durationpb.New(24 * time.Hour),
				},
				Members: []*routingv1.RotationMember{
					{UserId: "primary-user", Position: 0},
				},
			},
			{
				Id:        "rotation-secondary",
				Name:      "Secondary",
				Type:      routingv1.RotationType_ROTATION_TYPE_DAILY,
				Layer:     1, // Lower layer = backup
				StartTime: timestamppb.New(rotationStart),
				ShiftConfig: &routingv1.ShiftConfig{
					ShiftLength: durationpb.New(24 * time.Hour),
				},
				Members: []*routingv1.RotationMember{
					{UserId: "secondary-user", Position: 0},
				},
			},
		},
	}

	result := calc.GetOnCallAt(schedule, nil, time.Now())

	if result.PrimaryUserID != "primary-user" {
		t.Errorf("expected primary user from higher layer rotation, got '%s'", result.PrimaryUserID)
	}

	if result.SecondaryUserID != "secondary-user" {
		t.Errorf("expected secondary user from lower layer rotation, got '%s'", result.SecondaryUserID)
	}
}

func TestCalculator_GetOnCallAt_TimeRestrictions(t *testing.T) {
	calc := NewCalculator()

	// Use a specific time for testing - Monday 10:00 UTC
	loc, _ := time.LoadLocation("UTC")
	businessHoursTime := time.Date(2024, 1, 15, 10, 0, 0, 0, loc) // Monday 10:00

	// Rotation started before the test time
	rotationStart := time.Date(2024, 1, 1, 0, 0, 0, 0, loc)

	// Create a rotation that's only active on weekdays 9-17
	schedule := &routingv1.Schedule{
		Id:       "test-schedule",
		Name:     "Business Hours Schedule",
		Timezone: "UTC",
		Rotations: []*routingv1.Rotation{
			{
				Id:        "rotation-1",
				Name:      "Business Hours",
				Type:      routingv1.RotationType_ROTATION_TYPE_WEEKLY,
				Layer:     1,
				StartTime: timestamppb.New(rotationStart),
				ShiftConfig: &routingv1.ShiftConfig{
					ShiftLength: durationpb.New(7 * 24 * time.Hour),
				},
				Members: []*routingv1.RotationMember{
					{UserId: "business-user", Position: 0},
				},
				Restrictions: []*routingv1.TimeWindow{
					{
						DaysOfWeek: []int32{1, 2, 3, 4, 5}, // Monday-Friday
						StartTime:  "09:00",
						EndTime:    "17:00",
					},
				},
			},
		},
	}

	// Test during business hours on a weekday
	result := calc.GetOnCallAt(schedule, nil, businessHoursTime)
	if result.PrimaryUserID != "business-user" {
		t.Logf("Business hours time: %v (weekday: %d)", businessHoursTime, businessHoursTime.Weekday())
		t.Errorf("expected business-user during business hours, got '%s'", result.PrimaryUserID)
	}
}

func TestCalculator_ListUpcomingShifts(t *testing.T) {
	calc := NewCalculator()

	rotationStart := time.Now().Truncate(24 * time.Hour) // Start at midnight today

	schedule := &routingv1.Schedule{
		Id:       "test-schedule",
		Name:     "Test Schedule",
		Timezone: "UTC",
		Rotations: []*routingv1.Rotation{
			{
				Id:        "rotation-1",
				Name:      "Daily Rotation",
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
	}

	from := rotationStart
	until := from.Add(7 * 24 * time.Hour) // Get shifts for the next week

	shifts := calc.ListUpcomingShifts(schedule, nil, from, until, "")

	if len(shifts) < 7 {
		t.Errorf("expected at least 7 shifts for a week, got %d", len(shifts))
	}

	// Verify shifts alternate between users
	for i, shift := range shifts {
		expectedUser := "user-1"
		if i%2 == 1 {
			expectedUser = "user-2"
		}
		if shift.UserId != expectedUser {
			t.Errorf("shift %d: expected %s, got %s", i, expectedUser, shift.UserId)
		}
	}
}

func TestCalculator_ListUpcomingShifts_FilterByUser(t *testing.T) {
	calc := NewCalculator()

	rotationStart := time.Now().Truncate(24 * time.Hour)

	schedule := &routingv1.Schedule{
		Id:       "test-schedule",
		Name:     "Test Schedule",
		Timezone: "UTC",
		Rotations: []*routingv1.Rotation{
			{
				Id:        "rotation-1",
				Name:      "Daily Rotation",
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
	}

	from := rotationStart
	until := from.Add(10 * 24 * time.Hour)

	// Filter by user-1
	shifts := calc.ListUpcomingShifts(schedule, nil, from, until, "user-1")

	for _, shift := range shifts {
		if shift.UserId != "user-1" {
			t.Errorf("expected all shifts for user-1, got shift for '%s'", shift.UserId)
		}
	}
}

func TestCalculator_ListUpcomingShifts_WithOverrides(t *testing.T) {
	calc := NewCalculator()

	rotationStart := time.Now().Truncate(24 * time.Hour)

	schedule := &routingv1.Schedule{
		Id:       "test-schedule",
		Name:     "Test Schedule",
		Timezone: "UTC",
		Rotations: []*routingv1.Rotation{
			{
				Id:        "rotation-1",
				Name:      "Daily Rotation",
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
	}

	from := rotationStart
	until := from.Add(3 * 24 * time.Hour)

	// Add an override
	overrides := []*routingv1.ScheduleOverride{
		{
			Id:        "override-1",
			UserId:    "override-user",
			StartTime: timestamppb.New(from.Add(12 * time.Hour)),
			EndTime:   timestamppb.New(from.Add(36 * time.Hour)),
		},
	}

	shifts := calc.ListUpcomingShifts(schedule, overrides, from, until, "")

	// Should include override shift
	hasOverrideShift := false
	for _, shift := range shifts {
		if shift.UserId == "override-user" && shift.Type == routingv1.ShiftType_SHIFT_TYPE_OVERRIDE {
			hasOverrideShift = true
			break
		}
	}

	if !hasOverrideShift {
		t.Error("expected override shift to be included in upcoming shifts")
	}
}

func TestCalculator_CalculateNextHandoff(t *testing.T) {
	calc := NewCalculator()

	now := time.Now()
	rotationStart := now.Add(-12 * time.Hour) // Started 12 hours ago

	schedule := &routingv1.Schedule{
		Id:       "test-schedule",
		Name:     "Test Schedule",
		Timezone: "UTC",
		Rotations: []*routingv1.Rotation{
			{
				Id:        "rotation-1",
				Name:      "Daily Rotation",
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
	}

	nextHandoff := calc.CalculateNextHandoff(schedule, nil, now)

	if nextHandoff.IsZero() {
		t.Error("expected next handoff time to be calculated")
	}

	// Next handoff should be 12 hours from now (24h shift started 12h ago)
	expectedHandoff := rotationStart.Add(24 * time.Hour)
	diff := nextHandoff.Sub(expectedHandoff)
	if diff < -time.Minute || diff > time.Minute {
		t.Errorf("expected next handoff around %v, got %v (diff: %v)", expectedHandoff, nextHandoff, diff)
	}
}

func TestCalculator_CalculateNextHandoff_WithOverride(t *testing.T) {
	calc := NewCalculator()

	now := time.Now()
	rotationStart := now.Add(-12 * time.Hour)

	schedule := &routingv1.Schedule{
		Id:       "test-schedule",
		Name:     "Test Schedule",
		Timezone: "UTC",
		Rotations: []*routingv1.Rotation{
			{
				Id:        "rotation-1",
				Name:      "Daily Rotation",
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
	}

	// Override ends in 2 hours
	overrideEnd := now.Add(2 * time.Hour)
	overrides := []*routingv1.ScheduleOverride{
		{
			Id:        "override-1",
			UserId:    "override-user",
			StartTime: timestamppb.New(now.Add(-1 * time.Hour)),
			EndTime:   timestamppb.New(overrideEnd),
		},
	}

	nextHandoff := calc.CalculateNextHandoff(schedule, overrides, now)

	if nextHandoff.IsZero() {
		t.Error("expected next handoff time to be calculated")
	}

	// Next handoff should be when the override ends
	diff := nextHandoff.Sub(overrideEnd)
	if diff < -time.Minute || diff > time.Minute {
		t.Errorf("expected next handoff at override end %v, got %v (diff: %v)", overrideEnd, nextHandoff, diff)
	}
}

func TestCalculator_GetShiftDuration(t *testing.T) {
	calc := NewCalculator()

	tests := []struct {
		name     string
		rotation *routingv1.Rotation
		expected time.Duration
	}{
		{
			name: "Daily rotation default",
			rotation: &routingv1.Rotation{
				Type: routingv1.RotationType_ROTATION_TYPE_DAILY,
			},
			expected: 24 * time.Hour,
		},
		{
			name: "Weekly rotation default",
			rotation: &routingv1.Rotation{
				Type: routingv1.RotationType_ROTATION_TYPE_WEEKLY,
			},
			expected: 7 * 24 * time.Hour,
		},
		{
			name: "Biweekly rotation default",
			rotation: &routingv1.Rotation{
				Type: routingv1.RotationType_ROTATION_TYPE_BIWEEKLY,
			},
			expected: 14 * 24 * time.Hour,
		},
		{
			name: "Custom duration",
			rotation: &routingv1.Rotation{
				Type: routingv1.RotationType_ROTATION_TYPE_CUSTOM,
				ShiftConfig: &routingv1.ShiftConfig{
					ShiftLength: durationpb.New(12 * time.Hour),
				},
			},
			expected: 12 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration := calc.getShiftDuration(tt.rotation)
			if duration != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, duration)
			}
		})
	}
}

func TestCalculator_IsOverrideActive(t *testing.T) {
	calc := NewCalculator()

	now := time.Now()

	tests := []struct {
		name     string
		override *routingv1.ScheduleOverride
		at       time.Time
		expected bool
	}{
		{
			name:     "nil override",
			override: nil,
			at:       now,
			expected: false,
		},
		{
			name: "active override",
			override: &routingv1.ScheduleOverride{
				StartTime: timestamppb.New(now.Add(-1 * time.Hour)),
				EndTime:   timestamppb.New(now.Add(1 * time.Hour)),
			},
			at:       now,
			expected: true,
		},
		{
			name: "past override",
			override: &routingv1.ScheduleOverride{
				StartTime: timestamppb.New(now.Add(-3 * time.Hour)),
				EndTime:   timestamppb.New(now.Add(-1 * time.Hour)),
			},
			at:       now,
			expected: false,
		},
		{
			name: "future override",
			override: &routingv1.ScheduleOverride{
				StartTime: timestamppb.New(now.Add(1 * time.Hour)),
				EndTime:   timestamppb.New(now.Add(3 * time.Hour)),
			},
			at:       now,
			expected: false,
		},
		{
			name: "at start boundary",
			override: &routingv1.ScheduleOverride{
				StartTime: timestamppb.New(now),
				EndTime:   timestamppb.New(now.Add(1 * time.Hour)),
			},
			at:       now,
			expected: true,
		},
		{
			name: "at end boundary",
			override: &routingv1.ScheduleOverride{
				StartTime: timestamppb.New(now.Add(-1 * time.Hour)),
				EndTime:   timestamppb.New(now),
			},
			at:       now,
			expected: false, // End time is exclusive
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.isOverrideActive(tt.override, tt.at)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
