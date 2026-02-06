package team

import (
	"context"
	"testing"
	"time"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// InMemoryStore is an in-memory implementation for testing.
type InMemoryStore struct {
	teams   map[string]*routingv1.Team
	counter int64
}

// NewInMemoryStore creates a new InMemoryStore.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		teams: make(map[string]*routingv1.Team),
	}
}

func (s *InMemoryStore) Create(ctx context.Context, team *routingv1.Team) (*routingv1.Team, error) {
	if team == nil {
		return nil, ErrInvalidTeam
	}

	if team.Name == "" {
		return nil, ErrInvalidTeam
	}

	// Check for duplicate name
	for _, t := range s.teams {
		if t.Name == team.Name {
			return nil, ErrDuplicateName
		}
	}

	if team.Id == "" {
		s.counter++
		team.Id = "team-" + string(rune(s.counter+'0'))
	}

	s.teams[team.Id] = team
	return team, nil
}

func (s *InMemoryStore) Get(ctx context.Context, id string) (*routingv1.Team, error) {
	team, ok := s.teams[id]
	if !ok {
		return nil, ErrNotFound
	}
	return team, nil
}

func (s *InMemoryStore) List(ctx context.Context, req *routingv1.ListTeamsRequest) (*routingv1.ListTeamsResponse, error) {
	var teams []*routingv1.Team
	for _, t := range s.teams {
		teams = append(teams, t)
	}
	return &routingv1.ListTeamsResponse{
		Teams:      teams,
		TotalCount: int32(len(teams)),
	}, nil
}

func (s *InMemoryStore) Update(ctx context.Context, team *routingv1.Team) (*routingv1.Team, error) {
	if team == nil || team.Id == "" {
		return nil, ErrInvalidTeam
	}

	if _, ok := s.teams[team.Id]; !ok {
		return nil, ErrNotFound
	}

	// Check for duplicate name
	for _, t := range s.teams {
		if t.Name == team.Name && t.Id != team.Id {
			return nil, ErrDuplicateName
		}
	}

	s.teams[team.Id] = team
	return team, nil
}

func (s *InMemoryStore) Delete(ctx context.Context, id string) error {
	if _, ok := s.teams[id]; !ok {
		return ErrNotFound
	}
	delete(s.teams, id)
	return nil
}

func (s *InMemoryStore) AddMember(ctx context.Context, teamID string, member *routingv1.TeamMember) (*routingv1.Team, error) {
	team, ok := s.teams[teamID]
	if !ok {
		return nil, ErrNotFound
	}

	// Check if member already exists
	for _, m := range team.Members {
		if m.UserId == member.UserId {
			return nil, ErrMemberExists
		}
	}

	team.Members = append(team.Members, member)
	return team, nil
}

func (s *InMemoryStore) RemoveMember(ctx context.Context, teamID, userID string) (*routingv1.Team, error) {
	team, ok := s.teams[teamID]
	if !ok {
		return nil, ErrNotFound
	}

	found := false
	newMembers := make([]*routingv1.TeamMember, 0)
	for _, m := range team.Members {
		if m.UserId == userID {
			found = true
			continue
		}
		newMembers = append(newMembers, m)
	}

	if !found {
		return nil, ErrMemberNotFound
	}

	team.Members = newMembers
	return team, nil
}

func (s *InMemoryStore) UpdateMember(ctx context.Context, teamID string, member *routingv1.TeamMember) (*routingv1.Team, error) {
	team, ok := s.teams[teamID]
	if !ok {
		return nil, ErrNotFound
	}

	found := false
	for i, m := range team.Members {
		if m.UserId == member.UserId {
			team.Members[i] = member
			found = true
			break
		}
	}

	if !found {
		return nil, ErrMemberNotFound
	}

	return team, nil
}

func (s *InMemoryStore) GetByUser(ctx context.Context, userID string) ([]*routingv1.Team, error) {
	var teams []*routingv1.Team
	for _, t := range s.teams {
		for _, m := range t.Members {
			if m.UserId == userID {
				teams = append(teams, t)
				break
			}
		}
	}
	return teams, nil
}

// Ensure InMemoryStore implements Store
var _ Store = (*InMemoryStore)(nil)

// =============================================================================
// Tests
// =============================================================================

func TestInMemoryStore_Create(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()

	t.Run("create team successfully", func(t *testing.T) {
		team := &routingv1.Team{
			Name:        "Test Team",
			Description: "A test team",
		}

		created, err := store.Create(ctx, team)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if created.Id == "" {
			t.Error("expected team ID to be set")
		}

		if created.Name != "Test Team" {
			t.Errorf("expected name 'Test Team', got '%s'", created.Name)
		}
	})

	t.Run("create nil team", func(t *testing.T) {
		_, err := store.Create(ctx, nil)
		if err != ErrInvalidTeam {
			t.Errorf("expected ErrInvalidTeam, got %v", err)
		}
	})

	t.Run("create team without name", func(t *testing.T) {
		team := &routingv1.Team{}
		_, err := store.Create(ctx, team)
		if err != ErrInvalidTeam {
			t.Errorf("expected ErrInvalidTeam, got %v", err)
		}
	})

	t.Run("create team with duplicate name", func(t *testing.T) {
		team := &routingv1.Team{Name: "Test Team"}
		_, err := store.Create(ctx, team)
		if err != ErrDuplicateName {
			t.Errorf("expected ErrDuplicateName, got %v", err)
		}
	})
}

