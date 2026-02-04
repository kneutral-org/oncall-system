// Package team provides the team store implementation.
package team

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kneutral-org/alerting-system/internal/sqlc"
)

// TeamRole represents a team member's role.
type TeamRole string

const (
	TeamRoleMember  TeamRole = "member"
	TeamRoleLead    TeamRole = "lead"
	TeamRoleManager TeamRole = "manager"
)

// Team represents a team domain model.
type Team struct {
	ID                        uuid.UUID              `json:"id"`
	Name                      string                 `json:"name"`
	Description               string                 `json:"description,omitempty"`
	DefaultEscalationPolicyID *uuid.UUID             `json:"defaultEscalationPolicyId,omitempty"`
	DefaultChannel            *NotificationChannel   `json:"defaultChannel,omitempty"`
	AssignedSites             []string               `json:"assignedSites"`
	AssignedPops              []string               `json:"assignedPops"`
	Metadata                  map[string]interface{} `json:"metadata"`
	CreatedAt                 time.Time              `json:"createdAt"`
	UpdatedAt                 time.Time              `json:"updatedAt"`
}

// NotificationChannel represents a default notification channel for the team.
type NotificationChannel struct {
	Type   string                 `json:"type"`
	Config map[string]interface{} `json:"config"`
}

// TeamMember represents a team member.
type TeamMember struct {
	TeamID      uuid.UUID              `json:"teamId"`
	UserID      uuid.UUID              `json:"userId"`
	Role        TeamRole               `json:"role"`
	Preferences map[string]interface{} `json:"preferences,omitempty"`
	JoinedAt    time.Time              `json:"joinedAt"`
}

// ListTeamsParams contains parameters for listing teams.
type ListTeamsParams struct {
	NameFilter  string
	SitesFilter []string
	PopsFilter  []string
	Limit       int32
	Offset      int32
}

// TeamStore defines the interface for team persistence.
type TeamStore interface {
	// CreateTeam creates a new team.
	CreateTeam(ctx context.Context, team *Team) (*Team, error)

	// GetTeam retrieves a team by ID.
	GetTeam(ctx context.Context, id uuid.UUID) (*Team, error)

	// ListTeams retrieves teams based on filter criteria.
	ListTeams(ctx context.Context, params ListTeamsParams) ([]*Team, error)

	// UpdateTeam updates an existing team.
	UpdateTeam(ctx context.Context, team *Team) (*Team, error)

	// DeleteTeam deletes a team.
	DeleteTeam(ctx context.Context, id uuid.UUID) error

	// AddMember adds a member to a team.
	AddMember(ctx context.Context, teamID, userID uuid.UUID, role TeamRole) error

	// RemoveMember removes a member from a team.
	RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error

	// UpdateMember updates a team member's role.
	UpdateMember(ctx context.Context, teamID, userID uuid.UUID, role TeamRole) error

	// GetTeamMembers retrieves all members of a team.
	GetTeamMembers(ctx context.Context, teamID uuid.UUID) ([]*TeamMember, error)

	// GetUserTeams retrieves all teams a user belongs to.
	GetUserTeams(ctx context.Context, userID uuid.UUID) ([]*Team, error)
}

// PostgresTeamStore is the PostgreSQL implementation of TeamStore.
type PostgresTeamStore struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

// NewPostgresTeamStore creates a new PostgreSQL team store.
func NewPostgresTeamStore(pool *pgxpool.Pool) *PostgresTeamStore {
	return &PostgresTeamStore{
		pool:    pool,
		queries: sqlc.New(pool),
	}
}

// CreateTeam creates a new team.
func (s *PostgresTeamStore) CreateTeam(ctx context.Context, team *Team) (*Team, error) {
	var defaultChannelJSON []byte
	var err error
	if team.DefaultChannel != nil {
		defaultChannelJSON, err = json.Marshal(team.DefaultChannel)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal default channel: %w", err)
		}
	}

	metadataJSON, err := json.Marshal(team.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	params := sqlc.CreateTeamParams{
		Name:                      team.Name,
		Description:               toPgText(team.Description),
		DefaultEscalationPolicyID: toPgUUID(team.DefaultEscalationPolicyID),
		DefaultChannel:            defaultChannelJSON,
		AssignedSites:             team.AssignedSites,
		AssignedPops:              team.AssignedPops,
		Metadata:                  metadataJSON,
	}

	result, err := s.queries.CreateTeam(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create team: %w", err)
	}

	return sqlcToTeam(&result)
}

// GetTeam retrieves a team by ID.
func (s *PostgresTeamStore) GetTeam(ctx context.Context, id uuid.UUID) (*Team, error) {
	result, err := s.queries.GetTeam(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get team: %w", err)
	}

	return sqlcToTeam(&result)
}

// ListTeams retrieves teams based on filter criteria.
func (s *PostgresTeamStore) ListTeams(ctx context.Context, params ListTeamsParams) ([]*Team, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}

	result, err := s.queries.ListTeams(ctx, sqlc.ListTeamsParams{
		NameFilter:  params.NameFilter,
		SitesFilter: params.SitesFilter,
		PopsFilter:  params.PopsFilter,
		Lim:         limit,
		Off:         params.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}

	teams := make([]*Team, 0, len(result))
	for _, r := range result {
		team, err := sqlcToTeam(&r)
		if err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}

	return teams, nil
}

