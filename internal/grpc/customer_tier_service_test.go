package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/kneutral-org/alerting-system/internal/customer"
	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

func setupTestCustomerTierService(t *testing.T) (*CustomerTierService, *customer.InMemoryTierStore, *customer.InMemoryStore) {
	tierStore := customer.NewInMemoryTierStore()
	customerStore := customer.NewInMemoryStore()

	resolver := customer.NewResolver(customerStore, tierStore, customer.ResolverConfig{
		CacheTTL: time.Minute,
		Logger:   zerolog.Nop(),
	})
	t.Cleanup(func() {
		resolver.Stop()
	})

	service := NewCustomerTierService(tierStore, customerStore, resolver, zerolog.Nop())

	return service, tierStore, customerStore
}

func TestCustomerTierService_CreateCustomerTier(t *testing.T) {
	service, _, _ := setupTestCustomerTierService(t)
	ctx := context.Background()

	req := &routingv1.CreateCustomerTierRequest{
		Tier: &routingv1.CustomerTier{
			Name:             "Enterprise",
			Level:            1,
			CriticalResponse: durationpb.New(5 * time.Minute),
			HighResponse:     durationpb.New(30 * time.Minute),
			MediumResponse:   durationpb.New(2 * time.Hour),
			EscalationMultiplier: 0.5,
			Metadata: map[string]string{
				"sla": "24x7",
			},
		},
	}

	tier, err := service.CreateCustomerTier(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, tier.Id)
	assert.Equal(t, "Enterprise", tier.Name)
	assert.Equal(t, int32(1), tier.Level)
	assert.Equal(t, float32(0.5), tier.EscalationMultiplier)
}

