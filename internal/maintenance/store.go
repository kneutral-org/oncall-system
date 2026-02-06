// Package maintenance provides maintenance window management for the alerting system.
package maintenance

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

var (
	// ErrNotFound is returned when a maintenance window is not found.
	ErrNotFound = errors.New("maintenance window not found")
	// ErrInvalidWindow is returned when a maintenance window is invalid.
	ErrInvalidWindow = errors.New("invalid maintenance window")
	// ErrInvalidStatus is returned when a status transition is invalid.
	ErrInvalidStatus = errors.New("invalid status transition")
)

// Scope represents the scope of a maintenance window.
type Scope struct {
	Sites       []string          `json:"sites,omitempty"`
	Services    []string          `json:"services,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	LabelRegex  map[string]string `json:"labelRegex,omitempty"`
	Equipment   []string          `json:"equipment,omitempty"`
}

// Store defines the interface for maintenance window persistence.
type Store interface {
	// Create creates a new maintenance window.
	Create(ctx context.Context, window *routingv1.MaintenanceWindow) (*routingv1.MaintenanceWindow, error)

	// Get retrieves a maintenance window by ID.
	Get(ctx context.Context, id string) (*routingv1.MaintenanceWindow, error)

	// List retrieves maintenance windows with optional filters.
	List(ctx context.Context, req *routingv1.ListMaintenanceWindowsRequest) (*routingv1.ListMaintenanceWindowsResponse, error)

	// Update updates an existing maintenance window.
	Update(ctx context.Context, window *routingv1.MaintenanceWindow) (*routingv1.MaintenanceWindow, error)

	// Delete deletes a maintenance window by ID.
	Delete(ctx context.Context, id string) error

	// ListActive retrieves currently active maintenance windows.
	ListActive(ctx context.Context, siteIDs, serviceIDs []string) ([]*routingv1.MaintenanceWindow, error)

	// ListUpcoming retrieves maintenance windows starting within the given duration.
	ListUpcoming(ctx context.Context, duration time.Duration) ([]*routingv1.MaintenanceWindow, error)

	// UpdateStatus updates the status of a maintenance window.
	UpdateStatus(ctx context.Context, id string, status routingv1.MaintenanceStatus) error

	// TransitionStatuses updates statuses based on current time (scheduled->active, active->completed).
	TransitionStatuses(ctx context.Context) error
}

// PostgresStore implements Store using PostgreSQL.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore creates a new PostgresStore.
func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

// Create creates a new maintenance window in the database.
func (s *PostgresStore) Create(ctx context.Context, window *routingv1.MaintenanceWindow) (*routingv1.MaintenanceWindow, error) {
	if window == nil {
		return nil, ErrInvalidWindow
	}

	if window.StartTime == nil || window.EndTime == nil {
		return nil, fmt.Errorf("%w: start_time and end_time are required", ErrInvalidWindow)
	}

	if window.EndTime.AsTime().Before(window.StartTime.AsTime()) {
		return nil, fmt.Errorf("%w: end_time must be after start_time", ErrInvalidWindow)
	}

	// Generate ID if not provided
	if window.Id == "" {
		window.Id = uuid.New().String()
	}

	now := time.Now()
	window.CreatedAt = timestamppb.New(now)

	// Determine initial status
	startTime := window.StartTime.AsTime()
	endTime := window.EndTime.AsTime()

	if now.After(endTime) {
		window.Status = routingv1.MaintenanceStatus_MAINTENANCE_STATUS_COMPLETED
	} else if now.After(startTime) {
		window.Status = routingv1.MaintenanceStatus_MAINTENANCE_STATUS_IN_PROGRESS
	} else {
		window.Status = routingv1.MaintenanceStatus_MAINTENANCE_STATUS_SCHEDULED
	}

	// Default action
	if window.Action == routingv1.MaintenanceAction_MAINTENANCE_ACTION_UNSPECIFIED {
		window.Action = routingv1.MaintenanceAction_MAINTENANCE_ACTION_ANNOTATE
	}

	// Build scope JSON
	scope := buildScopeJSON(window)
	scopeJSON, err := json.Marshal(scope)
	if err != nil {
		return nil, fmt.Errorf("marshal scope: %w", err)
	}

	// Insert the window
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO maintenance_windows (id, name, description, start_time, end_time, status, action, scope, ticket_id, ticket_url, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, window.Id, window.Name, window.Description,
		startTime, endTime,
		statusToString(window.Status),
		actionToString(window.Action),
		scopeJSON,
		nullableString(window.ChangeTicketId),
		nil, // ticket_url not in proto
		nullableString(window.CreatedBy),
		now, now)
	if err != nil {
		return nil, fmt.Errorf("insert maintenance window: %w", err)
	}

	return window, nil
}

// Get retrieves a maintenance window by ID.
func (s *PostgresStore) Get(ctx context.Context, id string) (*routingv1.MaintenanceWindow, error) {
	window := &routingv1.MaintenanceWindow{}

	var startTime, endTime, createdAt, updatedAt time.Time
	var description, status, action sql.NullString
	var scopeJSON []byte
	var ticketID, ticketURL, createdBy, approvedBy sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, start_time, end_time, status, action, scope,
			ticket_id, ticket_url, created_by, approved_by, created_at, updated_at
		FROM maintenance_windows WHERE id = $1
	`, id).Scan(
		&window.Id, &window.Name, &description,
		&startTime, &endTime,
		&status, &action, &scopeJSON,
		&ticketID, &ticketURL, &createdBy, &approvedBy,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query maintenance window: %w", err)
	}

	window.Description = description.String
	window.StartTime = timestamppb.New(startTime)
	window.EndTime = timestamppb.New(endTime)
	window.Status = parseStatus(status.String)
	window.Action = parseAction(action.String)
	window.ChangeTicketId = ticketID.String
	window.CreatedBy = createdBy.String
	window.CreatedAt = timestamppb.New(createdAt)

	// Parse scope
	if scopeJSON != nil {
		var scope Scope
		if err := json.Unmarshal(scopeJSON, &scope); err == nil {
			window.AffectedSites = scope.Sites
			window.AffectedServices = scope.Services
			window.AffectedLabels = scopeLabelsToStrings(scope.Labels)
		}
	}

	return window, nil
}

