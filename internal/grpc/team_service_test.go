package grpc

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/kneutral-org/alerting-system/internal/team"
	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// TestTeamStore is an in-memory implementation for testing the service.
type TestTeamStore struct {
	teams   map[string]*routingv1.Team
	counter int64
}

func NewTestTeamStore() *TestTeamStore {
	return &TestTeamStore{
		teams: make(map[string]*routingv1.Team),
	}
}

func (s *TestTeamStore) Create(ctx context.Context, t *routingv1.Team) (*routingv1.Team, error) {
	if t == nil {
		return nil, team.ErrInvalidTeam
	}

	if t.Name == "" {
		return nil, team.ErrInvalidTeam
	}

	// Check for duplicate name
	for _, existing := range s.teams {
		if existing.Name == t.Name {
			return nil, team.ErrDuplicateName
		}
	}

	if t.Id == "" {
		s.counter++
		t.Id = "team-" + string(rune(s.counter+'0'))
	}

	s.teams[t.Id] = t
	return t, nil
}

func (s *TestTeamStore) Get(ctx context.Context, id string) (*routingv1.Team, error) {
	t, ok := s.teams[id]
	if !ok {
		return nil, team.ErrNotFound
	}
	return t, nil
}

func (s *TestTeamStore) List(ctx context.Context, req *routingv1.ListTeamsRequest) (*routingv1.ListTeamsResponse, error) {
	var teams []*routingv1.Team
	for _, t := range s.teams {
		teams = append(teams, t)
	}
	return &routingv1.ListTeamsResponse{
		Teams:      teams,
		TotalCount: int32(len(teams)),
	}, nil
}

func (s *TestTeamStore) Update(ctx context.Context, t *routingv1.Team) (*routingv1.Team, error) {
	if t == nil || t.Id == "" {
		return nil, team.ErrInvalidTeam
	}

	if _, ok := s.teams[t.Id]; !ok {
		return nil, team.ErrNotFound
	}

	// Check for duplicate name
	for _, existing := range s.teams {
		if existing.Name == t.Name && existing.Id != t.Id {
			return nil, team.ErrDuplicateName
		}
	}

	s.teams[t.Id] = t
	return t, nil
}

func (s *TestTeamStore) Delete(ctx context.Context, id string) error {
	if _, ok := s.teams[id]; !ok {
		return team.ErrNotFound
	}
	delete(s.teams, id)
	return nil
}

func (s *TestTeamStore) AddMember(ctx context.Context, teamID string, member *routingv1.TeamMember) (*routingv1.Team, error) {
	t, ok := s.teams[teamID]
	if !ok {
		return nil, team.ErrNotFound
	}

	// Check if member already exists
	for _, m := range t.Members {
		if m.UserId == member.UserId {
			return nil, team.ErrMemberExists
		}
	}

	t.Members = append(t.Members, member)
	return t, nil
}

func (s *TestTeamStore) RemoveMember(ctx context.Context, teamID, userID string) (*routingv1.Team, error) {
	t, ok := s.teams[teamID]
	if !ok {
		return nil, team.ErrNotFound
	}

	found := false
	newMembers := make([]*routingv1.TeamMember, 0)
	for _, m := range t.Members {
		if m.UserId == userID {
			found = true
			continue
		}
		newMembers = append(newMembers, m)
	}

	if !found {
		return nil, team.ErrMemberNotFound
	}

	t.Members = newMembers
	return t, nil
}

func (s *TestTeamStore) UpdateMember(ctx context.Context, teamID string, member *routingv1.TeamMember) (*routingv1.Team, error) {
	t, ok := s.teams[teamID]
	if !ok {
		return nil, team.ErrNotFound
	}

	found := false
	for i, m := range t.Members {
		if m.UserId == member.UserId {
			t.Members[i] = member
			found = true
			break
		}
	}

	if !found {
		return nil, team.ErrMemberNotFound
	}

	return t, nil
}