func TestCustomerTierService_CreateCustomerTier_NilTier(t *testing.T) {
	service, _, _ := setupTestCustomerTierService(t)
	ctx := context.Background()

	_, err := service.CreateCustomerTier(ctx, &routingv1.CreateCustomerTierRequest{})
	assert.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestCustomerTierService_CreateCustomerTier_NoName(t *testing.T) {
	service, _, _ := setupTestCustomerTierService(t)
	ctx := context.Background()

	_, err := service.CreateCustomerTier(ctx, &routingv1.CreateCustomerTierRequest{
		Tier: &routingv1.CustomerTier{Level: 1},
	})
	assert.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestCustomerTierService_CreateCustomerTier_DuplicateName(t *testing.T) {
	service, _, _ := setupTestCustomerTierService(t)
	ctx := context.Background()

	// Create first tier
	_, err := service.CreateCustomerTier(ctx, &routingv1.CreateCustomerTierRequest{
		Tier: &routingv1.CustomerTier{Name: "Enterprise", Level: 1},
	})
	require.NoError(t, err)

	// Try to create duplicate
	_, err = service.CreateCustomerTier(ctx, &routingv1.CreateCustomerTierRequest{
		Tier: &routingv1.CustomerTier{Name: "Enterprise", Level: 2},
	})
	assert.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.AlreadyExists, st.Code())
}

func TestCustomerTierService_GetCustomerTier(t *testing.T) {
	service, _, _ := setupTestCustomerTierService(t)
	ctx := context.Background()

	// Create a tier
	created, err := service.CreateCustomerTier(ctx, &routingv1.CreateCustomerTierRequest{
		Tier: &routingv1.CustomerTier{Name: "Enterprise", Level: 1},
	})
	require.NoError(t, err)

	// Get the tier
	tier, err := service.GetCustomerTier(ctx, &routingv1.GetCustomerTierRequest{Id: created.Id})
	require.NoError(t, err)
	assert.Equal(t, created.Id, tier.Id)
	assert.Equal(t, "Enterprise", tier.Name)
}

func TestCustomerTierService_GetCustomerTier_NotFound(t *testing.T) {
	service, _, _ := setupTestCustomerTierService(t)
	ctx := context.Background()

	_, err := service.GetCustomerTier(ctx, &routingv1.GetCustomerTierRequest{Id: "nonexistent"})
	assert.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestCustomerTierService_GetCustomerTier_NoID(t *testing.T) {
	service, _, _ := setupTestCustomerTierService(t)
	ctx := context.Background()

	_, err := service.GetCustomerTier(ctx, &routingv1.GetCustomerTierRequest{})
	assert.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestCustomerTierService_ListCustomerTiers(t *testing.T) {
	service, _, _ := setupTestCustomerTierService(t)
	ctx := context.Background()

	// Create multiple tiers
	tiers := []struct {
		name  string
		level int32
	}{
		{"Standard", 3},
		{"Enterprise", 1},
		{"Premium", 2},
	}

	for _, tier := range tiers {
		_, err := service.CreateCustomerTier(ctx, &routingv1.CreateCustomerTierRequest{
			Tier: &routingv1.CustomerTier{Name: tier.name, Level: tier.level},
		})
		require.NoError(t, err)
	}

	// List all tiers
	resp, err := service.ListCustomerTiers(ctx, &routingv1.ListCustomerTiersRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Tiers, 3)

	// Should be sorted by level
	assert.Equal(t, int32(1), resp.Tiers[0].Level)
	assert.Equal(t, int32(2), resp.Tiers[1].Level)
	assert.Equal(t, int32(3), resp.Tiers[2].Level)
}

func TestCustomerTierService_UpdateCustomerTier(t *testing.T) {
	service, _, _ := setupTestCustomerTierService(t)
	ctx := context.Background()

	// Create a tier
	created, err := service.CreateCustomerTier(ctx, &routingv1.CreateCustomerTierRequest{
		Tier: &routingv1.CustomerTier{Name: "Enterprise", Level: 1},
	})
	require.NoError(t, err)

	// Update the tier
	created.EscalationMultiplier = 0.25
	updated, err := service.UpdateCustomerTier(ctx, &routingv1.UpdateCustomerTierRequest{
		Tier: created,
	})
	require.NoError(t, err)
	assert.Equal(t, float32(0.25), updated.EscalationMultiplier)

	// Verify persisted
	tier, err := service.GetCustomerTier(ctx, &routingv1.GetCustomerTierRequest{Id: created.Id})
	require.NoError(t, err)
	assert.Equal(t, float32(0.25), tier.EscalationMultiplier)
}

func TestCustomerTierService_UpdateCustomerTier_NotFound(t *testing.T) {
	service, _, _ := setupTestCustomerTierService(t)
	ctx := context.Background()

	_, err := service.UpdateCustomerTier(ctx, &routingv1.UpdateCustomerTierRequest{
		Tier: &routingv1.CustomerTier{Id: "nonexistent", Name: "Test", Level: 1},
	})
	assert.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestCustomerTierService_DeleteCustomerTier(t *testing.T) {
	service, _, _ := setupTestCustomerTierService(t)
	ctx := context.Background()

	// Create a tier
	created, err := service.CreateCustomerTier(ctx, &routingv1.CreateCustomerTierRequest{
		Tier: &routingv1.CustomerTier{Name: "Enterprise", Level: 1},
	})
	require.NoError(t, err)

	// Delete the tier
	resp, err := service.DeleteCustomerTier(ctx, &routingv1.DeleteCustomerTierRequest{Id: created.Id})
	require.NoError(t, err)
	assert.True(t, resp.Success)

	// Verify deleted
	_, err = service.GetCustomerTier(ctx, &routingv1.GetCustomerTierRequest{Id: created.Id})
	assert.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestCustomerTierService_DeleteCustomerTier_NotFound(t *testing.T) {
	service, _, _ := setupTestCustomerTierService(t)
	ctx := context.Background()

	_, err := service.DeleteCustomerTier(ctx, &routingv1.DeleteCustomerTierRequest{Id: "nonexistent"})
	assert.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestCustomerTierService_ResolveCustomerTier_ByCustomerID(t *testing.T) {
	service, tierStore, customerStore := setupTestCustomerTierService(t)
	ctx := context.Background()

	// Create a tier
	tier, err := tierStore.Create(ctx, &customer.CustomerTier{
		Name:                 "Enterprise",
		Level:                1,
		EscalationMultiplier: 0.5,
	})
	require.NoError(t, err)

	// Create a customer
	cust, err := customerStore.Create(ctx, &customer.Customer{
		Name:      "Acme Corp",
		AccountID: "acme-001",
		TierID:    tier.ID,
	})
	require.NoError(t, err)

	// Resolve by customer ID
	resp, err := service.ResolveCustomerTier(ctx, &routingv1.ResolveCustomerTierRequest{
		CustomerId: cust.ID,
	})
	require.NoError(t, err)
	assert.True(t, resp.Found)
	assert.Equal(t, tier.ID, resp.Tier.Id)
}

func TestCustomerTierService_ResolveCustomerTier_ByLabels(t *testing.T) {
	service, tierStore, customerStore := setupTestCustomerTierService(t)
	ctx := context.Background()

	// Create a tier
	tier, err := tierStore.Create(ctx, &customer.CustomerTier{
		Name:                 "Enterprise",
		Level:                1,
		EscalationMultiplier: 0.5,
	})
	require.NoError(t, err)

	// Create a customer
	_, err = customerStore.Create(ctx, &customer.Customer{
		Name:      "Acme Corp",
		AccountID: "acme-001",
		TierID:    tier.ID,
		Domains:   []string{"acme.com"},
	})
	require.NoError(t, err)

	// Resolve by labels
	resp, err := service.ResolveCustomerTier(ctx, &routingv1.ResolveCustomerTierRequest{
		Labels: map[string]string{
			"domain": "acme.com",
		},
	})
	require.NoError(t, err)
	assert.True(t, resp.Found)
	assert.Equal(t, tier.ID, resp.Tier.Id)
}

func TestCustomerTierService_ResolveCustomerTier_NotFound(t *testing.T) {
	service, _, _ := setupTestCustomerTierService(t)
	ctx := context.Background()

	resp, err := service.ResolveCustomerTier(ctx, &routingv1.ResolveCustomerTierRequest{
		Labels: map[string]string{
			"customer": "nonexistent",
		},
	})
	require.NoError(t, err)
	assert.False(t, resp.Found)
	assert.Nil(t, resp.Tier)
}

func TestProtoToTier(t *testing.T) {
	proto := &routingv1.CustomerTier{
		Id:                   "tier-1",
		Name:                 "Enterprise",
		Level:                1,
		CriticalResponse:     durationpb.New(5 * time.Minute),
		HighResponse:         durationpb.New(30 * time.Minute),
		MediumResponse:       durationpb.New(2 * time.Hour),
		EscalationMultiplier: 0.5,
		DedicatedTeamId:      "team-1",
		Metadata: map[string]string{
			"sla": "24x7",
		},
	}

	tier := protoToTier(proto)
	assert.Equal(t, "tier-1", tier.ID)
	assert.Equal(t, "Enterprise", tier.Name)
	assert.Equal(t, 1, tier.Level)
	assert.Equal(t, 5*time.Minute, tier.CriticalResponseTime)
	assert.Equal(t, 30*time.Minute, tier.HighResponseTime)
	assert.Equal(t, 2*time.Hour, tier.MediumResponseTime)
	assert.Equal(t, 0.5, tier.EscalationMultiplier)
	assert.NotNil(t, tier.DedicatedTeamID)
	assert.Equal(t, "team-1", *tier.DedicatedTeamID)
}

func TestTierToProto(t *testing.T) {
	teamID := "team-1"
	tier := &customer.CustomerTier{
		ID:                   "tier-1",
		Name:                 "Enterprise",
		Level:                1,
		CriticalResponseTime: 5 * time.Minute,
		HighResponseTime:     30 * time.Minute,
		MediumResponseTime:   2 * time.Hour,
		EscalationMultiplier: 0.5,
		DedicatedTeamID:      &teamID,
		Metadata: map[string]string{
			"sla": "24x7",
		},
	}

	proto := tierToProto(tier)
	assert.Equal(t, "tier-1", proto.Id)
	assert.Equal(t, "Enterprise", proto.Name)
	assert.Equal(t, int32(1), proto.Level)
	assert.Equal(t, 5*time.Minute, proto.CriticalResponse.AsDuration())
	assert.Equal(t, 30*time.Minute, proto.HighResponse.AsDuration())
	assert.Equal(t, 2*time.Hour, proto.MediumResponse.AsDuration())
	assert.Equal(t, float32(0.5), proto.EscalationMultiplier)
	assert.Equal(t, "team-1", proto.DedicatedTeamId)
}
