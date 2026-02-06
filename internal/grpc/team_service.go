// Package grpc provides gRPC service implementations.
package grpc

import (
	"context"
	"errors"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/kneutral-org/alerting-system/internal/team"
	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// TeamService implements the TeamServiceServer interface.
type TeamService struct {
	routingv1.UnimplementedTeamServiceServer
	store  team.Store
	logger zerolog.Logger
}

// NewTeamService creates a new TeamService.
func NewTeamService(store team.Store, logger zerolog.Logger) *TeamService {
	return &TeamService{
		store:  store,
		logger: logger.With().Str("service", "team").Logger(),
	}
}

// =============================================================================
// Team CRUD (5 RPCs)
// =============================================================================

// CreateTeam creates a new team.
func (s *TeamService) CreateTeam(ctx context.Context, req *routingv1.CreateTeamRequest) (*routingv1.Team, error) {
	if req.Team == nil {
		return nil, status.Error(codes.InvalidArgument, "team is required")
	}

	if req.Team.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "team name is required")
	}

	s.logger.Info().
		Str("name", req.Team.Name).
		Int("memberCount", len(req.Team.Members)).
		Msg("creating team")

	t, err := s.store.Create(ctx, req.Team)
	if err != nil {
		if errors.Is(err, team.ErrDuplicateName) {
			return nil, status.Error(codes.AlreadyExists, "team name already exists")
		}
		if errors.Is(err, team.ErrInvalidTeam) {
			return nil, status.Errorf(codes.InvalidArgument, "invalid team: %v", err)
		}
		s.logger.Error().Err(err).Msg("failed to create team")
		return nil, status.Error(codes.Internal, "failed to create team")
	}

	s.logger.Info().
		Str("id", t.Id).
		Str("name", t.Name).
		Msg("team created")

	return t, nil
}

// GetTeam retrieves a team by ID.
func (s *TeamService) GetTeam(ctx context.Context, req *routingv1.GetTeamRequest) (*routingv1.Team, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	t, err := s.store.Get(ctx, req.Id)
	if err != nil {
		if errors.Is(err, team.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "team not found")
		}
		s.logger.Error().Err(err).Str("id", req.Id).Msg("failed to get team")
		return nil, status.Error(codes.Internal, "failed to get team")
	}

	return t, nil
}

// ListTeams retrieves teams with optional filters.
func (s *TeamService) ListTeams(ctx context.Context, req *routingv1.ListTeamsRequest) (*routingv1.ListTeamsResponse, error) {
	resp, err := s.store.List(ctx, req)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to list teams")
		return nil, status.Error(codes.Internal, "failed to list teams")
	}

	return resp, nil
}

// UpdateTeam updates an existing team.
func (s *TeamService) UpdateTeam(ctx context.Context, req *routingv1.UpdateTeamRequest) (*routingv1.Team, error) {
	if req.Team == nil || req.Team.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "team with id is required")
	}

	s.logger.Info().
		Str("id", req.Team.Id).
		Str("name", req.Team.Name).
		Msg("updating team")

	t, err := s.store.Update(ctx, req.Team)
	if err != nil {
		if errors.Is(err, team.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "team not found")
		}
		if errors.Is(err, team.ErrDuplicateName) {
			return nil, status.Error(codes.AlreadyExists, "team name already exists")
		}
		s.logger.Error().Err(err).Str("id", req.Team.Id).Msg("failed to update team")
		return nil, status.Error(codes.Internal, "failed to update team")
	}

	s.logger.Info().
		Str("id", t.Id).
		Msg("team updated")

	return t, nil
}

// DeleteTeam deletes a team by ID.
func (s *TeamService) DeleteTeam(ctx context.Context, req *routingv1.DeleteTeamRequest) (*routingv1.DeleteTeamResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	s.logger.Info().Str("id", req.Id).Msg("deleting team")

	err := s.store.Delete(ctx, req.Id)
	if err != nil {
		if errors.Is(err, team.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "team not found")
		}
		s.logger.Error().Err(err).Str("id", req.Id).Msg("failed to delete team")
		return nil, status.Error(codes.Internal, "failed to delete team")
	}

	s.logger.Info().Str("id", req.Id).Msg("team deleted")

	return &routingv1.DeleteTeamResponse{Success: true}, nil
}

// =============================================================================
// Member management (3 RPCs)
// =============================================================================