func (s *TestTeamStore) GetByUser(ctx context.Context, userID string) ([]*routingv1.Team, error) {
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

// Ensure TestTeamStore implements Store
var _ team.Store = (*TestTeamStore)(nil)

// =============================================================================
// Service Tests
// =============================================================================

func newTestTeamService() *TeamService {
	store := NewTestTeamStore()
	logger := zerolog.Nop()
	return NewTeamService(store, logger)
}

func TestTeamService_CreateTeam(t *testing.T) {
	ctx := context.Background()
	svc := newTestTeamService()

	t.Run("create team successfully", func(t *testing.T) {
		req := &routingv1.CreateTeamRequest{
			Team: &routingv1.Team{
				Name:        "Test Team",
				Description: "A test team",
			},
		}

		resp, err := svc.CreateTeam(ctx, req)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if resp.Id == "" {
			t.Error("expected team ID to be set")
		}

		if resp.Name != "Test Team" {
			t.Errorf("expected name 'Test Team', got '%s'", resp.Name)
		}
	})

	t.Run("create team with nil request", func(t *testing.T) {
		req := &routingv1.CreateTeamRequest{Team: nil}

		_, err := svc.CreateTeam(ctx, req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status error, got %v", err)
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", st.Code())
		}
	})

	t.Run("create team without name", func(t *testing.T) {
		req := &routingv1.CreateTeamRequest{
			Team: &routingv1.Team{Description: "No name"},
		}

		_, err := svc.CreateTeam(ctx, req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status error, got %v", err)
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", st.Code())
		}
	})

	t.Run("create team with duplicate name", func(t *testing.T) {
		req := &routingv1.CreateTeamRequest{
			Team: &routingv1.Team{Name: "Test Team"},
		}

		_, err := svc.CreateTeam(ctx, req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status error, got %v", err)
		}

		if st.Code() != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists, got %v", st.Code())
		}
	})
}

func TestTeamService_GetTeam(t *testing.T) {
	ctx := context.Background()
	svc := newTestTeamService()

	// Create a team first
	_, _ = svc.CreateTeam(ctx, &routingv1.CreateTeamRequest{
		Team: &routingv1.Team{Id: "team-1", Name: "Test Team"},
	})

	t.Run("get existing team", func(t *testing.T) {
		resp, err := svc.GetTeam(ctx, &routingv1.GetTeamRequest{Id: "team-1"})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if resp.Name != "Test Team" {
			t.Errorf("expected name 'Test Team', got '%s'", resp.Name)
		}
	})

	t.Run("get non-existent team", func(t *testing.T) {
		_, err := svc.GetTeam(ctx, &routingv1.GetTeamRequest{Id: "non-existent"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status error, got %v", err)
		}

		if st.Code() != codes.NotFound {
			t.Errorf("expected NotFound, got %v", st.Code())
		}
	})

	t.Run("get team with empty id", func(t *testing.T) {
		_, err := svc.GetTeam(ctx, &routingv1.GetTeamRequest{Id: ""})
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status error, got %v", err)
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", st.Code())
		}
	})
}

func TestTeamService_ListTeams(t *testing.T) {
	ctx := context.Background()
	svc := newTestTeamService()

	// Create some teams
	_, _ = svc.CreateTeam(ctx, &routingv1.CreateTeamRequest{
		Team: &routingv1.Team{Id: "team-1", Name: "Team A"},
	})
	_, _ = svc.CreateTeam(ctx, &routingv1.CreateTeamRequest{
		Team: &routingv1.Team{Id: "team-2", Name: "Team B"},
	})

	t.Run("list all teams", func(t *testing.T) {
		resp, err := svc.ListTeams(ctx, &routingv1.ListTeamsRequest{})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if resp.TotalCount != 2 {
			t.Errorf("expected 2 teams, got %d", resp.TotalCount)
		}
	})
}

