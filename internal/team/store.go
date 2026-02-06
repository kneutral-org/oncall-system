// Package team provides team management functionality for the on-call system.
package team

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

var (
	// ErrNotFound is returned when a team is not found.
	ErrNotFound = errors.New("team not found")
	// ErrInvalidTeam is returned when a team is invalid.
	ErrInvalidTeam = errors.New("invalid team")
	// ErrDuplicateName is returned when a team name already exists.
	ErrDuplicateName = errors.New("team name already exists")
	// ErrMemberNotFound is returned when a team member is not found.
	ErrMemberNotFound = errors.New("team member not found")
	// ErrMemberExists is returned when a team member already exists.
	ErrMemberExists = errors.New("team member already exists")
)

// Store defines the interface for team persistence.
type Store interface {
	// Team CRUD
	Create(ctx context.Context, team *routingv1.Team) (*routingv1.Team, error)
	Get(ctx context.Context, id string) (*routingv1.Team, error)
	List(ctx context.Context, req *routingv1.ListTeamsRequest) (*routingv1.ListTeamsResponse, error)
	Update(ctx context.Context, team *routingv1.Team) (*routingv1.Team, error)
	Delete(ctx context.Context, id string) error

	// Member management
	AddMember(ctx context.Context, teamID string, member *routingv1.TeamMember) (*routingv1.Team, error)
	RemoveMember(ctx context.Context, teamID, userID string) (*routingv1.Team, error)
	UpdateMember(ctx context.Context, teamID string, member *routingv1.TeamMember) (*routingv1.Team, error)

	// Get teams by user
	GetByUser(ctx context.Context, userID string) ([]*routingv1.Team, error)
}

// PostgresStore implements Store using PostgreSQL.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore creates a new PostgresStore.
func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

// Create creates a new team in the database.
func (s *PostgresStore) Create(ctx context.Context, team *routingv1.Team) (*routingv1.Team, error) {
	if team == nil {
		return nil, ErrInvalidTeam
	}

	if team.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidTeam)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Generate ID if not provided
	if team.Id == "" {
		team.Id = uuid.New().String()
	}

	now := time.Now()
	team.CreatedAt = timestamppb.New(now)
	team.UpdatedAt = timestamppb.New(now)

	// Insert the team
	_, err = tx.ExecContext(ctx, `
		INSERT INTO teams (id, name, description, default_escalation_policy_id, default_notification_channel_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, team.Id, team.Name, nullableString(team.Description),
		nullableString(team.DefaultEscalationPolicyId), nil, now, now)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrDuplicateName
		}
		return nil, fmt.Errorf("insert team: %w", err)
	}

	// Insert team members
	for _, member := range team.Members {
		if err := s.insertMember(ctx, tx, team.Id, member); err != nil {
			return nil, fmt.Errorf("insert member: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return team, nil
}

// insertMember inserts a team member into the database.
func (s *PostgresStore) insertMember(ctx context.Context, tx *sql.Tx, teamID string, member *routingv1.TeamMember) error {
	if member.UserId == "" {
		return fmt.Errorf("user_id is required")
	}

	role := roleToString(member.Role)
	now := time.Now()

	_, err := tx.ExecContext(ctx, `
		INSERT INTO team_members (team_id, user_id, role, joined_at)
		VALUES ($1, $2, $3, $4)
	`, teamID, member.UserId, role, now)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return ErrMemberExists
		}
		return fmt.Errorf("insert team member: %w", err)
	}

	return nil
}

// Get retrieves a team by ID with all members.
func (s *PostgresStore) Get(ctx context.Context, id string) (*routingv1.Team, error) {
	team := &routingv1.Team{}

	var createdAt, updatedAt time.Time
	var description, defaultEscalationPolicyID, defaultNotificationChannelID sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, default_escalation_policy_id, default_notification_channel_id, created_at, updated_at
		FROM teams WHERE id = $1
	`, id).Scan(&team.Id, &team.Name, &description, &defaultEscalationPolicyID, &defaultNotificationChannelID, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query team: %w", err)
	}

	team.Description = description.String
	team.DefaultEscalationPolicyId = defaultEscalationPolicyID.String
	team.CreatedAt = timestamppb.New(createdAt)
	team.UpdatedAt = timestamppb.New(updatedAt)

	// Load members
	members, err := s.loadMembers(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load members: %w", err)
	}
	team.Members = members

	// Extract manager user IDs
	for _, member := range members {
		if member.Role == routingv1.TeamRole_TEAM_ROLE_MANAGER || member.Role == routingv1.TeamRole_TEAM_ROLE_LEAD {
			team.ManagerUserIds = append(team.ManagerUserIds, member.UserId)
		}
	}

	return team, nil
}

