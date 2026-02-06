// Package schedule provides the on-call scheduling functionality.
package schedule

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

var (
	// ErrNotFound is returned when a schedule or related entity is not found.
	ErrNotFound = errors.New("not found")
	// ErrInvalidSchedule is returned when a schedule is invalid.
	ErrInvalidSchedule = errors.New("invalid schedule")
	// ErrInvalidRotation is returned when a rotation is invalid.
	ErrInvalidRotation = errors.New("invalid rotation")
	// ErrInvalidOverride is returned when an override is invalid.
	ErrInvalidOverride = errors.New("invalid override")
)

// Store defines the interface for schedule persistence.
type Store interface {
	// Schedule CRUD
	CreateSchedule(ctx context.Context, schedule *routingv1.Schedule) (*routingv1.Schedule, error)
	GetSchedule(ctx context.Context, id string) (*routingv1.Schedule, error)
	ListSchedules(ctx context.Context, req *routingv1.ListSchedulesRequest) (*routingv1.ListSchedulesResponse, error)
	UpdateSchedule(ctx context.Context, schedule *routingv1.Schedule) (*routingv1.Schedule, error)
	DeleteSchedule(ctx context.Context, id string) error

	// Rotation management
	AddRotation(ctx context.Context, scheduleID string, rotation *routingv1.Rotation) (*routingv1.Schedule, error)
	UpdateRotation(ctx context.Context, scheduleID string, rotation *routingv1.Rotation) (*routingv1.Schedule, error)
	RemoveRotation(ctx context.Context, scheduleID, rotationID string) (*routingv1.Schedule, error)

	// Override management
	CreateOverride(ctx context.Context, scheduleID string, override *routingv1.ScheduleOverride) (*routingv1.ScheduleOverride, error)
	DeleteOverride(ctx context.Context, scheduleID, overrideID string) error
	ListOverrides(ctx context.Context, scheduleID string, startTime, endTime *timestamppb.Timestamp, pageSize int, pageToken string) (*routingv1.ListOverridesResponse, error)
	GetActiveOverrides(ctx context.Context, scheduleID string, at time.Time) ([]*routingv1.ScheduleOverride, error)

	// Handoff
	RecordHandoffAck(ctx context.Context, scheduleID, userID string) error
}

// PostgresStore implements Store using PostgreSQL.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore creates a new PostgresStore.
func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