func TestTeamService_UpdateTeam(t *testing.T) {
	ctx := context.Background()
	svc := newTestTeamService()

	// Create a team first
	_, _ = svc.CreateTeam(ctx, &routingv1.CreateTeamRequest{
		Team: &routingv1.Team{Id: "team-1", Name: "Test Team"},
	})

	t.Run("update existing team", func(t *testing.T) {
		req := &routingv1.UpdateTeamRequest{
			Team: &routingv1.Team{
				Id:          "team-1",
				Name:        "Updated Team",
				Description: "Updated description",
			},
		}

		resp, err := svc.UpdateTeam(ctx, req)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if resp.Name != "Updated Team" {
			t.Errorf("expected name 'Updated Team', got '%s'", resp.Name)
		}
	})

	t.Run("update non-existent team", func(t *testing.T) {
		req := &routingv1.UpdateTeamRequest{
			Team: &routingv1.Team{Id: "non-existent", Name: "Test"},
		}

		_, err := svc.UpdateTeam(ctx, req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status error, got %v", err)
		}

		if st.Code() != codes.NotFound {
			t.Errorf("expected NotFound, got %v", st.Code())
		}
	})

	t.Run("update with nil team", func(t *testing.T) {
		req := &routingv1.UpdateTeamRequest{Team: nil}

		_, err := svc.UpdateTeam(ctx, req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status error, got %v", err)
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", st.Code())
		}
	})
}

func TestTeamService_DeleteTeam(t *testing.T) {
	ctx := context.Background()
	svc := newTestTeamService()

	// Create a team first
	_, _ = svc.CreateTeam(ctx, &routingv1.CreateTeamRequest{
		Team: &routingv1.Team{Id: "team-1", Name: "Test Team"},
	})

	t.Run("delete existing team", func(t *testing.T) {
		resp, err := svc.DeleteTeam(ctx, &routingv1.DeleteTeamRequest{Id: "team-1"})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if !resp.Success {
			t.Error("expected Success to be true")
		}

		// Verify it's deleted
		_, err = svc.GetTeam(ctx, &routingv1.GetTeamRequest{Id: "team-1"})
		if err == nil {
			t.Error("expected error after delete, got nil")
		}
	})

	t.Run("delete non-existent team", func(t *testing.T) {
		_, err := svc.DeleteTeam(ctx, &routingv1.DeleteTeamRequest{Id: "non-existent"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status error, got %v", err)
		}

		if st.Code() != codes.NotFound {
			t.Errorf("expected NotFound, got %v", st.Code())
		}
	})

	t.Run("delete with empty id", func(t *testing.T) {
		_, err := svc.DeleteTeam(ctx, &routingv1.DeleteTeamRequest{Id: ""})
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status error, got %v", err)
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", st.Code())
		}
	})
}