// List retrieves maintenance windows with optional filters.
func (s *PostgresStore) List(ctx context.Context, req *routingv1.ListMaintenanceWindowsRequest) (*routingv1.ListMaintenanceWindowsResponse, error) {
	query := `SELECT id, name, description, start_time, end_time, status, action, scope,
		ticket_id, ticket_url, created_by, approved_by, created_at, updated_at
		FROM maintenance_windows WHERE 1=1`
	args := []interface{}{}
	argIndex := 1

	if req.Status != routingv1.MaintenanceStatus_MAINTENANCE_STATUS_UNSPECIFIED {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, statusToString(req.Status))
		argIndex++
	}

	if req.StartTime != nil {
		query += fmt.Sprintf(" AND end_time >= $%d", argIndex)
		args = append(args, req.StartTime.AsTime())
		argIndex++
	}

	if req.EndTime != nil {
		query += fmt.Sprintf(" AND start_time <= $%d", argIndex)
		args = append(args, req.EndTime.AsTime())
		argIndex++
	}

	if req.SiteId != "" {
		query += fmt.Sprintf(" AND scope @> $%d::jsonb", argIndex)
		siteFilter, _ := json.Marshal(map[string][]string{"sites": {req.SiteId}})
		args = append(args, siteFilter)
		argIndex++
	}

	query += " ORDER BY start_time DESC"

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
		return nil, fmt.Errorf("query maintenance windows: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var windows []*routingv1.MaintenanceWindow
	for rows.Next() {
		window, err := s.scanWindow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan maintenance window: %w", err)
		}
		windows = append(windows, window)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	resp := &routingv1.ListMaintenanceWindowsResponse{
		TotalCount: int32(len(windows)),
	}

	if len(windows) > pageSize {
		windows = windows[:pageSize]
		offset := decodePageToken(req.PageToken)
		resp.NextPageToken = encodePageToken(offset + pageSize)
	}

	resp.Windows = windows
	return resp, nil
}

