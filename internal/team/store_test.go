// Package team provides the team store implementation.
package team

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockTeamStore is an in-memory implementation for testing.
type MockTeamStore struct {
	teams   map[uuid.UUID]*Team
	members map[uuid.UUID]map[uuid.UUID]*TeamMember
}

// NewMockTeamStore creates a new mock store.
func NewMockTeamStore() *MockTeamStore {
	return &MockTeamStore{
		teams:   make(map[uuid.UUID]*Team),
		members: make(map[uuid.UUID]map[uuid.UUID]*TeamMember),
	}
}

func (m *MockTeamStore) CreateTeam(ctx context.Context, team *Team) (*Team, error) {
	team.ID = uuid.New()
	team.CreatedAt = time.Now()
	team.UpdatedAt = time.Now()
	m.teams[team.ID] = team
	m.members[team.ID] = make(map[uuid.UUID]*TeamMember)
	return team, nil
}

func (m *MockTeamStore) GetTeam(ctx context.Context, id uuid.UUID) (*Team, error) {
	team, ok := m.teams[id]
	if !ok {
		return nil, nil
	}
	return team, nil
}

func (m *MockTeamStore) ListTeams(ctx context.Context, params ListTeamsParams) ([]*Team, error) {
	teams := make([]*Team, 0)
	for _, t := range m.teams {
		// Apply name filter
		if params.NameFilter != "" {
			// Simple contains check for testing
			if len(t.Name) < len(params.NameFilter) {
				continue
			}
			found := false
			for i := 0; i <= len(t.Name)-len(params.NameFilter); i++ {
				if t.Name[i:i+len(params.NameFilter)] == params.NameFilter {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		// Apply sites filter
		if len(params.SitesFilter) > 0 {
			match := false
			for _, fs := range params.SitesFilter {
				for _, ts := range t.AssignedSites {
					if fs == ts {
						match = true
						break
					}
				}
			}
			if !match {
				continue
			}
		}
		teams = append(teams, t)
	}
	return teams, nil
}

func (m *MockTeamStore) UpdateTeam(ctx context.Context, team *Team) (*Team, error) {
	existing, ok := m.teams[team.ID]
	if !ok {
		return nil, nil
	}
	team.CreatedAt = existing.CreatedAt
	team.UpdatedAt = time.Now()
	m.teams[team.ID] = team
	return team, nil
}

func (m *MockTeamStore) DeleteTeam(ctx context.Context, id uuid.UUID) error {
	delete(m.teams, id)
	delete(m.members, id)
	return nil
}

func (m *MockTeamStore) AddMember(ctx context.Context, teamID, userID uuid.UUID, role TeamRole) error {
	if _, ok := m.members[teamID]; !ok {
		m.members[teamID] = make(map[uuid.UUID]*TeamMember)
	}
	m.members[teamID][userID] = &TeamMember{
		TeamID:   teamID,
		UserID:   userID,
		Role:     role,
		JoinedAt: time.Now(),
	}
	return nil
}

func (m *MockTeamStore) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	if members, ok := m.members[teamID]; ok {
		delete(members, userID)
	}
	return nil
}

func (m *MockTeamStore) UpdateMember(ctx context.Context, teamID, userID uuid.UUID, role TeamRole) error {
	if members, ok := m.members[teamID]; ok {
		if member, exists := members[userID]; exists {
			member.Role = role
		}
	}
	return nil
}

func (m *MockTeamStore) GetTeamMembers(ctx context.Context, teamID uuid.UUID) ([]*TeamMember, error) {
	members := make([]*TeamMember, 0)
	if teamMembers, ok := m.members[teamID]; ok {
		for _, member := range teamMembers {
			members = append(members, member)
		}
	}
	return members, nil
}

func (m *MockTeamStore) GetUserTeams(ctx context.Context, userID uuid.UUID) ([]*Team, error) {
	teams := make([]*Team, 0)
	for teamID, members := range m.members {
		if _, ok := members[userID]; ok {
			if team, exists := m.teams[teamID]; exists {
				teams = append(teams, team)
			}
		}
	}
	return teams, nil
}

// Verify MockTeamStore implements TeamStore interface
var _ TeamStore = (*MockTeamStore)(nil)

func TestMockTeamStore_CreateTeam(t *testing.T) {
	store := NewMockTeamStore()
	ctx := context.Background()

	team := &Team{
		Name:          "NOC Team",
		Description:   "Network Operations Center",
		AssignedSites: []string{"DC1", "DC2"},
		AssignedPops:  []string{"POP-NYC", "POP-LAX"},
		Metadata:      map[string]interface{}{"region": "US-East"},
	}

	created, err := store.CreateTeam(ctx, team)
	require.NoError(t, err)
	require.NotNil(t, created)

	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, "NOC Team", created.Name)
	assert.Equal(t, "Network Operations Center", created.Description)
	assert.ElementsMatch(t, []string{"DC1", "DC2"}, created.AssignedSites)
	assert.ElementsMatch(t, []string{"POP-NYC", "POP-LAX"}, created.AssignedPops)
	assert.Equal(t, "US-East", created.Metadata["region"])
	assert.False(t, created.CreatedAt.IsZero())
	assert.False(t, created.UpdatedAt.IsZero())
}

func TestMockTeamStore_GetTeam(t *testing.T) {
	store := NewMockTeamStore()
	ctx := context.Background()

	team := &Team{
		Name: "Test Team",
	}

	created, err := store.CreateTeam(ctx, team)
	require.NoError(t, err)

	// Get existing team
	fetched, err := store.GetTeam(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, "Test Team", fetched.Name)

	// Get non-existing team
	fetched, err = store.GetTeam(ctx, uuid.New())
	require.NoError(t, err)
	assert.Nil(t, fetched)
}

func TestMockTeamStore_ListTeams(t *testing.T) {
	store := NewMockTeamStore()
	ctx := context.Background()

	// Create test teams
	teams := []*Team{
		{Name: "NOC Team", AssignedSites: []string{"DC1"}},
		{Name: "Database Team", AssignedSites: []string{"DC2"}},
		{Name: "Network Team", AssignedSites: []string{"DC1", "DC3"}},
	}

	for _, team := range teams {
		_, err := store.CreateTeam(ctx, team)
		require.NoError(t, err)
	}

	// List all teams
	result, err := store.ListTeams(ctx, ListTeamsParams{})
	require.NoError(t, err)
	assert.Len(t, result, 3)

	// List by name filter
	result, err = store.ListTeams(ctx, ListTeamsParams{NameFilter: "NOC"})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "NOC Team", result[0].Name)

	// List by site filter
	result, err = store.ListTeams(ctx, ListTeamsParams{SitesFilter: []string{"DC1"}})
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestMockTeamStore_UpdateTeam(t *testing.T) {
	store := NewMockTeamStore()
	ctx := context.Background()

	team := &Team{
		Name:        "Test Team",
		Description: "Original description",
	}

	created, err := store.CreateTeam(ctx, team)
	require.NoError(t, err)

	// Update the team
	created.Name = "Updated Team"
	created.Description = "Updated description"
	created.AssignedSites = []string{"DC-NEW"}

	updated, err := store.UpdateTeam(ctx, created)
	require.NoError(t, err)
	require.NotNil(t, updated)

	assert.Equal(t, "Updated Team", updated.Name)
	assert.Equal(t, "Updated description", updated.Description)
	assert.ElementsMatch(t, []string{"DC-NEW"}, updated.AssignedSites)
	assert.True(t, updated.UpdatedAt.After(updated.CreatedAt))

	// Update non-existing team
	nonExisting := &Team{ID: uuid.New(), Name: "Non-existing"}
	result, err := store.UpdateTeam(ctx, nonExisting)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestMockTeamStore_DeleteTeam(t *testing.T) {
	store := NewMockTeamStore()
	ctx := context.Background()

	team := &Team{Name: "Test Team"}

	created, err := store.CreateTeam(ctx, team)
	require.NoError(t, err)

	// Add a member
	userID := uuid.New()
	err = store.AddMember(ctx, created.ID, userID, TeamRoleMember)
	require.NoError(t, err)

	// Delete the team
	err = store.DeleteTeam(ctx, created.ID)
	require.NoError(t, err)

	// Verify team is deleted
	fetched, err := store.GetTeam(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, fetched)

	// Verify members are also deleted
	members, err := store.GetTeamMembers(ctx, created.ID)
	require.NoError(t, err)
	assert.Len(t, members, 0)
}

func TestMockTeamStore_MemberOperations(t *testing.T) {
	store := NewMockTeamStore()
	ctx := context.Background()

	team := &Team{Name: "Test Team"}
	created, err := store.CreateTeam(ctx, team)
	require.NoError(t, err)

	userID1 := uuid.New()
	userID2 := uuid.New()

	// Add members
	err = store.AddMember(ctx, created.ID, userID1, TeamRoleMember)
	require.NoError(t, err)

	err = store.AddMember(ctx, created.ID, userID2, TeamRoleLead)
	require.NoError(t, err)

	// Get team members
	members, err := store.GetTeamMembers(ctx, created.ID)
	require.NoError(t, err)
	assert.Len(t, members, 2)

	// Update member role
	err = store.UpdateMember(ctx, created.ID, userID1, TeamRoleManager)
	require.NoError(t, err)

	// Verify role was updated
	members, err = store.GetTeamMembers(ctx, created.ID)
	require.NoError(t, err)

	var foundMember *TeamMember
	for _, m := range members {
		if m.UserID == userID1 {
			foundMember = m
			break
		}
	}
	require.NotNil(t, foundMember)
	assert.Equal(t, TeamRoleManager, foundMember.Role)

	// Remove member
	err = store.RemoveMember(ctx, created.ID, userID1)
	require.NoError(t, err)

	members, err = store.GetTeamMembers(ctx, created.ID)
	require.NoError(t, err)
	assert.Len(t, members, 1)
	assert.Equal(t, userID2, members[0].UserID)
}

func TestMockTeamStore_GetUserTeams(t *testing.T) {
	store := NewMockTeamStore()
	ctx := context.Background()

	// Create multiple teams
	team1, err := store.CreateTeam(ctx, &Team{Name: "Team 1"})
	require.NoError(t, err)

	team2, err := store.CreateTeam(ctx, &Team{Name: "Team 2"})
	require.NoError(t, err)

	_, err = store.CreateTeam(ctx, &Team{Name: "Team 3"})
	require.NoError(t, err)

	userID := uuid.New()

	// Add user to team1 and team2
	err = store.AddMember(ctx, team1.ID, userID, TeamRoleMember)
	require.NoError(t, err)

	err = store.AddMember(ctx, team2.ID, userID, TeamRoleLead)
	require.NoError(t, err)

	// Get user's teams
	teams, err := store.GetUserTeams(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, teams, 2)

	teamNames := make([]string, len(teams))
	for i, t := range teams {
		teamNames[i] = t.Name
	}
	assert.ElementsMatch(t, []string{"Team 1", "Team 2"}, teamNames)
}

func TestTeamRole_Constants(t *testing.T) {
	assert.Equal(t, TeamRole("member"), TeamRoleMember)
	assert.Equal(t, TeamRole("lead"), TeamRoleLead)
	assert.Equal(t, TeamRole("manager"), TeamRoleManager)
}

func TestNotificationChannel_JSON(t *testing.T) {
	channel := NotificationChannel{
		Type: "slack",
		Config: map[string]interface{}{
			"channel":   "#alerts",
			"workspace": "company-workspace",
		},
	}

	// Test marshaling
	data, err := json.Marshal(channel)
	require.NoError(t, err)

	// Test unmarshaling
	var parsed NotificationChannel
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, channel.Type, parsed.Type)
	assert.Equal(t, "#alerts", parsed.Config["channel"])
	assert.Equal(t, "company-workspace", parsed.Config["workspace"])
}

func TestTeam_WithDefaultChannel(t *testing.T) {
	store := NewMockTeamStore()
	ctx := context.Background()

	team := &Team{
		Name:        "Test Team",
		Description: "Team with default channel",
		DefaultChannel: &NotificationChannel{
			Type: "slack",
			Config: map[string]interface{}{
				"channel": "#team-alerts",
			},
		},
	}

	created, err := store.CreateTeam(ctx, team)
	require.NoError(t, err)
	require.NotNil(t, created)
	require.NotNil(t, created.DefaultChannel)
	assert.Equal(t, "slack", created.DefaultChannel.Type)
	assert.Equal(t, "#team-alerts", created.DefaultChannel.Config["channel"])
}

func TestTeam_WithEscalationPolicy(t *testing.T) {
	store := NewMockTeamStore()
	ctx := context.Background()

	policyID := uuid.New()

	team := &Team{
		Name:                      "Test Team",
		DefaultEscalationPolicyID: &policyID,
	}

	created, err := store.CreateTeam(ctx, team)
	require.NoError(t, err)
	require.NotNil(t, created)
	require.NotNil(t, created.DefaultEscalationPolicyID)
	assert.Equal(t, policyID, *created.DefaultEscalationPolicyID)
}
