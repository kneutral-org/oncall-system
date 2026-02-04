package grpc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/kneutral-org/alerting-system/internal/escalation"
	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

func TestEscalationServiceServer_CreateEscalationPolicy(t *testing.T) {
	policyStore := escalation.NewInMemoryPolicyStore()
	activeStore := escalation.NewInMemoryActiveEscalationStore()
	server := NewEscalationServiceServer(policyStore, activeStore)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		req := &routingv1.CreateEscalationPolicyRequest{
			Policy: &routingv1.EscalationPolicy{
				Name:        "Test Policy",
				Description: "Test policy description",
				RepeatCount: 3,
				Steps: []*routingv1.EscalationStep{
					{
						StepNumber: 1,
						Delay:      durationpb.New(60),
						Targets: []*routingv1.EscalationTarget{
							{
								Type:   routingv1.EscalationTargetType_ESCALATION_TARGET_TYPE_USER,
								UserId: uuid.New().String(),
							},
						},
					},
				},
			},
		}

		resp, err := server.CreateEscalationPolicy(ctx, req)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.GetId())
		assert.Equal(t, "Test Policy", resp.GetName())
		assert.Equal(t, "Test policy description", resp.GetDescription())
		assert.Equal(t, int32(3), resp.GetRepeatCount())
	})

	t.Run("error_nil_policy", func(t *testing.T) {
		req := &routingv1.CreateEscalationPolicyRequest{}

		_, err := server.CreateEscalationPolicy(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("error_empty_name", func(t *testing.T) {
		req := &routingv1.CreateEscalationPolicyRequest{
			Policy: &routingv1.EscalationPolicy{
				Name: "",
			},
		}

		_, err := server.CreateEscalationPolicy(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestEscalationServiceServer_GetEscalationPolicy(t *testing.T) {
	policyStore := escalation.NewInMemoryPolicyStore()
	activeStore := escalation.NewInMemoryActiveEscalationStore()
	server := NewEscalationServiceServer(policyStore, activeStore)
	ctx := context.Background()

	// Create a policy first
	createReq := &routingv1.CreateEscalationPolicyRequest{
		Policy: &routingv1.EscalationPolicy{
			Name: "Test Policy",
		},
	}
	created, err := server.CreateEscalationPolicy(ctx, createReq)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		req := &routingv1.GetEscalationPolicyRequest{
			Id: created.GetId(),
		}

		resp, err := server.GetEscalationPolicy(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, created.GetId(), resp.GetId())
		assert.Equal(t, "Test Policy", resp.GetName())
	})

	t.Run("error_empty_id", func(t *testing.T) {
		req := &routingv1.GetEscalationPolicyRequest{
			Id: "",
		}

		_, err := server.GetEscalationPolicy(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("error_invalid_id", func(t *testing.T) {
		req := &routingv1.GetEscalationPolicyRequest{
			Id: "invalid-uuid",
		}

		_, err := server.GetEscalationPolicy(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("error_not_found", func(t *testing.T) {
		req := &routingv1.GetEscalationPolicyRequest{
			Id: uuid.New().String(),
		}

		_, err := server.GetEscalationPolicy(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
	})
}

func TestEscalationServiceServer_ListEscalationPolicies(t *testing.T) {
	policyStore := escalation.NewInMemoryPolicyStore()
	activeStore := escalation.NewInMemoryActiveEscalationStore()
	server := NewEscalationServiceServer(policyStore, activeStore)
	ctx := context.Background()

	// Create some policies
	for i := 0; i < 3; i++ {
		req := &routingv1.CreateEscalationPolicyRequest{
			Policy: &routingv1.EscalationPolicy{
				Name: "Policy " + string(rune('A'+i)),
			},
		}
		_, err := server.CreateEscalationPolicy(ctx, req)
		require.NoError(t, err)
	}

	t.Run("success", func(t *testing.T) {
		req := &routingv1.ListEscalationPoliciesRequest{
			PageSize: 10,
		}

		resp, err := server.ListEscalationPolicies(ctx, req)
		require.NoError(t, err)
		assert.Len(t, resp.GetPolicies(), 3)
		assert.Equal(t, int32(3), resp.GetTotalCount())
	})

	t.Run("with_page_size", func(t *testing.T) {
		req := &routingv1.ListEscalationPoliciesRequest{
			PageSize: 2,
		}

		resp, err := server.ListEscalationPolicies(ctx, req)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(resp.GetPolicies()), 2)
	})
}

func TestEscalationServiceServer_UpdateEscalationPolicy(t *testing.T) {
	policyStore := escalation.NewInMemoryPolicyStore()
	activeStore := escalation.NewInMemoryActiveEscalationStore()
	server := NewEscalationServiceServer(policyStore, activeStore)
	ctx := context.Background()

	// Create a policy first
	createReq := &routingv1.CreateEscalationPolicyRequest{
		Policy: &routingv1.EscalationPolicy{
			Name: "Original Name",
		},
	}
	created, err := server.CreateEscalationPolicy(ctx, createReq)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		req := &routingv1.UpdateEscalationPolicyRequest{
			Policy: &routingv1.EscalationPolicy{
				Id:          created.GetId(),
				Name:        "Updated Name",
				Description: "Updated description",
				RepeatCount: 5,
			},
		}

		resp, err := server.UpdateEscalationPolicy(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", resp.GetName())
		assert.Equal(t, "Updated description", resp.GetDescription())
		assert.Equal(t, int32(5), resp.GetRepeatCount())
	})

	t.Run("error_nil_policy", func(t *testing.T) {
		req := &routingv1.UpdateEscalationPolicyRequest{}

		_, err := server.UpdateEscalationPolicy(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("error_not_found", func(t *testing.T) {
		req := &routingv1.UpdateEscalationPolicyRequest{
			Policy: &routingv1.EscalationPolicy{
				Id:   uuid.New().String(),
				Name: "New Name",
			},
		}

		_, err := server.UpdateEscalationPolicy(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
	})
}

func TestEscalationServiceServer_DeleteEscalationPolicy(t *testing.T) {
	policyStore := escalation.NewInMemoryPolicyStore()
	activeStore := escalation.NewInMemoryActiveEscalationStore()
	server := NewEscalationServiceServer(policyStore, activeStore)
	ctx := context.Background()

	// Create a policy first
	createReq := &routingv1.CreateEscalationPolicyRequest{
		Policy: &routingv1.EscalationPolicy{
			Name: "To Delete",
		},
	}
	created, err := server.CreateEscalationPolicy(ctx, createReq)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		req := &routingv1.DeleteEscalationPolicyRequest{
			Id: created.GetId(),
		}

		resp, err := server.DeleteEscalationPolicy(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.GetSuccess())

		// Verify policy is deleted
		getReq := &routingv1.GetEscalationPolicyRequest{Id: created.GetId()}
		_, err = server.GetEscalationPolicy(ctx, getReq)
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("error_empty_id", func(t *testing.T) {
		req := &routingv1.DeleteEscalationPolicyRequest{
			Id: "",
		}

		_, err := server.DeleteEscalationPolicy(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestEscalationServiceServer_RuntimeOperations_Unimplemented(t *testing.T) {
	policyStore := escalation.NewInMemoryPolicyStore()
	activeStore := escalation.NewInMemoryActiveEscalationStore()
	server := NewEscalationServiceServer(policyStore, activeStore)
	ctx := context.Background()

	t.Run("start_escalation_unimplemented", func(t *testing.T) {
		req := &routingv1.StartEscalationRequest{
			PolicyId: uuid.New().String(),
			AlertId:  uuid.New().String(),
		}

		_, err := server.StartEscalation(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.Unimplemented, status.Code(err))
	})

	t.Run("get_escalation_status_unimplemented", func(t *testing.T) {
		req := &routingv1.GetEscalationStatusRequest{
			EscalationId: uuid.New().String(),
		}

		_, err := server.GetEscalationStatus(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.Unimplemented, status.Code(err))
	})

	t.Run("stop_escalation_unimplemented", func(t *testing.T) {
		req := &routingv1.StopEscalationRequest{
			EscalationId: uuid.New().String(),
		}

		_, err := server.StopEscalation(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.Unimplemented, status.Code(err))
	})
}
