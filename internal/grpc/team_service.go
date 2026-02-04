// Package grpc provides gRPC service implementations for the alerting system.
package grpc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kneutral-org/alerting-system/internal/team"
	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// TeamServiceServer implements the gRPC TeamService.
type TeamServiceServer struct {
	routingv1.UnimplementedTeamServiceServer
	store team.TeamStore
}

// NewTeamServiceServer creates a new TeamServiceServer.
func NewTeamServiceServer(store team.TeamStore) *TeamServiceServer {
	return &TeamServiceServer{
		store: store,
	}
}

// CreateTeam creates a new team.
func (s *TeamServiceServer) CreateTeam(ctx context.Context, req *CreateTeamRequest) (*routingv1.Team, error) {
	if req.GetTeam() == nil {
		return nil, status.Error(codes.InvalidArgument, "team is required")
	}

	protoTeam := req.GetTeam()
	if protoTeam.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "team name is required")
	}

	internalTeam := protoTeamToInternal(protoTeam)

	created, err := s.store.CreateTeam(ctx, internalTeam)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create team: %v", err)
	}

	return internalTeamToProto(created), nil
}

// GetTeam retrieves a team by ID.
func (s *TeamServiceServer) GetTeam(ctx context.Context, req *GetTeamRequest) (*routingv1.Team, error) {
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "team id is required")
	}

	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid team id: %v", err)
	}

	t, err := s.store.GetTeam(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get team: %v", err)
	}
	if t == nil {
		return nil, status.Error(codes.NotFound, "team not found")
	}

	// Fetch team members
	members, err := s.store.GetTeamMembers(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get team members: %v", err)
	}

	protoTeam := internalTeamToProto(t)
	protoTeam.Members = internalTeamMembersToProto(members)

	return protoTeam, nil
}

// ListTeams lists teams with optional filters.
func (s *TeamServiceServer) ListTeams(ctx context.Context, req *ListTeamsRequest) (*routingv1.ListTeamsResponse, error) {
	params := team.ListTeamsParams{
		NameFilter: req.GetNameContains(),
		Limit:      req.GetPageSize(),
	}

	if req.GetSiteId() != "" {
		params.SitesFilter = []string{req.GetSiteId()}
	}

	// Handle pagination token (for simplicity, using offset-based pagination)
	// In production, you might want cursor-based pagination
	if params.Limit <= 0 {
		params.Limit = 100
	}

	teams, err := s.store.ListTeams(ctx, params)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list teams: %v", err)
	}

	protoTeams := make([]*routingv1.Team, 0, len(teams))
	for _, t := range teams {
		protoTeams = append(protoTeams, internalTeamToProto(t))
	}

	return &routingv1.ListTeamsResponse{
		Teams:      protoTeams,
		TotalCount: int32(len(protoTeams)),
	}, nil
}

// UpdateTeam updates an existing team.
func (s *TeamServiceServer) UpdateTeam(ctx context.Context, req *UpdateTeamRequest) (*routingv1.Team, error) {
	if req.GetTeam() == nil {
		return nil, status.Error(codes.InvalidArgument, "team is required")
	}

	protoTeam := req.GetTeam()
	if protoTeam.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "team id is required")
	}

	id, err := uuid.Parse(protoTeam.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid team id: %v", err)
	}

	// Check if team exists
	existing, err := s.store.GetTeam(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get team: %v", err)
	}
	if existing == nil {
		return nil, status.Error(codes.NotFound, "team not found")
	}

	internalTeam := protoTeamToInternal(protoTeam)
	internalTeam.ID = id

	updated, err := s.store.UpdateTeam(ctx, internalTeam)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update team: %v", err)
	}
	if updated == nil {
		return nil, status.Error(codes.NotFound, "team not found")
	}

	return internalTeamToProto(updated), nil
}

// DeleteTeam deletes a team.
func (s *TeamServiceServer) DeleteTeam(ctx context.Context, req *DeleteTeamRequest) (*routingv1.DeleteTeamResponse, error) {
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "team id is required")
	}

	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid team id: %v", err)
	}

	err = s.store.DeleteTeam(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete team: %v", err)
	}

	return &routingv1.DeleteTeamResponse{Success: true}, nil
}