// Update updates an existing maintenance window.
func (s *PostgresStore) Update(ctx context.Context, window *routingv1.MaintenanceWindow) (*routingv1.MaintenanceWindow, error) {
	if window == nil || window.Id == "" {
		return nil, ErrInvalidWindow
	}

	// Build scope JSON
	scope := buildScopeJSON(window)
	scopeJSON, err := json.Marshal(scope)
	if err != nil {
		return nil, fmt.Errorf("marshal scope: %w", err)
	}

	now := time.Now()

	result, err := s.db.ExecContext(ctx, `
		UPDATE maintenance_windows
		SET name = $1, description = $2, start_time = $3, end_time = $4,
			status = $5, action = $6, scope = $7, ticket_id = $8, updated_at = $9
		WHERE id = $10
	`, window.Name, window.Description,
		window.StartTime.AsTime(), window.EndTime.AsTime(),
		statusToString(window.Status),
		actionToString(window.Action),
		scopeJSON,
		nullableString(window.ChangeTicketId),
		now,
		window.Id)
	if err != nil {
		return nil, fmt.Errorf("update maintenance window: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return s.Get(ctx, window.Id)
}

// Delete deletes a maintenance window by ID.
func (s *PostgresStore) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM maintenance_windows WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete maintenance window: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// ListActive retrieves currently active maintenance windows.
func (s *PostgresStore) ListActive(ctx context.Context, siteIDs, serviceIDs []string) ([]*routingv1.MaintenanceWindow, error) {
	now := time.Now()

	query := `SELECT id, name, description, start_time, end_time, status, action, scope,
		ticket_id, ticket_url, created_by, approved_by, created_at, updated_at
		FROM maintenance_windows
		WHERE status = 'active' AND start_time <= $1 AND end_time > $1`
	args := []interface{}{now}
	argIndex := 2

	// Filter by sites if provided
	if len(siteIDs) > 0 {
		// Check if any site in the scope matches
		query += fmt.Sprintf(" AND (scope->'sites' ?| $%d OR NOT scope ? 'sites' OR scope->'sites' = '[]'::jsonb)", argIndex)
		args = append(args, pq(siteIDs))
		argIndex++
	}

	// Filter by services if provided
	if len(serviceIDs) > 0 {
		query += fmt.Sprintf(" AND (scope->'services' ?| $%d OR NOT scope ? 'services' OR scope->'services' = '[]'::jsonb)", argIndex)
		args = append(args, pq(serviceIDs))
		// argIndex++ // Commented out as it's the last usage
	}

	query += " ORDER BY start_time ASC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query active maintenance windows: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var windows []*routingv1.MaintenanceWindow
	for rows.Next() {
		window, err := s.scanWindow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan maintenance window: %w", err)
		}
		windows = append(windows, window)
	}

	return windows, rows.Err()
}

// ListUpcoming retrieves maintenance windows starting within the given duration.
func (s *PostgresStore) ListUpcoming(ctx context.Context, duration time.Duration) ([]*routingv1.MaintenanceWindow, error) {
	now := time.Now()
	until := now.Add(duration)

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, start_time, end_time, status, action, scope,
			ticket_id, ticket_url, created_by, approved_by, created_at, updated_at
		FROM maintenance_windows
		WHERE status = 'scheduled' AND start_time > $1 AND start_time <= $2
		ORDER BY start_time ASC
	`, now, until)
	if err != nil {
		return nil, fmt.Errorf("query upcoming maintenance windows: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var windows []*routingv1.MaintenanceWindow
	for rows.Next() {
		window, err := s.scanWindow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan maintenance window: %w", err)
		}
		windows = append(windows, window)
	}

	return windows, rows.Err()
}

// UpdateStatus updates the status of a maintenance window.
func (s *PostgresStore) UpdateStatus(ctx context.Context, id string, status routingv1.MaintenanceStatus) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE maintenance_windows SET status = $1, updated_at = $2 WHERE id = $3
	`, statusToString(status), time.Now(), id)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// TransitionStatuses updates statuses based on current time.
func (s *PostgresStore) TransitionStatuses(ctx context.Context) error {
	now := time.Now()

	// Transition scheduled -> active
	_, err := s.db.ExecContext(ctx, `
		UPDATE maintenance_windows
		SET status = 'active', updated_at = $1
		WHERE status = 'scheduled' AND start_time <= $1
	`, now)
	if err != nil {
		return fmt.Errorf("transition scheduled to active: %w", err)
	}

	// Transition active -> completed
	_, err = s.db.ExecContext(ctx, `
		UPDATE maintenance_windows
		SET status = 'completed', updated_at = $1
		WHERE status = 'active' AND end_time <= $1
	`, now)
	if err != nil {
		return fmt.Errorf("transition active to completed: %w", err)
	}

	return nil
}

// scanWindow scans a maintenance window from a row.
func (s *PostgresStore) scanWindow(rows *sql.Rows) (*routingv1.MaintenanceWindow, error) {
	window := &routingv1.MaintenanceWindow{}

	var startTime, endTime, createdAt, updatedAt time.Time
	var description, status, action sql.NullString
	var scopeJSON []byte
	var ticketID, ticketURL, createdBy, approvedBy sql.NullString

	if err := rows.Scan(
		&window.Id, &window.Name, &description,
		&startTime, &endTime,
		&status, &action, &scopeJSON,
		&ticketID, &ticketURL, &createdBy, &approvedBy,
		&createdAt, &updatedAt,
	); err != nil {
		return nil, err
	}

	window.Description = description.String
	window.StartTime = timestamppb.New(startTime)
	window.EndTime = timestamppb.New(endTime)
	window.Status = parseStatus(status.String)
	window.Action = parseAction(action.String)
	window.ChangeTicketId = ticketID.String
	window.CreatedBy = createdBy.String
	window.CreatedAt = timestamppb.New(createdAt)

	// Parse scope
	if scopeJSON != nil {
		var scope Scope
		if err := json.Unmarshal(scopeJSON, &scope); err == nil {
			window.AffectedSites = scope.Sites
			window.AffectedServices = scope.Services
			window.AffectedLabels = scopeLabelsToStrings(scope.Labels)
		}
	}

	return window, nil
}

// Helper functions

func buildScopeJSON(window *routingv1.MaintenanceWindow) Scope {
	scope := Scope{
		Sites:    window.AffectedSites,
		Services: window.AffectedServices,
		Labels:   make(map[string]string),
	}

	// Parse affected_labels which are in "key=value" format
	for _, label := range window.AffectedLabels {
		key, value := parseLabelMatcher(label)
		if key != "" {
			scope.Labels[key] = value
		}
	}

	return scope
}

func scopeLabelsToStrings(labels map[string]string) []string {
	var result []string
	for k, v := range labels {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}

func parseLabelMatcher(label string) (string, string) {
	for i, c := range label {
		if c == '=' {
			return label[:i], label[i+1:]
		}
	}
	return label, ""
}

func statusToString(status routingv1.MaintenanceStatus) string {
	switch status {
	case routingv1.MaintenanceStatus_MAINTENANCE_STATUS_SCHEDULED:
		return "scheduled"
	case routingv1.MaintenanceStatus_MAINTENANCE_STATUS_IN_PROGRESS:
		return "active"
	case routingv1.MaintenanceStatus_MAINTENANCE_STATUS_COMPLETED:
		return "completed"
	case routingv1.MaintenanceStatus_MAINTENANCE_STATUS_CANCELLED:
		return "cancelled"
	default:
		return "scheduled"
	}
}

func parseStatus(s string) routingv1.MaintenanceStatus {
	switch s {
	case "scheduled":
		return routingv1.MaintenanceStatus_MAINTENANCE_STATUS_SCHEDULED
	case "active":
		return routingv1.MaintenanceStatus_MAINTENANCE_STATUS_IN_PROGRESS
	case "completed":
		return routingv1.MaintenanceStatus_MAINTENANCE_STATUS_COMPLETED
	case "cancelled":
		return routingv1.MaintenanceStatus_MAINTENANCE_STATUS_CANCELLED
	default:
		return routingv1.MaintenanceStatus_MAINTENANCE_STATUS_UNSPECIFIED
	}
}

func actionToString(action routingv1.MaintenanceAction) string {
	switch action {
	case routingv1.MaintenanceAction_MAINTENANCE_ACTION_SUPPRESS:
		return "suppress"
	case routingv1.MaintenanceAction_MAINTENANCE_ACTION_ANNOTATE:
		return "annotate"
	case routingv1.MaintenanceAction_MAINTENANCE_ACTION_REDUCE_SEVERITY:
		return "route_to_team" // Maps to reduce_severity conceptually
	default:
		return "annotate"
	}
}

func parseAction(s string) routingv1.MaintenanceAction {
	switch s {
	case "suppress":
		return routingv1.MaintenanceAction_MAINTENANCE_ACTION_SUPPRESS
	case "annotate":
		return routingv1.MaintenanceAction_MAINTENANCE_ACTION_ANNOTATE
	case "route_to_team":
		return routingv1.MaintenanceAction_MAINTENANCE_ACTION_REDUCE_SEVERITY
	default:
		return routingv1.MaintenanceAction_MAINTENANCE_ACTION_UNSPECIFIED
	}
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func encodePageToken(offset int) string {
	return fmt.Sprintf("%d", offset)
}

func decodePageToken(token string) int {
	var offset int
	_, _ = fmt.Sscanf(token, "%d", &offset)
	return offset
}

// pq converts a string slice to a PostgreSQL array format for the ?| operator.
func pq(s []string) string {
	data, _ := json.Marshal(s)
	return string(data)
}

// Ensure PostgresStore implements Store
var _ Store = (*PostgresStore)(nil)
