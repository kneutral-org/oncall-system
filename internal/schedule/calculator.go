// Package schedule provides the on-call scheduling functionality.
package schedule

import (
	"sort"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// OnCallResult represents the result of an on-call calculation.
type OnCallResult struct {
	PrimaryUserID   string
	SecondaryUserID string
	CurrentShift    *routingv1.Shift
	NextHandoff     time.Time
}

// Calculator calculates who is on-call based on schedule, rotations, and overrides.
type Calculator struct {
	// timezone for schedule calculations
	defaultTimezone *time.Location
}

// NewCalculator creates a new on-call calculator.
func NewCalculator() *Calculator {
	return &Calculator{
		defaultTimezone: time.UTC,
	}
}

// GetOnCallAt calculates who is on-call at a specific time for a schedule.
func (c *Calculator) GetOnCallAt(schedule *routingv1.Schedule, overrides []*routingv1.ScheduleOverride, at time.Time) *OnCallResult {
	if schedule == nil || len(schedule.Rotations) == 0 {
		return &OnCallResult{}
	}

	// Load timezone
	loc := c.loadTimezone(schedule.Timezone)
	localTime := at.In(loc)

	// First check for active overrides - they take highest priority
	for _, override := range overrides {
		if c.isOverrideActive(override, at) {
			shift := c.createOverrideShift(schedule.Id, override)
			return &OnCallResult{
				PrimaryUserID:   override.UserId,
				SecondaryUserID: "",
				CurrentShift:    shift,
				NextHandoff:     override.EndTime.AsTime(),
			}
		}
	}

	// Sort rotations by layer (higher layer = higher priority)
	sortedRotations := make([]*routingv1.Rotation, len(schedule.Rotations))
	copy(sortedRotations, schedule.Rotations)
	sort.Slice(sortedRotations, func(i, j int) bool {
		return sortedRotations[i].Layer > sortedRotations[j].Layer
	})

	var primaryUserID, secondaryUserID string
	var currentShift *routingv1.Shift
	var nextHandoff time.Time

	// Evaluate each rotation to find on-call users
	for i, rotation := range sortedRotations {
		if len(rotation.Members) == 0 {
			continue
		}

		// Check if rotation is active (time restrictions)
		if !c.isRotationActive(rotation, localTime) {
			continue
		}

		// Calculate who is on-call for this rotation
		userID, shift, handoff := c.calculateRotationOnCall(schedule.Id, rotation, at, loc)

		if userID != "" {
			if i == 0 || primaryUserID == "" {
				// First rotation or first valid user becomes primary
				primaryUserID = userID
				currentShift = shift
				nextHandoff = handoff
			} else if secondaryUserID == "" {
				// Second rotation becomes secondary
				secondaryUserID = userID
			}
		}

		// We have both primary and secondary
		if primaryUserID != "" && secondaryUserID != "" {
			break
		}
	}

	return &OnCallResult{
		PrimaryUserID:   primaryUserID,
		SecondaryUserID: secondaryUserID,
		CurrentShift:    currentShift,
		NextHandoff:     nextHandoff,
	}
}

// ListUpcomingShifts generates upcoming shifts for a schedule.
func (c *Calculator) ListUpcomingShifts(schedule *routingv1.Schedule, overrides []*routingv1.ScheduleOverride, from, until time.Time, filterUserID string) []*routingv1.Shift {
	if schedule == nil || len(schedule.Rotations) == 0 {
		return nil
	}

	loc := c.loadTimezone(schedule.Timezone)
	var shifts []*routingv1.Shift

	// Generate shifts from overrides first
	for _, override := range overrides {
		startTime := override.StartTime.AsTime()
		endTime := override.EndTime.AsTime()

		// Check if override falls within our time range
		if endTime.Before(from) || startTime.After(until) {
			continue
		}

		// Filter by user if specified
		if filterUserID != "" && override.UserId != filterUserID {
			continue
		}

		shift := c.createOverrideShift(schedule.Id, override)
		shifts = append(shifts, shift)
	}

	// Generate regular rotation shifts
	for _, rotation := range schedule.Rotations {
		if len(rotation.Members) == 0 {
			continue
		}

		rotationShifts := c.generateRotationShifts(schedule.Id, rotation, from, until, loc, filterUserID)
		shifts = append(shifts, rotationShifts...)
	}

	// Sort shifts by start time
	sort.Slice(shifts, func(i, j int) bool {
		return shifts[i].StartTime.AsTime().Before(shifts[j].StartTime.AsTime())
	})

	return shifts
}

// calculateRotationOnCall calculates who is on-call for a specific rotation at a given time.
func (c *Calculator) calculateRotationOnCall(scheduleID string, rotation *routingv1.Rotation, at time.Time, loc *time.Location) (string, *routingv1.Shift, time.Time) {
	if len(rotation.Members) == 0 {
		return "", nil, time.Time{}
	}

	// Determine shift duration based on rotation type
	shiftDuration := c.getShiftDuration(rotation)

	// Calculate rotation start time
	rotationStart := rotation.StartTime.AsTime()
	if at.Before(rotationStart) {
		return "", nil, time.Time{}
	}

	// Calculate how many complete shifts have passed since rotation start
	elapsed := at.Sub(rotationStart)
	shiftIndex := int(elapsed / shiftDuration)

	// Calculate which member is on-call (round-robin)
	memberIndex := shiftIndex % len(rotation.Members)

	// Find the member at this position
	var onCallMember *routingv1.RotationMember
	for _, member := range rotation.Members {
		if int(member.Position) == memberIndex {
			onCallMember = member
			break
		}
	}

	// If no member found at exact position, use modulo of members
	if onCallMember == nil {
		onCallMember = rotation.Members[memberIndex%len(rotation.Members)]
	}

	// Calculate shift boundaries
	shiftStart := rotationStart.Add(time.Duration(shiftIndex) * shiftDuration)
	shiftEnd := shiftStart.Add(shiftDuration)

	shift := &routingv1.Shift{
		Id:         uuid.New().String(),
		ScheduleId: scheduleID,
		RotationId: rotation.Id,
		UserId:     onCallMember.UserId,
		StartTime:  timestamppb.New(shiftStart),
		EndTime:    timestamppb.New(shiftEnd),
		Type:       routingv1.ShiftType_SHIFT_TYPE_REGULAR,
		OncallLevel: 1,
	}

	return onCallMember.UserId, shift, shiftEnd
}

// generateRotationShifts generates shifts for a rotation within a time range.
func (c *Calculator) generateRotationShifts(scheduleID string, rotation *routingv1.Rotation, from, until time.Time, loc *time.Location, filterUserID string) []*routingv1.Shift {
	if len(rotation.Members) == 0 {
		return nil
	}

	shiftDuration := c.getShiftDuration(rotation)
	rotationStart := rotation.StartTime.AsTime()

	var shifts []*routingv1.Shift

	// Start from rotation start or 'from' time, whichever is later
	startTime := rotationStart
	if from.After(rotationStart) {
		// Align to shift boundaries
		elapsed := from.Sub(rotationStart)
		shiftsElapsed := int(elapsed / shiftDuration)
		startTime = rotationStart.Add(time.Duration(shiftsElapsed) * shiftDuration)
	}

	// Generate shifts until 'until' time
	currentTime := startTime
	for currentTime.Before(until) {
		shiftEnd := currentTime.Add(shiftDuration)

		// Calculate member index
		elapsed := currentTime.Sub(rotationStart)
		shiftIndex := int(elapsed / shiftDuration)
		memberIndex := shiftIndex % len(rotation.Members)

		// Find member
		var member *routingv1.RotationMember
		for _, m := range rotation.Members {
			if int(m.Position) == memberIndex {
				member = m
				break
			}
		}
		if member == nil {
			member = rotation.Members[memberIndex%len(rotation.Members)]
		}

		// Filter by user if specified
		if filterUserID != "" && member.UserId != filterUserID {
			currentTime = shiftEnd
			continue
		}

		// Check time restrictions
		localTime := currentTime.In(loc)
		if c.isRotationActive(rotation, localTime) {
			shift := &routingv1.Shift{
				Id:          uuid.New().String(),
				ScheduleId:  scheduleID,
				RotationId:  rotation.Id,
				UserId:      member.UserId,
				StartTime:   timestamppb.New(currentTime),
				EndTime:     timestamppb.New(shiftEnd),
				Type:        routingv1.ShiftType_SHIFT_TYPE_REGULAR,
				OncallLevel: 1,
			}
			shifts = append(shifts, shift)
		}

		currentTime = shiftEnd
	}

	return shifts
}

// getShiftDuration returns the duration of a shift based on rotation type.
func (c *Calculator) getShiftDuration(rotation *routingv1.Rotation) time.Duration {
	// If shift config has explicit duration, use it
	if rotation.ShiftConfig != nil && rotation.ShiftConfig.ShiftLength != nil {
		return rotation.ShiftConfig.ShiftLength.AsDuration()
	}

	// Default durations based on rotation type
	switch rotation.Type {
	case routingv1.RotationType_ROTATION_TYPE_DAILY:
		return 24 * time.Hour
	case routingv1.RotationType_ROTATION_TYPE_WEEKLY:
		return 7 * 24 * time.Hour
	case routingv1.RotationType_ROTATION_TYPE_BIWEEKLY:
		return 14 * 24 * time.Hour
	case routingv1.RotationType_ROTATION_TYPE_CUSTOM:
		// Default to daily if custom but no explicit duration
		return 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}

// isRotationActive checks if a rotation is active at a given local time based on time restrictions.
func (c *Calculator) isRotationActive(rotation *routingv1.Rotation, localTime time.Time) bool {
	if len(rotation.Restrictions) == 0 {
		return true // No restrictions means always active
	}

	for _, restriction := range rotation.Restrictions {
		if c.isTimeInWindow(localTime, restriction) {
			return !restriction.Invert // If in window and not inverted, active
		}
	}

	// If no restrictions matched and restrictions exist, check inversion
	if len(rotation.Restrictions) > 0 && rotation.Restrictions[0].Invert {
		return true // Inverted restriction means active when NOT in window
	}

	return false
}

// isTimeInWindow checks if a time falls within a time window.
func (c *Calculator) isTimeInWindow(t time.Time, window *routingv1.TimeWindow) bool {
	// Check day of week
	if len(window.DaysOfWeek) > 0 {
		currentDay := int32(t.Weekday())
		dayMatch := false
		for _, day := range window.DaysOfWeek {
			if day == currentDay {
				dayMatch = true
				break
			}
		}
		if !dayMatch {
			return false
		}
	}

	// Check time range
	if window.StartTime != "" && window.EndTime != "" {
		currentTimeStr := t.Format("15:04")
		if currentTimeStr < window.StartTime || currentTimeStr >= window.EndTime {
			return false
		}
	}

	return true
}

// isOverrideActive checks if an override is active at a given time.
func (c *Calculator) isOverrideActive(override *routingv1.ScheduleOverride, at time.Time) bool {
	if override == nil || override.StartTime == nil || override.EndTime == nil {
		return false
	}

	startTime := override.StartTime.AsTime()
	endTime := override.EndTime.AsTime()

	return !at.Before(startTime) && at.Before(endTime)
}

// createOverrideShift creates a shift from an override.
func (c *Calculator) createOverrideShift(scheduleID string, override *routingv1.ScheduleOverride) *routingv1.Shift {
	return &routingv1.Shift{
		Id:          uuid.New().String(),
		ScheduleId:  scheduleID,
		RotationId:  "", // Overrides don't belong to a rotation
		UserId:      override.UserId,
		StartTime:   override.StartTime,
		EndTime:     override.EndTime,
		Type:        routingv1.ShiftType_SHIFT_TYPE_OVERRIDE,
		OncallLevel: 1,
	}
}

// loadTimezone loads a timezone by name, defaulting to UTC if invalid.
func (c *Calculator) loadTimezone(tzName string) *time.Location {
	if tzName == "" {
		return time.UTC
	}

	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return time.UTC
	}

	return loc
}

// CalculateNextHandoff calculates when the next handoff will occur for a schedule.
func (c *Calculator) CalculateNextHandoff(schedule *routingv1.Schedule, overrides []*routingv1.ScheduleOverride, from time.Time) time.Time {
	if schedule == nil || len(schedule.Rotations) == 0 {
		return time.Time{}
	}

	// Check for active overrides first
	for _, override := range overrides {
		if c.isOverrideActive(override, from) {
			return override.EndTime.AsTime()
		}
	}

	// Find the earliest next handoff from all rotations
	var nextHandoff time.Time

	for _, rotation := range schedule.Rotations {
		if len(rotation.Members) == 0 {
			continue
		}

		shiftDuration := c.getShiftDuration(rotation)
		rotationStart := rotation.StartTime.AsTime()

		if from.Before(rotationStart) {
			if nextHandoff.IsZero() || rotationStart.Before(nextHandoff) {
				nextHandoff = rotationStart
			}
			continue
		}

		// Calculate next handoff time
		elapsed := from.Sub(rotationStart)
		shiftsElapsed := int(elapsed / shiftDuration)
		currentShiftEnd := rotationStart.Add(time.Duration(shiftsElapsed+1) * shiftDuration)

		if nextHandoff.IsZero() || currentShiftEnd.Before(nextHandoff) {
			nextHandoff = currentShiftEnd
		}
	}

	return nextHandoff
}