// AddTeamMember adds a member to a team.
func (s *TeamServiceServer) AddTeamMember(ctx context.Context, req *AddTeamMemberRequest) (*routingv1.Team, error) {
	if req.GetTeamId() == "" {
		return nil, status.Error(codes.InvalidArgument, "team_id is required")
	}
	if req.GetMember() == nil {
		return nil, status.Error(codes.InvalidArgument, "member is required")
	}
	if req.GetMember().GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "member user_id is required")
	}

	teamID, err := uuid.Parse(req.GetTeamId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid team_id: %v", err)
	}

	userID, err := uuid.Parse(req.GetMember().GetUserId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}

	// Check if team exists
	t, err := s.store.GetTeam(ctx, teamID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get team: %v", err)
	}
	if t == nil {
		return nil, status.Error(codes.NotFound, "team not found")
	}

	role := protoRoleToInternal(req.GetMember().GetRole())
	err = s.store.AddMember(ctx, teamID, userID, role)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to add team member: %v", err)
	}

	// Fetch updated team with members
	members, err := s.store.GetTeamMembers(ctx, teamID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get team members: %v", err)
	}

	protoTeam := internalTeamToProto(t)
	protoTeam.Members = internalTeamMembersToProto(members)

	return protoTeam, nil
}

// RemoveTeamMember removes a member from a team.
func (s *TeamServiceServer) RemoveTeamMember(ctx context.Context, req *RemoveTeamMemberRequest) (*routingv1.Team, error) {
	if req.GetTeamId() == "" {
		return nil, status.Error(codes.InvalidArgument, "team_id is required")
	}
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	teamID, err := uuid.Parse(req.GetTeamId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid team_id: %v", err)
	}

	userID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}

	// Check if team exists
	t, err := s.store.GetTeam(ctx, teamID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get team: %v", err)
	}
	if t == nil {
		return nil, status.Error(codes.NotFound, "team not found")
	}

	err = s.store.RemoveMember(ctx, teamID, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove team member: %v", err)
	}

	// Fetch updated team with members
	members, err := s.store.GetTeamMembers(ctx, teamID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get team members: %v", err)
	}

	protoTeam := internalTeamToProto(t)
	protoTeam.Members = internalTeamMembersToProto(members)

	return protoTeam, nil
}

// UpdateTeamMember updates a team member's role.
func (s *TeamServiceServer) UpdateTeamMember(ctx context.Context, req *UpdateTeamMemberRequest) (*routingv1.Team, error) {
	if req.GetTeamId() == "" {
		return nil, status.Error(codes.InvalidArgument, "team_id is required")
	}
	if req.GetMember() == nil {
		return nil, status.Error(codes.InvalidArgument, "member is required")
	}
	if req.GetMember().GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "member user_id is required")
	}

	teamID, err := uuid.Parse(req.GetTeamId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid team_id: %v", err)
	}

	userID, err := uuid.Parse(req.GetMember().GetUserId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}

	// Check if team exists
	t, err := s.store.GetTeam(ctx, teamID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get team: %v", err)
	}
	if t == nil {
		return nil, status.Error(codes.NotFound, "team not found")
	}

	role := protoRoleToInternal(req.GetMember().GetRole())
	err = s.store.UpdateMember(ctx, teamID, userID, role)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update team member: %v", err)
	}

	// Fetch updated team with members
	members, err := s.store.GetTeamMembers(ctx, teamID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get team members: %v", err)
	}

	protoTeam := internalTeamToProto(t)
	protoTeam.Members = internalTeamMembersToProto(members)

	return protoTeam, nil
}

// GetUserTeams retrieves all teams a user belongs to.
func (s *TeamServiceServer) GetUserTeams(ctx context.Context, req *GetUserTeamsRequest) (*routingv1.ListTeamsResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	userID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id: %v", err)
	}

	teams, err := s.store.GetUserTeams(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user teams: %v", err)
	}

	protoTeams := make([]*routingv1.Team, 0, len(teams))
	for _, t := range teams {
		protoTeams = append(protoTeams, internalTeamToProto(t))
	}

	return &routingv1.ListTeamsResponse{
		Teams:      protoTeams,
		TotalCount: int32(len(protoTeams)),
	}, nil
}