// AddTeamMember adds a member to a team.
func (s *TeamService) AddTeamMember(ctx context.Context, req *routingv1.AddTeamMemberRequest) (*routingv1.Team, error) {
	if req.TeamId == "" {
		return nil, status.Error(codes.InvalidArgument, "team_id is required")
	}

	if req.Member == nil {
		return nil, status.Error(codes.InvalidArgument, "member is required")
	}

	if req.Member.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "member user_id is required")
	}

	s.logger.Info().
		Str("teamId", req.TeamId).
		Str("userId", req.Member.UserId).
		Str("role", req.Member.Role.String()).
		Msg("adding team member")

	t, err := s.store.AddMember(ctx, req.TeamId, req.Member)
	if err != nil {
		if errors.Is(err, team.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "team not found")
		}
		if errors.Is(err, team.ErrMemberExists) {
			return nil, status.Error(codes.AlreadyExists, "member already exists in team")
		}
		s.logger.Error().Err(err).Str("teamId", req.TeamId).Msg("failed to add team member")
		return nil, status.Error(codes.Internal, "failed to add team member")
	}

	s.logger.Info().
		Str("teamId", req.TeamId).
		Str("userId", req.Member.UserId).
		Msg("team member added")

	return t, nil
}

// RemoveTeamMember removes a member from a team.
func (s *TeamService) RemoveTeamMember(ctx context.Context, req *routingv1.RemoveTeamMemberRequest) (*routingv1.Team, error) {
	if req.TeamId == "" {
		return nil, status.Error(codes.InvalidArgument, "team_id is required")
	}

	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	s.logger.Info().
		Str("teamId", req.TeamId).
		Str("userId", req.UserId).
		Msg("removing team member")

	t, err := s.store.RemoveMember(ctx, req.TeamId, req.UserId)
	if err != nil {
		if errors.Is(err, team.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "team not found")
		}
		if errors.Is(err, team.ErrMemberNotFound) {
			return nil, status.Error(codes.NotFound, "member not found in team")
		}
		s.logger.Error().Err(err).Str("teamId", req.TeamId).Msg("failed to remove team member")
		return nil, status.Error(codes.Internal, "failed to remove team member")
	}

	s.logger.Info().
		Str("teamId", req.TeamId).
		Str("userId", req.UserId).
		Msg("team member removed")

	return t, nil
}

// UpdateTeamMember updates a member's role in a team.
func (s *TeamService) UpdateTeamMember(ctx context.Context, req *routingv1.UpdateTeamMemberRequest) (*routingv1.Team, error) {
	if req.TeamId == "" {
		return nil, status.Error(codes.InvalidArgument, "team_id is required")
	}

	if req.Member == nil || req.Member.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "member with user_id is required")
	}

	s.logger.Info().
		Str("teamId", req.TeamId).
		Str("userId", req.Member.UserId).
		Str("newRole", req.Member.Role.String()).
		Msg("updating team member")

	t, err := s.store.UpdateMember(ctx, req.TeamId, req.Member)
	if err != nil {
		if errors.Is(err, team.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "team not found")
		}
		if errors.Is(err, team.ErrMemberNotFound) {
			return nil, status.Error(codes.NotFound, "member not found in team")
		}
		s.logger.Error().Err(err).Str("teamId", req.TeamId).Msg("failed to update team member")
		return nil, status.Error(codes.Internal, "failed to update team member")
	}

	s.logger.Info().
		Str("teamId", req.TeamId).
		Str("userId", req.Member.UserId).
		Msg("team member updated")

	return t, nil
}

// =============================================================================
// User Teams (1 RPC)
// =============================================================================

// GetUserTeams retrieves all teams that a user is a member of.
func (s *TeamService) GetUserTeams(ctx context.Context, req *routingv1.GetUserTeamsRequest) (*routingv1.ListTeamsResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	teams, err := s.store.GetByUser(ctx, req.UserId)
	if err != nil {
		s.logger.Error().Err(err).Str("userId", req.UserId).Msg("failed to get user teams")
		return nil, status.Error(codes.Internal, "failed to get user teams")
	}

	return &routingv1.ListTeamsResponse{
		Teams:      teams,
		TotalCount: int32(len(teams)),
	}, nil
}

// Ensure TeamService implements the interface
var _ routingv1.TeamServiceServer = (*TeamService)(nil)