func TestTeamService_AddTeamMember(t *testing.T) {
	ctx := context.Background()
	svc := newTestTeamService()

	// Create a team first
	_, _ = svc.CreateTeam(ctx, &routingv1.CreateTeamRequest{
		Team: &routingv1.Team{Id: "team-1", Name: "Test Team"},
	})

	t.Run("add member successfully", func(t *testing.T) {
		req := &routingv1.AddTeamMemberRequest{
			TeamId: "team-1",
			Member: &routingv1.TeamMember{
				UserId: "user-1",
				Role:   routingv1.TeamRole_TEAM_ROLE_MEMBER,
			},
		}

		resp, err := svc.AddTeamMember(ctx, req)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(resp.Members) != 1 {
			t.Errorf("expected 1 member, got %d", len(resp.Members))
		}
	})

	t.Run("add duplicate member", func(t *testing.T) {
		req := &routingv1.AddTeamMemberRequest{
			TeamId: "team-1",
			Member: &routingv1.TeamMember{
				UserId: "user-1",
				Role:   routingv1.TeamRole_TEAM_ROLE_MEMBER,
			},
		}

		_, err := svc.AddTeamMember(ctx, req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status error, got %v", err)
		}

		if st.Code() != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists, got %v", st.Code())
		}
	})

	t.Run("add member to non-existent team", func(t *testing.T) {
		req := &routingv1.AddTeamMemberRequest{
			TeamId: "non-existent",
			Member: &routingv1.TeamMember{UserId: "user-2"},
		}

		_, err := svc.AddTeamMember(ctx, req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status error, got %v", err)
		}

		if st.Code() != codes.NotFound {
			t.Errorf("expected NotFound, got %v", st.Code())
		}
	})

	t.Run("add member with empty team_id", func(t *testing.T) {
		req := &routingv1.AddTeamMemberRequest{
			TeamId: "",
			Member: &routingv1.TeamMember{UserId: "user-2"},
		}

		_, err := svc.AddTeamMember(ctx, req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status error, got %v", err)
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", st.Code())
		}
	})

	t.Run("add nil member", func(t *testing.T) {
		req := &routingv1.AddTeamMemberRequest{
			TeamId: "team-1",
			Member: nil,
		}

		_, err := svc.AddTeamMember(ctx, req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status error, got %v", err)
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", st.Code())
		}
	})
}

func TestTeamService_RemoveTeamMember(t *testing.T) {
	ctx := context.Background()
	svc := newTestTeamService()

	// Create a team with a member
	_, _ = svc.CreateTeam(ctx, &routingv1.CreateTeamRequest{
		Team: &routingv1.Team{
			Id:   "team-1",
			Name: "Test Team",
			Members: []*routingv1.TeamMember{
				{UserId: "user-1", Role: routingv1.TeamRole_TEAM_ROLE_MEMBER},
			},
		},
	})

	t.Run("remove existing member", func(t *testing.T) {
		req := &routingv1.RemoveTeamMemberRequest{
			TeamId: "team-1",
			UserId: "user-1",
		}

		resp, err := svc.RemoveTeamMember(ctx, req)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(resp.Members) != 0 {
			t.Errorf("expected 0 members, got %d", len(resp.Members))
		}
	})

	t.Run("remove non-existent member", func(t *testing.T) {
		req := &routingv1.RemoveTeamMemberRequest{
			TeamId: "team-1",
			UserId: "non-existent",
		}

		_, err := svc.RemoveTeamMember(ctx, req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status error, got %v", err)
		}

		if st.Code() != codes.NotFound {
			t.Errorf("expected NotFound, got %v", st.Code())
		}
	})

	t.Run("remove from non-existent team", func(t *testing.T) {
		req := &routingv1.RemoveTeamMemberRequest{
			TeamId: "non-existent",
			UserId: "user-1",
		}

		_, err := svc.RemoveTeamMember(ctx, req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status error, got %v", err)
		}

		if st.Code() != codes.NotFound {
			t.Errorf("expected NotFound, got %v", st.Code())
		}
	})
}