// Type aliases for request/response types from proto package
type (
	CreateTeamRequest       = routingv1.CreateTeamRequest
	GetTeamRequest          = routingv1.GetTeamRequest
	ListTeamsRequest        = routingv1.ListTeamsRequest
	UpdateTeamRequest       = routingv1.UpdateTeamRequest
	DeleteTeamRequest       = routingv1.DeleteTeamRequest
	AddTeamMemberRequest    = routingv1.AddTeamMemberRequest
	RemoveTeamMemberRequest = routingv1.RemoveTeamMemberRequest
	UpdateTeamMemberRequest = routingv1.UpdateTeamMemberRequest
	GetUserTeamsRequest     = routingv1.GetUserTeamsRequest
)

// Conversion helpers

func protoTeamToInternal(p *routingv1.Team) *team.Team {
	t := &team.Team{
		Name:          p.GetName(),
		Description:   p.GetDescription(),
		AssignedSites: p.GetAssignedSites(),
		AssignedPops:  p.GetAssignedPops(),
	}

	if p.GetId() != "" {
		if id, err := uuid.Parse(p.GetId()); err == nil {
			t.ID = id
		}
	}

	if p.GetDefaultEscalationPolicyId() != "" {
		if id, err := uuid.Parse(p.GetDefaultEscalationPolicyId()); err == nil {
			t.DefaultEscalationPolicyID = &id
		}
	}

	if p.GetDefaultChannel() != nil {
		t.DefaultChannel = &team.NotificationChannel{
			Type: p.GetDefaultChannel().GetChannel().String(),
		}
	}

	if p.GetMetadata() != nil {
		t.Metadata = make(map[string]interface{})
		for k, v := range p.GetMetadata() {
			t.Metadata[k] = v
		}
	}

	return t
}

func internalTeamToProto(t *team.Team) *routingv1.Team {
	p := &routingv1.Team{
		Id:            t.ID.String(),
		Name:          t.Name,
		Description:   t.Description,
		AssignedSites: t.AssignedSites,
		AssignedPops:  t.AssignedPops,
		CreatedAt:     timestamppb.New(t.CreatedAt),
		UpdatedAt:     timestamppb.New(t.UpdatedAt),
	}

	if t.DefaultEscalationPolicyID != nil {
		p.DefaultEscalationPolicyId = t.DefaultEscalationPolicyID.String()
	}

	if t.Metadata != nil {
		p.Metadata = make(map[string]string)
		for k, v := range t.Metadata {
			if s, ok := v.(string); ok {
				p.Metadata[k] = s
			}
		}
	}

	return p
}

// protoTeamMemberToInternal converts a proto TeamMember to internal domain model.
// nolint:unused // Kept for future use when batch member operations are implemented.
func protoTeamMemberToInternal(p *routingv1.TeamMember) *team.TeamMember {
	m := &team.TeamMember{
		Role: protoRoleToInternal(p.GetRole()),
	}

	if p.GetUserId() != "" {
		if id, err := uuid.Parse(p.GetUserId()); err == nil {
			m.UserID = id
		}
	}

	return m
}

func internalTeamMemberToProto(m *team.TeamMember) *routingv1.TeamMember {
	return &routingv1.TeamMember{
		UserId:   m.UserID.String(),
		Role:     internalRoleToProto(m.Role),
		JoinedAt: timestamppb.New(m.JoinedAt),
	}
}

func internalTeamMembersToProto(members []*team.TeamMember) []*routingv1.TeamMember {
	result := make([]*routingv1.TeamMember, 0, len(members))
	for _, m := range members {
		result = append(result, internalTeamMemberToProto(m))
	}
	return result
}

func protoRoleToInternal(r routingv1.TeamRole) team.TeamRole {
	switch r {
	case routingv1.TeamRole_TEAM_ROLE_MEMBER:
		return team.TeamRoleMember
	case routingv1.TeamRole_TEAM_ROLE_LEAD:
		return team.TeamRoleLead
	case routingv1.TeamRole_TEAM_ROLE_MANAGER:
		return team.TeamRoleManager
	default:
		return team.TeamRoleMember
	}
}

func internalRoleToProto(r team.TeamRole) routingv1.TeamRole {
	switch r {
	case team.TeamRoleMember:
		return routingv1.TeamRole_TEAM_ROLE_MEMBER
	case team.TeamRoleLead:
		return routingv1.TeamRole_TEAM_ROLE_LEAD
	case team.TeamRoleManager:
		return routingv1.TeamRole_TEAM_ROLE_MANAGER
	default:
		return routingv1.TeamRole_TEAM_ROLE_UNSPECIFIED
	}
}