// loadMembers loads all members for a team.
func (s *PostgresStore) loadMembers(ctx context.Context, teamID string) ([]*routingv1.TeamMember, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT user_id, role, joined_at
		FROM team_members WHERE team_id = $1
		ORDER BY joined_at
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var members []*routingv1.TeamMember
	for rows.Next() {
		member := &routingv1.TeamMember{}
		var role string
		var joinedAt time.Time

		if err := rows.Scan(&member.UserId, &role, &joinedAt); err != nil {
			return nil, err
		}

		member.Role = parseRole(role)
		member.JoinedAt = timestamppb.New(joinedAt)

		members = append(members, member)
	}

	return members, rows.Err()
}

// List retrieves teams with optional filters.
func (s *PostgresStore) List(ctx context.Context, req *routingv1.ListTeamsRequest) (*routingv1.ListTeamsResponse, error) {
	query := `SELECT id, name, description, default_escalation_policy_id, default_notification_channel_id, created_at, updated_at FROM teams WHERE 1=1`
	args := []interface{}{}
	argIndex := 1

	if req.NameContains != "" {
		query += fmt.Sprintf(" AND name ILIKE $%d", argIndex)
		args = append(args, "%"+req.NameContains+"%")
		argIndex++
	}

	// Filter by site - requires joining with a hypothetical team_sites table or metadata
	// For now, we skip this filter as assigned_sites are not stored in the DB per the migration

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
		return nil, fmt.Errorf("query teams: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var teams []*routingv1.Team
	for rows.Next() {
		team := &routingv1.Team{}
		var createdAt, updatedAt time.Time
		var description, defaultEscalationPolicyID, defaultNotificationChannelID sql.NullString

		if err := rows.Scan(&team.Id, &team.Name, &description, &defaultEscalationPolicyID, &defaultNotificationChannelID, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan team: %w", err)
		}

		team.Description = description.String
		team.DefaultEscalationPolicyId = defaultEscalationPolicyID.String
		team.CreatedAt = timestamppb.New(createdAt)
		team.UpdatedAt = timestamppb.New(updatedAt)

		// Load members
		members, err := s.loadMembers(ctx, team.Id)
		if err != nil {
			return nil, err
		}
		team.Members = members

		teams = append(teams, team)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	resp := &routingv1.ListTeamsResponse{
		TotalCount: int32(len(teams)),
	}

	if len(teams) > pageSize {
		teams = teams[:pageSize]
		offset := decodePageToken(req.PageToken)
		resp.NextPageToken = encodePageToken(offset + pageSize)
	}

	resp.Teams = teams
	return resp, nil
}

// Update updates an existing team.
func (s *PostgresStore) Update(ctx context.Context, team *routingv1.Team) (*routingv1.Team, error) {
	if team == nil || team.Id == "" {
		return nil, ErrInvalidTeam
	}

	now := time.Now()

	result, err := s.db.ExecContext(ctx, `
		UPDATE teams SET name = $1, description = $2, default_escalation_policy_id = $3, updated_at = $4
		WHERE id = $5
	`, team.Name, nullableString(team.Description), nullableString(team.DefaultEscalationPolicyId), now, team.Id)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrDuplicateName
		}
		return nil, fmt.Errorf("update team: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return s.Get(ctx, team.Id)
}

// Delete deletes a team by ID.
func (s *PostgresStore) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM teams WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete team: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// AddMember adds a member to a team.
func (s *PostgresStore) AddMember(ctx context.Context, teamID string, member *routingv1.TeamMember) (*routingv1.Team, error) {
	if member == nil {
		return nil, fmt.Errorf("member is required")
	}

	// Verify team exists
	if _, err := s.Get(ctx, teamID); err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.insertMember(ctx, tx, teamID, member); err != nil {
		return nil, err
	}

	// Update team timestamp
	_, err = tx.ExecContext(ctx, "UPDATE teams SET updated_at = $1 WHERE id = $2", time.Now(), teamID)
	if err != nil {
		return nil, fmt.Errorf("update team timestamp: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return s.Get(ctx, teamID)
}

// RemoveMember removes a member from a team.
func (s *PostgresStore) RemoveMember(ctx context.Context, teamID, userID string) (*routingv1.Team, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, "DELETE FROM team_members WHERE team_id = $1 AND user_id = $2", teamID, userID)
	if err != nil {
		return nil, fmt.Errorf("delete team member: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, ErrMemberNotFound
	}

	// Update team timestamp
	_, err = tx.ExecContext(ctx, "UPDATE teams SET updated_at = $1 WHERE id = $2", time.Now(), teamID)
	if err != nil {
		return nil, fmt.Errorf("update team timestamp: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return s.Get(ctx, teamID)
}

// UpdateMember updates a member's role in a team.
func (s *PostgresStore) UpdateMember(ctx context.Context, teamID string, member *routingv1.TeamMember) (*routingv1.Team, error) {
	if member == nil || member.UserId == "" {
		return nil, fmt.Errorf("member with user_id is required")
	}

	role := roleToString(member.Role)

	result, err := s.db.ExecContext(ctx, `
		UPDATE team_members SET role = $1 WHERE team_id = $2 AND user_id = $3
	`, role, teamID, member.UserId)
	if err != nil {
		return nil, fmt.Errorf("update team member: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, ErrMemberNotFound
	}

	// Update team timestamp
	_, err = s.db.ExecContext(ctx, "UPDATE teams SET updated_at = $1 WHERE id = $2", time.Now(), teamID)
	if err != nil {
		return nil, fmt.Errorf("update team timestamp: %w", err)
	}

	return s.Get(ctx, teamID)
}

// GetByUser retrieves all teams that a user is a member of.
func (s *PostgresStore) GetByUser(ctx context.Context, userID string) ([]*routingv1.Team, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.name, t.description, t.default_escalation_policy_id, t.default_notification_channel_id, t.created_at, t.updated_at
		FROM teams t
		INNER JOIN team_members tm ON t.id = tm.team_id
		WHERE tm.user_id = $1
		ORDER BY t.name
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query teams by user: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var teams []*routingv1.Team
	for rows.Next() {
		team := &routingv1.Team{}
		var createdAt, updatedAt time.Time
		var description, defaultEscalationPolicyID, defaultNotificationChannelID sql.NullString

		if err := rows.Scan(&team.Id, &team.Name, &description, &defaultEscalationPolicyID, &defaultNotificationChannelID, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan team: %w", err)
		}

		team.Description = description.String
		team.DefaultEscalationPolicyId = defaultEscalationPolicyID.String
		team.CreatedAt = timestamppb.New(createdAt)
		team.UpdatedAt = timestamppb.New(updatedAt)

		// Load members
		members, err := s.loadMembers(ctx, team.Id)
		if err != nil {
			return nil, err
		}
		team.Members = members

		teams = append(teams, team)
	}

	return teams, rows.Err()
}

// Helper functions

func roleToString(role routingv1.TeamRole) string {
	switch role {
	case routingv1.TeamRole_TEAM_ROLE_MANAGER:
		return "admin" // Maps to DB schema
	case routingv1.TeamRole_TEAM_ROLE_LEAD:
		return "admin" // Lead is treated as admin for DB purposes
	case routingv1.TeamRole_TEAM_ROLE_MEMBER:
		return "member"
	default:
		return "member"
	}
}

func parseRole(s string) routingv1.TeamRole {
	switch s {
	case "admin":
		return routingv1.TeamRole_TEAM_ROLE_MANAGER
	case "member":
		return routingv1.TeamRole_TEAM_ROLE_MEMBER
	case "observer":
		return routingv1.TeamRole_TEAM_ROLE_MEMBER // Observers are treated as members in proto
	default:
		return routingv1.TeamRole_TEAM_ROLE_UNSPECIFIED
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

// Ensure PostgresStore implements Store
var _ Store = (*PostgresStore)(nil)