// UpdateTeam updates an existing team.
func (s *PostgresTeamStore) UpdateTeam(ctx context.Context, team *Team) (*Team, error) {
	var defaultChannelJSON []byte
	var err error
	if team.DefaultChannel != nil {
		defaultChannelJSON, err = json.Marshal(team.DefaultChannel)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal default channel: %w", err)
		}
	}

	metadataJSON, err := json.Marshal(team.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	params := sqlc.UpdateTeamParams{
		ID:                        team.ID,
		Name:                      team.Name,
		Description:               toPgText(team.Description),
		DefaultEscalationPolicyID: toPgUUID(team.DefaultEscalationPolicyID),
		DefaultChannel:            defaultChannelJSON,
		AssignedSites:             team.AssignedSites,
		AssignedPops:              team.AssignedPops,
		Metadata:                  metadataJSON,
	}

	result, err := s.queries.UpdateTeam(ctx, params)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to update team: %w", err)
	}

	return sqlcToTeam(&result)
}

// DeleteTeam deletes a team.
func (s *PostgresTeamStore) DeleteTeam(ctx context.Context, id uuid.UUID) error {
	err := s.queries.DeleteTeam(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete team: %w", err)
	}
	return nil
}

// AddMember adds a member to a team.
func (s *PostgresTeamStore) AddMember(ctx context.Context, teamID, userID uuid.UUID, role TeamRole) error {
	err := s.queries.AddTeamMember(ctx, sqlc.AddTeamMemberParams{
		TeamID:      teamID,
		UserID:      userID,
		Role:        sqlc.TeamRole(role),
		Preferences: nil,
	})
	if err != nil {
		return fmt.Errorf("failed to add team member: %w", err)
	}
	return nil
}

// RemoveMember removes a member from a team.
func (s *PostgresTeamStore) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	err := s.queries.RemoveTeamMember(ctx, sqlc.RemoveTeamMemberParams{
		TeamID: teamID,
		UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("failed to remove team member: %w", err)
	}
	return nil
}

// UpdateMember updates a team member's role.
func (s *PostgresTeamStore) UpdateMember(ctx context.Context, teamID, userID uuid.UUID, role TeamRole) error {
	err := s.queries.UpdateTeamMember(ctx, sqlc.UpdateTeamMemberParams{
		TeamID:      teamID,
		UserID:      userID,
		Role:        sqlc.TeamRole(role),
		Preferences: nil,
	})
	if err != nil {
		return fmt.Errorf("failed to update team member: %w", err)
	}
	return nil
}

// GetTeamMembers retrieves all members of a team.
func (s *PostgresTeamStore) GetTeamMembers(ctx context.Context, teamID uuid.UUID) ([]*TeamMember, error) {
	result, err := s.queries.GetTeamMembers(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team members: %w", err)
	}

	members := make([]*TeamMember, 0, len(result))
	for _, r := range result {
		member, err := sqlcToTeamMember(&r)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}

	return members, nil
}

// GetUserTeams retrieves all teams a user belongs to.
func (s *PostgresTeamStore) GetUserTeams(ctx context.Context, userID uuid.UUID) ([]*Team, error) {
	result, err := s.queries.GetUserTeams(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user teams: %w", err)
	}

	teams := make([]*Team, 0, len(result))
	for _, r := range result {
		team, err := sqlcToTeam(&r)
		if err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}

	return teams, nil
}

// Helper functions

func sqlcToTeam(t *sqlc.Team) (*Team, error) {
	team := &Team{
		ID:            t.ID,
		Name:          t.Name,
		AssignedSites: t.AssignedSites,
		AssignedPops:  t.AssignedPops,
		CreatedAt:     t.CreatedAt,
		UpdatedAt:     t.UpdatedAt,
	}

	if t.Description.Valid {
		team.Description = t.Description.String
	}

	if t.DefaultEscalationPolicyID.Valid {
		id := uuid.UUID(t.DefaultEscalationPolicyID.Bytes)
		team.DefaultEscalationPolicyID = &id
	}

	if len(t.DefaultChannel) > 0 {
		team.DefaultChannel = &NotificationChannel{}
		if err := json.Unmarshal(t.DefaultChannel, team.DefaultChannel); err != nil {
			return nil, fmt.Errorf("failed to unmarshal default channel: %w", err)
		}
	}

	if len(t.Metadata) > 0 {
		if err := json.Unmarshal(t.Metadata, &team.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return team, nil
}

func sqlcToTeamMember(m *sqlc.TeamMember) (*TeamMember, error) {
	member := &TeamMember{
		TeamID:   m.TeamID,
		UserID:   m.UserID,
		Role:     TeamRole(m.Role),
		JoinedAt: m.JoinedAt,
	}

	if len(m.Preferences) > 0 {
		if err := json.Unmarshal(m.Preferences, &member.Preferences); err != nil {
			return nil, fmt.Errorf("failed to unmarshal preferences: %w", err)
		}
	}

	return member, nil
}

func toPgText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

func toPgUUID(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}
