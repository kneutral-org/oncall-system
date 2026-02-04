package team

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InMemoryTeamStore is an in-memory implementation of TeamStore for testing.
type InMemoryTeamStore struct {
	mu      sync.RWMutex
	teams   map[uuid.UUID]*Team
	members map[uuid.UUID][]*TeamMember // teamID -> members
}

// NewInMemoryTeamStore creates a new in-memory team store.
func NewInMemoryTeamStore() *InMemoryTeamStore {
	return &InMemoryTeamStore{
		teams:   make(map[uuid.UUID]*Team),
		members: make(map[uuid.UUID][]*TeamMember),
	}
}

// CreateTeam creates a new team.
func (s *InMemoryTeamStore) CreateTeam(ctx context.Context, team *Team) (*Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	team.ID = uuid.New()
	team.CreatedAt = time.Now()
	team.UpdatedAt = time.Now()

	// Deep copy
	stored := *team
	s.teams[team.ID] = &stored

	return team, nil
}

// GetTeam retrieves a team by ID.
func (s *InMemoryTeamStore) GetTeam(ctx context.Context, id uuid.UUID) (*Team, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	team, ok := s.teams[id]
	if !ok {
		return nil, nil
	}

	result := *team
	return &result, nil
}

// ListTeams retrieves teams based on filter criteria.
func (s *InMemoryTeamStore) ListTeams(ctx context.Context, params ListTeamsParams) ([]*Team, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}

	result := make([]*Team, 0)
	offset := int(params.Offset)
	count := 0

	for _, t := range s.teams {
		// Apply filters
		if params.NameFilter != "" && !strings.Contains(strings.ToLower(t.Name), strings.ToLower(params.NameFilter)) {
			continue
		}

		if len(params.SitesFilter) > 0 {
			found := false
			for _, site := range params.SitesFilter {
				for _, assignedSite := range t.AssignedSites {
					if site == assignedSite {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				continue
			}
		}

		if count < offset {
			count++
			continue
		}

		if int32(len(result)) >= limit {
			break
		}

		team := *t
		result = append(result, &team)
		count++
	}

	return result, nil
}

// UpdateTeam updates an existing team.
func (s *InMemoryTeamStore) UpdateTeam(ctx context.Context, team *Team) (*Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.teams[team.ID]; !ok {
		return nil, nil
	}

	team.UpdatedAt = time.Now()

	stored := *team
	s.teams[team.ID] = &stored

	return team, nil
}

// DeleteTeam deletes a team.
func (s *InMemoryTeamStore) DeleteTeam(ctx context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.teams, id)
	delete(s.members, id)
	return nil
}

// AddMember adds a member to a team.
func (s *InMemoryTeamStore) AddMember(ctx context.Context, teamID, userID uuid.UUID, role TeamRole) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	member := &TeamMember{
		TeamID:   teamID,
		UserID:   userID,
		Role:     role,
		JoinedAt: time.Now(),
	}

	s.members[teamID] = append(s.members[teamID], member)
	return nil
}

// RemoveMember removes a member from a team.
func (s *InMemoryTeamStore) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	members := s.members[teamID]
	for i, m := range members {
		if m.UserID == userID {
			s.members[teamID] = append(members[:i], members[i+1:]...)
			break
		}
	}
	return nil
}

// UpdateMember updates a team member's role.
func (s *InMemoryTeamStore) UpdateMember(ctx context.Context, teamID, userID uuid.UUID, role TeamRole) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, m := range s.members[teamID] {
		if m.UserID == userID {
			m.Role = role
			break
		}
	}
	return nil
}

// GetTeamMembers retrieves all members of a team.
func (s *InMemoryTeamStore) GetTeamMembers(ctx context.Context, teamID uuid.UUID) ([]*TeamMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	members := s.members[teamID]
	result := make([]*TeamMember, 0, len(members))
	for _, m := range members {
		member := *m
		result = append(result, &member)
	}
	return result, nil
}

// GetUserTeams retrieves all teams a user belongs to.
func (s *InMemoryTeamStore) GetUserTeams(ctx context.Context, userID uuid.UUID) ([]*Team, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Team, 0)

	for teamID, members := range s.members {
		for _, m := range members {
			if m.UserID == userID {
				if team, ok := s.teams[teamID]; ok {
					t := *team
					result = append(result, &t)
				}
				break
			}
		}
	}

	return result, nil
}
