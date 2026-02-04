package grpc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/kneutral-org/alerting-system/internal/team"
	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

func TestTeamServiceServer_CreateTeam(t *testing.T) {
	store := team.NewInMemoryTeamStore()
	server := NewTeamServiceServer(store)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		req := &routingv1.CreateTeamRequest{
			Team: &routingv1.Team{
				Name:        "Test Team",
				Description: "Test team description",
			},
		}

		resp, err := server.CreateTeam(ctx, req)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.GetId())
		assert.Equal(t, "Test Team", resp.GetName())
		assert.Equal(t, "Test team description", resp.GetDescription())
	})

	t.Run("error_nil_team", func(t *testing.T) {
		req := &routingv1.CreateTeamRequest{}

		_, err := server.CreateTeam(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("error_empty_name", func(t *testing.T) {
		req := &routingv1.CreateTeamRequest{
			Team: &routingv1.Team{
				Name: "",
			},
		}

		_, err := server.CreateTeam(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestTeamServiceServer_GetTeam(t *testing.T) {
	store := team.NewInMemoryTeamStore()
	server := NewTeamServiceServer(store)
	ctx := context.Background()

	// Create a team first
	createReq := &routingv1.CreateTeamRequest{
		Team: &routingv1.Team{
			Name: "Test Team",
		},
	}
	created, err := server.CreateTeam(ctx, createReq)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		req := &routingv1.GetTeamRequest{
			Id: created.GetId(),
		}

		resp, err := server.GetTeam(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, created.GetId(), resp.GetId())
		assert.Equal(t, "Test Team", resp.GetName())
	})

	t.Run("error_empty_id", func(t *testing.T) {
		req := &routingv1.GetTeamRequest{
			Id: "",
		}

		_, err := server.GetTeam(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("error_invalid_id", func(t *testing.T) {
		req := &routingv1.GetTeamRequest{
			Id: "invalid-uuid",
		}

		_, err := server.GetTeam(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("error_not_found", func(t *testing.T) {
		req := &routingv1.GetTeamRequest{
			Id: uuid.New().String(),
		}

		_, err := server.GetTeam(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
	})
}

func TestTeamServiceServer_ListTeams(t *testing.T) {
	store := team.NewInMemoryTeamStore()
	server := NewTeamServiceServer(store)
	ctx := context.Background()

	// Create some teams
	for i := 0; i < 3; i++ {
		req := &routingv1.CreateTeamRequest{
			Team: &routingv1.Team{
				Name: "Team " + string(rune('A'+i)),
			},
		}
		_, err := server.CreateTeam(ctx, req)
		require.NoError(t, err)
	}

	t.Run("success", func(t *testing.T) {
		req := &routingv1.ListTeamsRequest{
			PageSize: 10,
		}

		resp, err := server.ListTeams(ctx, req)
		require.NoError(t, err)
		assert.Len(t, resp.GetTeams(), 3)
		assert.Equal(t, int32(3), resp.GetTotalCount())
	})

	t.Run("with_page_size", func(t *testing.T) {
		req := &routingv1.ListTeamsRequest{
			PageSize: 2,
		}

		resp, err := server.ListTeams(ctx, req)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(resp.GetTeams()), 2)
	})
}

func TestTeamServiceServer_UpdateTeam(t *testing.T) {
	store := team.NewInMemoryTeamStore()
	server := NewTeamServiceServer(store)
	ctx := context.Background()

	// Create a team first
	createReq := &routingv1.CreateTeamRequest{
		Team: &routingv1.Team{
			Name: "Original Name",
		},
	}
	created, err := server.CreateTeam(ctx, createReq)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		req := &routingv1.UpdateTeamRequest{
			Team: &routingv1.Team{
				Id:          created.GetId(),
				Name:        "Updated Name",
				Description: "Updated description",
			},
		}

		resp, err := server.UpdateTeam(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", resp.GetName())
		assert.Equal(t, "Updated description", resp.GetDescription())
	})

	t.Run("error_nil_team", func(t *testing.T) {
		req := &routingv1.UpdateTeamRequest{}

		_, err := server.UpdateTeam(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("error_not_found", func(t *testing.T) {
		req := &routingv1.UpdateTeamRequest{
			Team: &routingv1.Team{
				Id:   uuid.New().String(),
				Name: "New Name",
			},
		}

		_, err := server.UpdateTeam(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
	})
}

func TestTeamServiceServer_DeleteTeam(t *testing.T) {
	store := team.NewInMemoryTeamStore()
	server := NewTeamServiceServer(store)
	ctx := context.Background()

	// Create a team first
	createReq := &routingv1.CreateTeamRequest{
		Team: &routingv1.Team{
			Name: "To Delete",
		},
	}
	created, err := server.CreateTeam(ctx, createReq)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		req := &routingv1.DeleteTeamRequest{
			Id: created.GetId(),
		}

		resp, err := server.DeleteTeam(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.GetSuccess())

		// Verify team is deleted
		getReq := &routingv1.GetTeamRequest{Id: created.GetId()}
		_, err = server.GetTeam(ctx, getReq)
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("error_empty_id", func(t *testing.T) {
		req := &routingv1.DeleteTeamRequest{
			Id: "",
		}

		_, err := server.DeleteTeam(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestTeamServiceServer_AddTeamMember(t *testing.T) {
	store := team.NewInMemoryTeamStore()
	server := NewTeamServiceServer(store)
	ctx := context.Background()

	// Create a team first
	createReq := &routingv1.CreateTeamRequest{
		Team: &routingv1.Team{
			Name: "Test Team",
		},
	}
	created, err := server.CreateTeam(ctx, createReq)
	require.NoError(t, err)

	userID := uuid.New().String()

	t.Run("success", func(t *testing.T) {
		req := &routingv1.AddTeamMemberRequest{
			TeamId: created.GetId(),
			Member: &routingv1.TeamMember{
				UserId: userID,
				Role:   routingv1.TeamRole_TEAM_ROLE_MEMBER,
			},
		}

		resp, err := server.AddTeamMember(ctx, req)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.GetMembers())
	})

	t.Run("error_missing_team_id", func(t *testing.T) {
		req := &routingv1.AddTeamMemberRequest{
			Member: &routingv1.TeamMember{
				UserId: userID,
			},
		}

		_, err := server.AddTeamMember(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("error_missing_member", func(t *testing.T) {
		req := &routingv1.AddTeamMemberRequest{
			TeamId: created.GetId(),
		}

		_, err := server.AddTeamMember(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("error_team_not_found", func(t *testing.T) {
		req := &routingv1.AddTeamMemberRequest{
			TeamId: uuid.New().String(),
			Member: &routingv1.TeamMember{
				UserId: userID,
			},
		}

		_, err := server.AddTeamMember(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
	})
}

func TestTeamServiceServer_RemoveTeamMember(t *testing.T) {
	store := team.NewInMemoryTeamStore()
	server := NewTeamServiceServer(store)
	ctx := context.Background()

	// Create a team and add a member
	createReq := &routingv1.CreateTeamRequest{
		Team: &routingv1.Team{
			Name: "Test Team",
		},
	}
	created, err := server.CreateTeam(ctx, createReq)
	require.NoError(t, err)

	userID := uuid.New().String()
	addReq := &routingv1.AddTeamMemberRequest{
		TeamId: created.GetId(),
		Member: &routingv1.TeamMember{
			UserId: userID,
			Role:   routingv1.TeamRole_TEAM_ROLE_MEMBER,
		},
	}
	_, err = server.AddTeamMember(ctx, addReq)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		req := &routingv1.RemoveTeamMemberRequest{
			TeamId: created.GetId(),
			UserId: userID,
		}

		resp, err := server.RemoveTeamMember(ctx, req)
		require.NoError(t, err)
		assert.Empty(t, resp.GetMembers())
	})

	t.Run("error_missing_team_id", func(t *testing.T) {
		req := &routingv1.RemoveTeamMemberRequest{
			UserId: userID,
		}

		_, err := server.RemoveTeamMember(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestTeamServiceServer_UpdateTeamMember(t *testing.T) {
	store := team.NewInMemoryTeamStore()
	server := NewTeamServiceServer(store)
	ctx := context.Background()

	// Create a team and add a member
	createReq := &routingv1.CreateTeamRequest{
		Team: &routingv1.Team{
			Name: "Test Team",
		},
	}
	created, err := server.CreateTeam(ctx, createReq)
	require.NoError(t, err)

	userID := uuid.New().String()
	addReq := &routingv1.AddTeamMemberRequest{
		TeamId: created.GetId(),
		Member: &routingv1.TeamMember{
			UserId: userID,
			Role:   routingv1.TeamRole_TEAM_ROLE_MEMBER,
		},
	}
	_, err = server.AddTeamMember(ctx, addReq)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		req := &routingv1.UpdateTeamMemberRequest{
			TeamId: created.GetId(),
			Member: &routingv1.TeamMember{
				UserId: userID,
				Role:   routingv1.TeamRole_TEAM_ROLE_LEAD,
			},
		}

		resp, err := server.UpdateTeamMember(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})
}

func TestTeamServiceServer_GetUserTeams(t *testing.T) {
	store := team.NewInMemoryTeamStore()
	server := NewTeamServiceServer(store)
	ctx := context.Background()

	// Create teams and add a user to them
	userID := uuid.New().String()

	for i := 0; i < 2; i++ {
		createReq := &routingv1.CreateTeamRequest{
			Team: &routingv1.Team{
				Name: "Team " + string(rune('A'+i)),
			},
		}
		created, err := server.CreateTeam(ctx, createReq)
		require.NoError(t, err)

		addReq := &routingv1.AddTeamMemberRequest{
			TeamId: created.GetId(),
			Member: &routingv1.TeamMember{
				UserId: userID,
				Role:   routingv1.TeamRole_TEAM_ROLE_MEMBER,
			},
		}
		_, err = server.AddTeamMember(ctx, addReq)
		require.NoError(t, err)
	}

	t.Run("success", func(t *testing.T) {
		req := &routingv1.GetUserTeamsRequest{
			UserId: userID,
		}

		resp, err := server.GetUserTeams(ctx, req)
		require.NoError(t, err)
		assert.Len(t, resp.GetTeams(), 2)
	})

	t.Run("error_missing_user_id", func(t *testing.T) {
		req := &routingv1.GetUserTeamsRequest{}

		_, err := server.GetUserTeams(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}