// CreateSchedule creates a new schedule in the database.
func (s *PostgresStore) CreateSchedule(ctx context.Context, schedule *routingv1.Schedule) (*routingv1.Schedule, error) {
	if schedule == nil {
		return nil, ErrInvalidSchedule
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Generate ID if not provided
	if schedule.Id == "" {
		schedule.Id = uuid.New().String()
	}

	now := time.Now()
	schedule.CreatedAt = timestamppb.New(now)
	schedule.UpdatedAt = timestamppb.New(now)

	// Default timezone
	if schedule.Timezone == "" {
		schedule.Timezone = "UTC"
	}

	// Insert the schedule
	var teamID *string
	if schedule.TeamId != "" {
		teamID = &schedule.TeamId
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO schedules (id, name, description, timezone, team_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, schedule.Id, schedule.Name, schedule.Description, schedule.Timezone, teamID, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert schedule: %w", err)
	}

	// Insert rotations
	for _, rotation := range schedule.Rotations {
		if err := s.insertRotation(ctx, tx, schedule.Id, rotation); err != nil {
			return nil, fmt.Errorf("insert rotation: %w", err)
		}
	}

	// Insert overrides
	for _, override := range schedule.Overrides {
		if err := s.insertOverride(ctx, tx, schedule.Id, override); err != nil {
			return nil, fmt.Errorf("insert override: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return schedule, nil
}

// insertRotation inserts a rotation and its members into the database.
func (s *PostgresStore) insertRotation(ctx context.Context, tx *sql.Tx, scheduleID string, rotation *routingv1.Rotation) error {
	if rotation.Id == "" {
		rotation.Id = uuid.New().String()
	}

	// Convert shift config to hours
	var shiftLengthHours *int
	var handoffTime *string
	var handoffDay *int

	if rotation.ShiftConfig != nil {
		if rotation.ShiftConfig.ShiftLength != nil {
			hours := int(rotation.ShiftConfig.ShiftLength.AsDuration().Hours())
			shiftLengthHours = &hours
		}
		if rotation.ShiftConfig.HandoffTime != "" {
			handoffTime = &rotation.ShiftConfig.HandoffTime
		}
		if len(rotation.ShiftConfig.HandoffDays) > 0 {
			day := int(rotation.ShiftConfig.HandoffDays[0])
			handoffDay = &day
		}
	}

	// Convert restrictions to time restriction fields
	var restrictionStart, restrictionEnd *string
	var restrictionDays []int32

	if len(rotation.Restrictions) > 0 {
		restriction := rotation.Restrictions[0]
		if restriction.StartTime != "" {
			restrictionStart = &restriction.StartTime
		}
		if restriction.EndTime != "" {
			restrictionEnd = &restriction.EndTime
		}
		restrictionDays = restriction.DaysOfWeek
	}

	var startTime time.Time
	if rotation.StartTime != nil {
		startTime = rotation.StartTime.AsTime()
	} else {
		startTime = time.Now()
	}

	_, err := tx.ExecContext(ctx, `
		INSERT INTO rotations (id, schedule_id, name, priority, rotation_type, start_time,
			shift_length_hours, handoff_time, handoff_day, time_restriction_start,
			time_restriction_end, time_restriction_days, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, rotation.Id, scheduleID, rotation.Name, rotation.Layer, rotation.Type.String(),
		startTime, shiftLengthHours, handoffTime, handoffDay, restrictionStart, restrictionEnd,
		intSliceToArray(restrictionDays), time.Now())
	if err != nil {
		return fmt.Errorf("insert rotation: %w", err)
	}

	// Insert rotation members
	for _, member := range rotation.Members {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO rotation_members (rotation_id, user_id, position)
			VALUES ($1, $2, $3)
		`, rotation.Id, member.UserId, member.Position)
		if err != nil {
			return fmt.Errorf("insert rotation member: %w", err)
		}
	}

	return nil
}

// insertOverride inserts a schedule override into the database.
func (s *PostgresStore) insertOverride(ctx context.Context, tx *sql.Tx, scheduleID string, override *routingv1.ScheduleOverride) error {
	if override.Id == "" {
		override.Id = uuid.New().String()
	}

	var startTime, endTime time.Time
	if override.StartTime != nil {
		startTime = override.StartTime.AsTime()
	}
	if override.EndTime != nil {
		endTime = override.EndTime.AsTime()
	}

	_, err := tx.ExecContext(ctx, `
		INSERT INTO schedule_overrides (id, schedule_id, user_id, start_time, end_time, reason, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, override.Id, scheduleID, override.UserId, startTime, endTime, override.Reason, override.CreatedBy, time.Now())
	if err != nil {
		return fmt.Errorf("insert override: %w", err)
	}

	return nil
}

// GetSchedule retrieves a schedule by ID with all related data.
func (s *PostgresStore) GetSchedule(ctx context.Context, id string) (*routingv1.Schedule, error) {
	schedule := &routingv1.Schedule{}

	var createdAt, updatedAt time.Time
	var description sql.NullString
	var teamID sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, timezone, team_id, created_at, updated_at
		FROM schedules WHERE id = $1
	`, id).Scan(&schedule.Id, &schedule.Name, &description, &schedule.Timezone, &teamID, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query schedule: %w", err)
	}

	schedule.Description = description.String
	schedule.TeamId = teamID.String
	schedule.CreatedAt = timestamppb.New(createdAt)
	schedule.UpdatedAt = timestamppb.New(updatedAt)

	// Load rotations
	rotations, err := s.loadRotations(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load rotations: %w", err)
	}
	schedule.Rotations = rotations

	// Load overrides
	overrides, err := s.loadOverrides(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load overrides: %w", err)
	}
	schedule.Overrides = overrides

	return schedule, nil
}

// loadRotations loads all rotations for a schedule.
func (s *PostgresStore) loadRotations(ctx context.Context, scheduleID string) ([]*routingv1.Rotation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, priority, rotation_type, start_time, shift_length_hours,
			handoff_time, handoff_day, time_restriction_start, time_restriction_end, time_restriction_days
		FROM rotations WHERE schedule_id = $1 ORDER BY priority DESC
	`, scheduleID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var rotations []*routingv1.Rotation
	for rows.Next() {
		rotation := &routingv1.Rotation{}
		var name sql.NullString
		var startTime time.Time
		var shiftLengthHours sql.NullInt32
		var handoffTime sql.NullString
		var handoffDay sql.NullInt32
		var restrictionStart, restrictionEnd sql.NullString
		var restrictionDays []byte
		var rotationType string

		if err := rows.Scan(&rotation.Id, &name, &rotation.Layer, &rotationType, &startTime,
			&shiftLengthHours, &handoffTime, &handoffDay, &restrictionStart, &restrictionEnd, &restrictionDays); err != nil {
			return nil, err
		}

		rotation.Name = name.String
		rotation.Type = parseRotationType(rotationType)
		rotation.StartTime = timestamppb.New(startTime)

		// Build shift config
		rotation.ShiftConfig = &routingv1.ShiftConfig{}
		if shiftLengthHours.Valid {
			rotation.ShiftConfig.ShiftLength = durationpb.New(time.Duration(shiftLengthHours.Int32) * time.Hour)
		}
		if handoffTime.Valid {
			rotation.ShiftConfig.HandoffTime = handoffTime.String
		}
		if handoffDay.Valid {
			rotation.ShiftConfig.HandoffDays = []int32{handoffDay.Int32}
		}

		// Build restrictions
		if restrictionStart.Valid || restrictionEnd.Valid {
			restriction := &routingv1.TimeWindow{
				StartTime: restrictionStart.String,
				EndTime:   restrictionEnd.String,
			}
			if restrictionDays != nil {
				var days []int32
				if err := json.Unmarshal(restrictionDays, &days); err == nil {
					restriction.DaysOfWeek = days
				}
			}
			rotation.Restrictions = []*routingv1.TimeWindow{restriction}
		}

		// Load rotation members
		members, err := s.loadRotationMembers(ctx, rotation.Id)
		if err != nil {
			return nil, err
		}
		rotation.Members = members

		rotations = append(rotations, rotation)
	}

	return rotations, rows.Err()
}

// loadRotationMembers loads members for a rotation.
func (s *PostgresStore) loadRotationMembers(ctx context.Context, rotationID string) ([]*routingv1.RotationMember, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT user_id, position FROM rotation_members WHERE rotation_id = $1 ORDER BY position
	`, rotationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var members []*routingv1.RotationMember
	for rows.Next() {
		member := &routingv1.RotationMember{}
		if err := rows.Scan(&member.UserId, &member.Position); err != nil {
			return nil, err
		}
		members = append(members, member)
	}

	return members, rows.Err()
}

// loadOverrides loads all overrides for a schedule.
func (s *PostgresStore) loadOverrides(ctx context.Context, scheduleID string) ([]*routingv1.ScheduleOverride, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, start_time, end_time, reason, created_by, created_at
		FROM schedule_overrides WHERE schedule_id = $1 ORDER BY start_time
	`, scheduleID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var overrides []*routingv1.ScheduleOverride
	for rows.Next() {
		override := &routingv1.ScheduleOverride{}
		var startTime, endTime, createdAt time.Time
		var reason, createdBy sql.NullString

		if err := rows.Scan(&override.Id, &override.UserId, &startTime, &endTime, &reason, &createdBy, &createdAt); err != nil {
			return nil, err
		}

		override.StartTime = timestamppb.New(startTime)
		override.EndTime = timestamppb.New(endTime)
		override.Reason = reason.String
		override.CreatedBy = createdBy.String
		override.CreatedAt = timestamppb.New(createdAt)

		overrides = append(overrides, override)
	}

	return overrides, rows.Err()
}

// ListSchedules retrieves schedules with optional filters.
func (s *PostgresStore) ListSchedules(ctx context.Context, req *routingv1.ListSchedulesRequest) (*routingv1.ListSchedulesResponse, error) {
	query := `SELECT id, name, description, timezone, team_id, created_at, updated_at FROM schedules WHERE 1=1`
	args := []interface{}{}
	argIndex := 1

	if req.TeamId != "" {
		query += fmt.Sprintf(" AND team_id = $%d", argIndex)
		args = append(args, req.TeamId)
		argIndex++
	}

	query += " ORDER BY name ASC"

	pageSize := int(req.PageSize)
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}
	query += fmt.Sprintf(" LIMIT $%d", argIndex)
	args = append(args, pageSize+1)
	argIndex++

	if req.PageToken != "" {
		offset := decodePageToken(req.PageToken)
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query schedules: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var schedules []*routingv1.Schedule
	for rows.Next() {
		schedule := &routingv1.Schedule{}
		var createdAt, updatedAt time.Time
		var description, teamID sql.NullString

		if err := rows.Scan(&schedule.Id, &schedule.Name, &description, &schedule.Timezone, &teamID, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan schedule: %w", err)
		}

		schedule.Description = description.String
		schedule.TeamId = teamID.String
		schedule.CreatedAt = timestamppb.New(createdAt)
		schedule.UpdatedAt = timestamppb.New(updatedAt)

		// Load rotations
		rotations, err := s.loadRotations(ctx, schedule.Id)
		if err != nil {
			return nil, err
		}
		schedule.Rotations = rotations

		schedules = append(schedules, schedule)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	resp := &routingv1.ListSchedulesResponse{
		TotalCount: int32(len(schedules)),
	}

	if len(schedules) > pageSize {
		schedules = schedules[:pageSize]
		offset := decodePageToken(req.PageToken)
		resp.NextPageToken = encodePageToken(offset + pageSize)
	}

	resp.Schedules = schedules
	return resp, nil
}

// UpdateSchedule updates an existing schedule.
func (s *PostgresStore) UpdateSchedule(ctx context.Context, schedule *routingv1.Schedule) (*routingv1.Schedule, error) {
	if schedule == nil || schedule.Id == "" {
		return nil, ErrInvalidSchedule
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now()
	schedule.UpdatedAt = timestamppb.New(now)

	var teamID *string
	if schedule.TeamId != "" {
		teamID = &schedule.TeamId
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE schedules SET name = $1, description = $2, timezone = $3, team_id = $4, updated_at = $5
		WHERE id = $6
	`, schedule.Name, schedule.Description, schedule.Timezone, teamID, now, schedule.Id)
	if err != nil {
		return nil, fmt.Errorf("update schedule: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return s.GetSchedule(ctx, schedule.Id)
}

// DeleteSchedule deletes a schedule by ID.
func (s *PostgresStore) DeleteSchedule(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM schedules WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete schedule: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// AddRotation adds a rotation to a schedule.
func (s *PostgresStore) AddRotation(ctx context.Context, scheduleID string, rotation *routingv1.Rotation) (*routingv1.Schedule, error) {
	if rotation == nil {
		return nil, ErrInvalidRotation
	}

	// Verify schedule exists
	if _, err := s.GetSchedule(ctx, scheduleID); err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.insertRotation(ctx, tx, scheduleID, rotation); err != nil {
		return nil, err
	}

	// Update schedule timestamp
	_, err = tx.ExecContext(ctx, "UPDATE schedules SET updated_at = $1 WHERE id = $2", time.Now(), scheduleID)
	if err != nil {
		return nil, fmt.Errorf("update schedule timestamp: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return s.GetSchedule(ctx, scheduleID)
}

// UpdateRotation updates a rotation within a schedule.
func (s *PostgresStore) UpdateRotation(ctx context.Context, scheduleID string, rotation *routingv1.Rotation) (*routingv1.Schedule, error) {
	if rotation == nil || rotation.Id == "" {
		return nil, ErrInvalidRotation
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Delete existing rotation and members
	_, err = tx.ExecContext(ctx, "DELETE FROM rotations WHERE id = $1 AND schedule_id = $2", rotation.Id, scheduleID)
	if err != nil {
		return nil, fmt.Errorf("delete existing rotation: %w", err)
	}

	// Re-insert rotation
	if err := s.insertRotation(ctx, tx, scheduleID, rotation); err != nil {
		return nil, err
	}

	// Update schedule timestamp
	_, err = tx.ExecContext(ctx, "UPDATE schedules SET updated_at = $1 WHERE id = $2", time.Now(), scheduleID)
	if err != nil {
		return nil, fmt.Errorf("update schedule timestamp: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return s.GetSchedule(ctx, scheduleID)
}

// RemoveRotation removes a rotation from a schedule.
func (s *PostgresStore) RemoveRotation(ctx context.Context, scheduleID, rotationID string) (*routingv1.Schedule, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, "DELETE FROM rotations WHERE id = $1 AND schedule_id = $2", rotationID, scheduleID)
	if err != nil {
		return nil, fmt.Errorf("delete rotation: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	// Update schedule timestamp
	_, err = tx.ExecContext(ctx, "UPDATE schedules SET updated_at = $1 WHERE id = $2", time.Now(), scheduleID)
	if err != nil {
		return nil, fmt.Errorf("update schedule timestamp: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return s.GetSchedule(ctx, scheduleID)
}

// CreateOverride creates a schedule override.
func (s *PostgresStore) CreateOverride(ctx context.Context, scheduleID string, override *routingv1.ScheduleOverride) (*routingv1.ScheduleOverride, error) {
	if override == nil {
		return nil, ErrInvalidOverride
	}

	// Verify schedule exists
	if _, err := s.GetSchedule(ctx, scheduleID); err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.insertOverride(ctx, tx, scheduleID, override); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	override.CreatedAt = timestamppb.Now()
	return override, nil
}

// DeleteOverride deletes a schedule override.
func (s *PostgresStore) DeleteOverride(ctx context.Context, scheduleID, overrideID string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM schedule_overrides WHERE id = $1 AND schedule_id = $2", overrideID, scheduleID)
	if err != nil {
		return fmt.Errorf("delete override: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// ListOverrides lists overrides for a schedule within a time range.
func (s *PostgresStore) ListOverrides(ctx context.Context, scheduleID string, startTime, endTime *timestamppb.Timestamp, pageSize int, pageToken string) (*routingv1.ListOverridesResponse, error) {
	query := `SELECT id, user_id, start_time, end_time, reason, created_by, created_at
		FROM schedule_overrides WHERE schedule_id = $1`
	args := []interface{}{scheduleID}
	argIndex := 2

	if startTime != nil {
		query += fmt.Sprintf(" AND end_time >= $%d", argIndex)
		args = append(args, startTime.AsTime())
		argIndex++
	}

	if endTime != nil {
		query += fmt.Sprintf(" AND start_time <= $%d", argIndex)
		args = append(args, endTime.AsTime())
		argIndex++
	}

	query += " ORDER BY start_time"

	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}
	query += fmt.Sprintf(" LIMIT $%d", argIndex)
	args = append(args, pageSize+1)
	argIndex++

	if pageToken != "" {
		offset := decodePageToken(pageToken)
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query overrides: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var overrides []*routingv1.ScheduleOverride
	for rows.Next() {
		override := &routingv1.ScheduleOverride{}
		var startT, endT, createdAt time.Time
		var reason, createdBy sql.NullString

		if err := rows.Scan(&override.Id, &override.UserId, &startT, &endT, &reason, &createdBy, &createdAt); err != nil {
			return nil, fmt.Errorf("scan override: %w", err)
		}

		override.StartTime = timestamppb.New(startT)
		override.EndTime = timestamppb.New(endT)
		override.Reason = reason.String
		override.CreatedBy = createdBy.String
		override.CreatedAt = timestamppb.New(createdAt)

		overrides = append(overrides, override)
	}

	resp := &routingv1.ListOverridesResponse{}

	if len(overrides) > pageSize {
		overrides = overrides[:pageSize]
		offset := decodePageToken(pageToken)
		resp.NextPageToken = encodePageToken(offset + pageSize)
	}

	resp.Overrides = overrides
	return resp, rows.Err()
}

// GetActiveOverrides returns overrides active at a given time.
func (s *PostgresStore) GetActiveOverrides(ctx context.Context, scheduleID string, at time.Time) ([]*routingv1.ScheduleOverride, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, start_time, end_time, reason, created_by, created_at
		FROM schedule_overrides
		WHERE schedule_id = $1 AND start_time <= $2 AND end_time > $2
		ORDER BY created_at DESC
	`, scheduleID, at)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var overrides []*routingv1.ScheduleOverride
	for rows.Next() {
		override := &routingv1.ScheduleOverride{}
		var startT, endT, createdAt time.Time
		var reason, createdBy sql.NullString

		if err := rows.Scan(&override.Id, &override.UserId, &startT, &endT, &reason, &createdBy, &createdAt); err != nil {
			return nil, err
		}

		override.StartTime = timestamppb.New(startT)
		override.EndTime = timestamppb.New(endT)
		override.Reason = reason.String
		override.CreatedBy = createdBy.String
		override.CreatedAt = timestamppb.New(createdAt)

		overrides = append(overrides, override)
	}

	return overrides, rows.Err()
}

// RecordHandoffAck records a handoff acknowledgment.
func (s *PostgresStore) RecordHandoffAck(ctx context.Context, scheduleID, userID string) error {
	// For now, we just verify the schedule exists
	// In a full implementation, we would record this in a handoff_acks table
	_, err := s.GetSchedule(ctx, scheduleID)
	return err
}

// Helper functions
func encodePageToken(offset int) string {
	return fmt.Sprintf("%d", offset)
}

func decodePageToken(token string) int {
	var offset int
	_, _ = fmt.Sscanf(token, "%d", &offset)
	return offset
}

func parseRotationType(s string) routingv1.RotationType {
	if v, ok := routingv1.RotationType_value[s]; ok {
		return routingv1.RotationType(v)
	}
	return routingv1.RotationType_ROTATION_TYPE_UNSPECIFIED
}

func intSliceToArray(s []int32) []byte {
	if s == nil {
		return nil
	}
	data, _ := json.Marshal(s)
	return data
}

// Ensure PostgresStore implements Store
var _ Store = (*PostgresStore)(nil)