func TestInMemoryStore_Get(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()

	// Create a team first
	team := &routingv1.Team{
		Id:   "team-1",
		Name: "Test Team",
	}
	_, _ = store.Create(ctx, team)

	t.Run("get existing team", func(t *testing.T) {
		got, err := store.Get(ctx, "team-1")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if got.Name != "Test Team" {
			t.Errorf("expected name 'Test Team', got '%s'", got.Name)
		}
	})

	t.Run("get non-existent team", func(t *testing.T) {
		_, err := store.Get(ctx, "non-existent")
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestInMemoryStore_List(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()

	// Create some teams
	_, _ = store.Create(ctx, &routingv1.Team{Id: "team-1", Name: "Team A"})
	_, _ = store.Create(ctx, &routingv1.Team{Id: "team-2", Name: "Team B"})

	t.Run("list all teams", func(t *testing.T) {
		resp, err := store.List(ctx, &routingv1.ListTeamsRequest{})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if resp.TotalCount != 2 {
			t.Errorf("expected 2 teams, got %d", resp.TotalCount)
		}
	})
}

func TestInMemoryStore_Update(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()

	// Create a team first
	_, _ = store.Create(ctx, &routingv1.Team{Id: "team-1", Name: "Test Team"})

	t.Run("update existing team", func(t *testing.T) {
		updated, err := store.Update(ctx, &routingv1.Team{
			Id:          "team-1",
			Name:        "Updated Team",
			Description: "Updated description",
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if updated.Name != "Updated Team" {
			t.Errorf("expected name 'Updated Team', got '%s'", updated.Name)
		}
	})

	t.Run("update non-existent team", func(t *testing.T) {
		_, err := store.Update(ctx, &routingv1.Team{Id: "non-existent", Name: "Test"})
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("update with nil team", func(t *testing.T) {
		_, err := store.Update(ctx, nil)
		if err != ErrInvalidTeam {
			t.Errorf("expected ErrInvalidTeam, got %v", err)
		}
	})
}

func TestInMemoryStore_Delete(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()

	// Create a team first
	_, _ = store.Create(ctx, &routingv1.Team{Id: "team-1", Name: "Test Team"})

	t.Run("delete existing team", func(t *testing.T) {
		err := store.Delete(ctx, "team-1")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Verify it's deleted
		_, err = store.Get(ctx, "team-1")
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound after delete, got %v", err)
		}
	})

	t.Run("delete non-existent team", func(t *testing.T) {
		err := store.Delete(ctx, "non-existent")
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestInMemoryStore_AddMember(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()

	// Create a team first
	_, _ = store.Create(ctx, &routingv1.Team{Id: "team-1", Name: "Test Team"})

	t.Run("add member successfully", func(t *testing.T) {
		member := &routingv1.TeamMember{
			UserId: "user-1",
			Role:   routingv1.TeamRole_TEAM_ROLE_MEMBER,
		}

		team, err := store.AddMember(ctx, "team-1", member)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(team.Members) != 1 {
			t.Errorf("expected 1 member, got %d", len(team.Members))
		}
	})

	t.Run("add duplicate member", func(t *testing.T) {
		member := &routingv1.TeamMember{
			UserId: "user-1",
			Role:   routingv1.TeamRole_TEAM_ROLE_MEMBER,
		}

		_, err := store.AddMember(ctx, "team-1", member)
		if err != ErrMemberExists {
			t.Errorf("expected ErrMemberExists, got %v", err)
		}
	})

	t.Run("add member to non-existent team", func(t *testing.T) {
		member := &routingv1.TeamMember{UserId: "user-2"}
		_, err := store.AddMember(ctx, "non-existent", member)
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestInMemoryStore_RemoveMember(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()

	// Create a team with a member
	team := &routingv1.Team{
		Id:   "team-1",
		Name: "Test Team",
		Members: []*routingv1.TeamMember{
			{UserId: "user-1", Role: routingv1.TeamRole_TEAM_ROLE_MEMBER},
		},
	}
	_, _ = store.Create(ctx, team)

	t.Run("remove existing member", func(t *testing.T) {
		updated, err := store.RemoveMember(ctx, "team-1", "user-1")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(updated.Members) != 0 {
			t.Errorf("expected 0 members, got %d", len(updated.Members))
		}
	})

	t.Run("remove non-existent member", func(t *testing.T) {
		_, err := store.RemoveMember(ctx, "team-1", "non-existent")
		if err != ErrMemberNotFound {
			t.Errorf("expected ErrMemberNotFound, got %v", err)
		}
	})

	t.Run("remove from non-existent team", func(t *testing.T) {
		_, err := store.RemoveMember(ctx, "non-existent", "user-1")
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestInMemoryStore_UpdateMember(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()

	// Create a team with a member
	team := &routingv1.Team{
		Id:   "team-1",
		Name: "Test Team",
		Members: []*routingv1.TeamMember{
			{UserId: "user-1", Role: routingv1.TeamRole_TEAM_ROLE_MEMBER},
		},
	}
	_, _ = store.Create(ctx, team)

	t.Run("update member role", func(t *testing.T) {
		member := &routingv1.TeamMember{
			UserId: "user-1",
			Role:   routingv1.TeamRole_TEAM_ROLE_MANAGER,
		}

		updated, err := store.UpdateMember(ctx, "team-1", member)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if updated.Members[0].Role != routingv1.TeamRole_TEAM_ROLE_MANAGER {
			t.Errorf("expected role MANAGER, got %v", updated.Members[0].Role)
		}
	})

	t.Run("update non-existent member", func(t *testing.T) {
		member := &routingv1.TeamMember{UserId: "non-existent"}
		_, err := store.UpdateMember(ctx, "team-1", member)
		if err != ErrMemberNotFound {
			t.Errorf("expected ErrMemberNotFound, got %v", err)
		}
	})
}

func TestInMemoryStore_GetByUser(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()

	// Create teams with members
	team1 := &routingv1.Team{
		Id:   "team-1",
		Name: "Team A",
		Members: []*routingv1.TeamMember{
			{UserId: "user-1"},
			{UserId: "user-2"},
		},
	}
	team2 := &routingv1.Team{
		Id:   "team-2",
		Name: "Team B",
		Members: []*routingv1.TeamMember{
			{UserId: "user-1"},
		},
	}
	_, _ = store.Create(ctx, team1)
	_, _ = store.Create(ctx, team2)

	t.Run("get teams for user with multiple teams", func(t *testing.T) {
		teams, err := store.GetByUser(ctx, "user-1")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(teams) != 2 {
			t.Errorf("expected 2 teams, got %d", len(teams))
		}
	})

	t.Run("get teams for user with one team", func(t *testing.T) {
		teams, err := store.GetByUser(ctx, "user-2")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(teams) != 1 {
			t.Errorf("expected 1 team, got %d", len(teams))
		}
	})

	t.Run("get teams for user with no teams", func(t *testing.T) {
		teams, err := store.GetByUser(ctx, "user-3")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(teams) != 0 {
			t.Errorf("expected 0 teams, got %d", len(teams))
		}
	})
}

func TestRoleConversion(t *testing.T) {
	tests := []struct {
		role     routingv1.TeamRole
		expected string
	}{
		{routingv1.TeamRole_TEAM_ROLE_MANAGER, "admin"},
		{routingv1.TeamRole_TEAM_ROLE_LEAD, "admin"},
		{routingv1.TeamRole_TEAM_ROLE_MEMBER, "member"},
		{routingv1.TeamRole_TEAM_ROLE_UNSPECIFIED, "member"},
	}

	for _, tt := range tests {
		t.Run(tt.role.String(), func(t *testing.T) {
			got := roleToString(tt.role)
			if got != tt.expected {
				t.Errorf("roleToString(%v) = %s, want %s", tt.role, got, tt.expected)
			}
		})
	}
}

func TestParseRole(t *testing.T) {
	tests := []struct {
		input    string
		expected routingv1.TeamRole
	}{
		{"admin", routingv1.TeamRole_TEAM_ROLE_MANAGER},
		{"member", routingv1.TeamRole_TEAM_ROLE_MEMBER},
		{"observer", routingv1.TeamRole_TEAM_ROLE_MEMBER},
		{"unknown", routingv1.TeamRole_TEAM_ROLE_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseRole(tt.input)
			if got != tt.expected {
				t.Errorf("parseRole(%s) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestPageTokenEncoding(t *testing.T) {
	t.Run("encode and decode", func(t *testing.T) {
		offset := 50
		token := encodePageToken(offset)
		decoded := decodePageToken(token)

		if decoded != offset {
			t.Errorf("expected offset %d, got %d", offset, decoded)
		}
	})

	t.Run("decode empty token", func(t *testing.T) {
		decoded := decodePageToken("")
		if decoded != 0 {
			t.Errorf("expected 0 for empty token, got %d", decoded)
		}
	})
}

// Benchmark tests
func BenchmarkInMemoryStore_Create(b *testing.B) {
	ctx := context.Background()
	store := NewInMemoryStore()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		team := &routingv1.Team{
			Name: "Team " + time.Now().String(),
		}
		_, _ = store.Create(ctx, team)
	}
}

func BenchmarkInMemoryStore_Get(b *testing.B) {
	ctx := context.Background()
	store := NewInMemoryStore()

	// Create a team first
	_, _ = store.Create(ctx, &routingv1.Team{Id: "team-1", Name: "Test Team"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Get(ctx, "team-1")
	}
}