func TestTeamService_UpdateTeamMember(t *testing.T) {
	ctx := context.Background()
	svc := newTestTeamService()

	// Create a team with a member
	_, _ = svc.CreateTeam(ctx, &routingv1.CreateTeamRequest{
		Team: &routingv1.Team{
			Id:   "team-1",
			Name: "Test Team",
			Members: []*routingv1.TeamMember{
				{UserId: "user-1", Role: routingv1.TeamRole_TEAM_ROLE_MEMBER},
			},
		},
	})

	t.Run("update member role", func(t *testing.T) {
		req := &routingv1.UpdateTeamMemberRequest{
			TeamId: "team-1",
			Member: &routingv1.TeamMember{
				UserId: "user-1",
				Role:   routingv1.TeamRole_TEAM_ROLE_MANAGER,
			},
		}

		resp, err := svc.UpdateTeamMember(ctx, req)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if resp.Members[0].Role != routingv1.TeamRole_TEAM_ROLE_MANAGER {
			t.Errorf("expected role MANAGER, got %v", resp.Members[0].Role)
		}
	})

	t.Run("update non-existent member", func(t *testing.T) {
		req := &routingv1.UpdateTeamMemberRequest{
			TeamId: "team-1",
			Member: &routingv1.TeamMember{UserId: "non-existent"},
		}

		_, err := svc.UpdateTeamMember(ctx, req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status error, got %v", err)
		}

		if st.Code() != codes.NotFound {
			t.Errorf("expected NotFound, got %v", st.Code())
		}
	})

	t.Run("update with empty team_id", func(t *testing.T) {
		req := &routingv1.UpdateTeamMemberRequest{
			TeamId: "",
			Member: &routingv1.TeamMember{UserId: "user-1"},
		}

		_, err := svc.UpdateTeamMember(ctx, req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status error, got %v", err)
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", st.Code())
		}
	})

	t.Run("update with nil member", func(t *testing.T) {
		req := &routingv1.UpdateTeamMemberRequest{
			TeamId: "team-1",
			Member: nil,
		}

		_, err := svc.UpdateTeamMember(ctx, req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status error, got %v", err)
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", st.Code())
		}
	})
}

func TestTeamService_GetUserTeams(t *testing.T) {
	ctx := context.Background()
	svc := newTestTeamService()

	// Create teams with members
	_, _ = svc.CreateTeam(ctx, &routingv1.CreateTeamRequest{
		Team: &routingv1.Team{
			Id:   "team-1",
			Name: "Team A",
			Members: []*routingv1.TeamMember{
				{UserId: "user-1"},
				{UserId: "user-2"},
			},
		},
	})
	_, _ = svc.CreateTeam(ctx, &routingv1.CreateTeamRequest{
		Team: &routingv1.Team{
			Id:   "team-2",
			Name: "Team B",
			Members: []*routingv1.TeamMember{
				{UserId: "user-1"},
			},
		},
	})

	t.Run("get teams for user with multiple teams", func(t *testing.T) {
		resp, err := svc.GetUserTeams(ctx, &routingv1.GetUserTeamsRequest{UserId: "user-1"})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if resp.TotalCount != 2 {
			t.Errorf("expected 2 teams, got %d", resp.TotalCount)
		}
	})

	t.Run("get teams for user with one team", func(t *testing.T) {
		resp, err := svc.GetUserTeams(ctx, &routingv1.GetUserTeamsRequest{UserId: "user-2"})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if resp.TotalCount != 1 {
			t.Errorf("expected 1 team, got %d", resp.TotalCount)
		}
	})

	t.Run("get teams for user with no teams", func(t *testing.T) {
		resp, err := svc.GetUserTeams(ctx, &routingv1.GetUserTeamsRequest{UserId: "user-3"})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if resp.TotalCount != 0 {
			t.Errorf("expected 0 teams, got %d", resp.TotalCount)
		}
	})

	t.Run("get teams with empty user_id", func(t *testing.T) {
		_, err := svc.GetUserTeams(ctx, &routingv1.GetUserTeamsRequest{UserId: ""})
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status error, got %v", err)
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", st.Code())
		}
	})
}

// Benchmark tests
func BenchmarkTeamService_CreateTeam(b *testing.B) {
	ctx := context.Background()
	svc := newTestTeamService()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := &routingv1.CreateTeamRequest{
			Team: &routingv1.Team{
				Name: "Team " + string(rune(i)),
			},
		}
		_, _ = svc.CreateTeam(ctx, req)
	}
}

func BenchmarkTeamService_GetTeam(b *testing.B) {
	ctx := context.Background()
	svc := newTestTeamService()

	// Create a team first
	_, _ = svc.CreateTeam(ctx, &routingv1.CreateTeamRequest{
		Team: &routingv1.Team{Id: "team-1", Name: "Test Team"},
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.GetTeam(ctx, &routingv1.GetTeamRequest{Id: "team-1"})
	}
}
