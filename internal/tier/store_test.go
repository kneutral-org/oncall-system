// Package tier provides the customer tier store implementation.
package tier

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Verify InMemoryCustomerTierStore implements CustomerTierStore interface
var _ CustomerTierStore = (*InMemoryCustomerTierStore)(nil)

func TestInMemoryCustomerTierStore_DefaultTiers(t *testing.T) {
	store := NewInMemoryCustomerTierStore()
	ctx := context.Background()

	// List all tiers - should have default tiers
	tiers, err := store.ListTiers(ctx)
	require.NoError(t, err)
	assert.Len(t, tiers, 4)

	// Verify tiers are sorted by level
	for i := 0; i < len(tiers)-1; i++ {
		assert.Less(t, tiers[i].Level, tiers[i+1].Level)
	}

	// Verify default tier names
	names := make([]string, len(tiers))
	for i, tier := range tiers {
		names[i] = tier.Name
	}
	assert.Equal(t, []string{"Platinum", "Gold", "Silver", "Bronze"}, names)
}

func TestInMemoryCustomerTierStore_CreateTier(t *testing.T) {
	store := NewInMemoryCustomerTierStore()
	ctx := context.Background()

	teamID := uuid.New()
	tier := &CustomerTier{
		Name:                    "Enterprise",
		Level:                   0, // Highest priority
		CriticalResponseMinutes: 1,
		HighResponseMinutes:     5,
		MediumResponseMinutes:   15,
		EscalationMultiplier:    0.25,
		DedicatedTeamID:         &teamID,
		Metadata:                map[string]interface{}{"sla": "99.99%"},
	}

	created, err := store.CreateTier(ctx, tier)
	require.NoError(t, err)
	require.NotNil(t, created)

	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, "Enterprise", created.Name)
	assert.Equal(t, 0, created.Level)
	assert.Equal(t, 1, created.CriticalResponseMinutes)
	assert.Equal(t, 5, created.HighResponseMinutes)
	assert.Equal(t, 15, created.MediumResponseMinutes)
	assert.Equal(t, 0.25, created.EscalationMultiplier)
	assert.Equal(t, teamID, *created.DedicatedTeamID)
	assert.Equal(t, "99.99%", created.Metadata["sla"])
	assert.False(t, created.CreatedAt.IsZero())
}

func TestInMemoryCustomerTierStore_CreateTierDefaults(t *testing.T) {
	store := NewInMemoryCustomerTierStore()
	ctx := context.Background()

	tier := &CustomerTier{
		Name:  "Minimal Tier",
		Level: 99,
	}

	created, err := store.CreateTier(ctx, tier)
	require.NoError(t, err)
	require.NotNil(t, created)

	assert.Equal(t, 15, created.CriticalResponseMinutes)
	assert.Equal(t, 30, created.HighResponseMinutes)
	assert.Equal(t, 60, created.MediumResponseMinutes)
	assert.Equal(t, 1.0, created.EscalationMultiplier)
	assert.NotNil(t, created.Metadata)
}

func TestInMemoryCustomerTierStore_GetTier(t *testing.T) {
	store := NewInMemoryCustomerTierStore()
	ctx := context.Background()

	tier := &CustomerTier{
		Name:  "Test Tier",
		Level: 10,
	}

	created, err := store.CreateTier(ctx, tier)
	require.NoError(t, err)

	// Get existing tier
	fetched, err := store.GetTier(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, "Test Tier", fetched.Name)

	// Get non-existing tier
	fetched, err = store.GetTier(ctx, uuid.New())
	require.NoError(t, err)
	assert.Nil(t, fetched)
}

func TestInMemoryCustomerTierStore_GetTierByLevel(t *testing.T) {
	store := NewInMemoryCustomerTierStore()
	ctx := context.Background()

	// Get existing default tier by level
	fetched, err := store.GetTierByLevel(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, "Platinum", fetched.Name)

	fetched, err = store.GetTierByLevel(ctx, 3)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, "Silver", fetched.Name)

	// Get non-existing tier level
	fetched, err = store.GetTierByLevel(ctx, 999)
	require.NoError(t, err)
	assert.Nil(t, fetched)
}

func TestInMemoryCustomerTierStore_ListTiers(t *testing.T) {
	store := NewInMemoryCustomerTierStore()
	ctx := context.Background()

	// Add custom tiers
	_, err := store.CreateTier(ctx, &CustomerTier{Name: "Enterprise", Level: 0})
	require.NoError(t, err)

	_, err = store.CreateTier(ctx, &CustomerTier{Name: "Free", Level: 5})
	require.NoError(t, err)

	// List all tiers
	tiers, err := store.ListTiers(ctx)
	require.NoError(t, err)
	assert.Len(t, tiers, 6) // 4 default + 2 custom

	// Verify sorted by level
	for i := 0; i < len(tiers)-1; i++ {
		assert.Less(t, tiers[i].Level, tiers[i+1].Level)
	}
}

func TestInMemoryCustomerTierStore_UpdateTier(t *testing.T) {
	store := NewInMemoryCustomerTierStore()
	ctx := context.Background()

	tier := &CustomerTier{
		Name:                    "Test Tier",
		Level:                   10,
		CriticalResponseMinutes: 30,
	}

	created, err := store.CreateTier(ctx, tier)
	require.NoError(t, err)

	// Update the tier
	created.Name = "Updated Tier"
	created.Level = 11
	created.CriticalResponseMinutes = 15

	updated, err := store.UpdateTier(ctx, created)
	require.NoError(t, err)
	require.NotNil(t, updated)

	assert.Equal(t, "Updated Tier", updated.Name)
	assert.Equal(t, 11, updated.Level)
	assert.Equal(t, 15, updated.CriticalResponseMinutes)

	// Verify level index was updated
	fetched, err := store.GetTierByLevel(ctx, 11)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, updated.ID, fetched.ID)

	// Old level should not work
	fetched, err = store.GetTierByLevel(ctx, 10)
	require.NoError(t, err)
	assert.Nil(t, fetched)

	// Update non-existing tier
	nonExisting := &CustomerTier{ID: uuid.New(), Name: "Non-existing", Level: 999}
	result, err := store.UpdateTier(ctx, nonExisting)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestInMemoryCustomerTierStore_DeleteTier(t *testing.T) {
	store := NewInMemoryCustomerTierStore()
	ctx := context.Background()

	tier := &CustomerTier{
		Name:  "Test Tier",
		Level: 10,
	}

	created, err := store.CreateTier(ctx, tier)
	require.NoError(t, err)

	// Delete the tier
	err = store.DeleteTier(ctx, created.ID)
	require.NoError(t, err)

	// Verify tier is deleted
	fetched, err := store.GetTier(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, fetched)

	// Verify level index is also deleted
	fetched, err = store.GetTierByLevel(ctx, 10)
	require.NoError(t, err)
	assert.Nil(t, fetched)
}

func TestInMemoryCustomerTierStore_ResolveTier_ByCustomerID(t *testing.T) {
	store := NewInMemoryCustomerTierStore()
	ctx := context.Background()

	// Set up customer tier mapping
	store.SetCustomerTierMapping("customer-123", 1) // Platinum
	store.SetCustomerTierMapping("customer-456", 3) // Silver

	// Resolve by customer ID
	tier, err := store.ResolveTier(ctx, "customer-123", nil)
	require.NoError(t, err)
	require.NotNil(t, tier)
	assert.Equal(t, "Platinum", tier.Name)

	tier, err = store.ResolveTier(ctx, "customer-456", nil)
	require.NoError(t, err)
	require.NotNil(t, tier)
	assert.Equal(t, "Silver", tier.Name)
}

func TestInMemoryCustomerTierStore_ResolveTier_ByTierLabel(t *testing.T) {
	store := NewInMemoryCustomerTierStore()
	ctx := context.Background()

	// Resolve by tier name label
	tier, err := store.ResolveTier(ctx, "", map[string]string{"tier": "Gold"})
	require.NoError(t, err)
	require.NotNil(t, tier)
	assert.Equal(t, "Gold", tier.Name)
}

func TestInMemoryCustomerTierStore_ResolveTier_ByCustomerTierLabel(t *testing.T) {
	store := NewInMemoryCustomerTierStore()
	ctx := context.Background()

	// Resolve by customer_tier level label
	tier, err := store.ResolveTier(ctx, "", map[string]string{"customer_tier": "2"})
	require.NoError(t, err)
	require.NotNil(t, tier)
	assert.Equal(t, "Gold", tier.Name)
	assert.Equal(t, 2, tier.Level)
}

func TestInMemoryCustomerTierStore_ResolveTier_DefaultToSilver(t *testing.T) {
	store := NewInMemoryCustomerTierStore()
	ctx := context.Background()

	// Resolve with no match should default to Silver (level 3)
	tier, err := store.ResolveTier(ctx, "unknown-customer", map[string]string{"env": "production"})
	require.NoError(t, err)
	require.NotNil(t, tier)
	assert.Equal(t, "Silver", tier.Name)
	assert.Equal(t, 3, tier.Level)
}

func TestInMemoryCustomerTierStore_ResolveTier_FallbackToLowest(t *testing.T) {
	store := NewInMemoryCustomerTierStore()
	ctx := context.Background()

	// Delete all tiers except one
	tiers, _ := store.ListTiers(ctx)
	for _, t := range tiers {
		if t.Level != 1 { // Keep only Platinum
			_ = store.DeleteTier(ctx, t.ID)
		}
	}

	// Resolve should fall back to the only remaining tier
	tier, err := store.ResolveTier(ctx, "unknown", nil)
	require.NoError(t, err)
	require.NotNil(t, tier)
	assert.Equal(t, "Platinum", tier.Name)
}

func TestDeepCopyTier(t *testing.T) {
	teamID := uuid.New()
	original := &CustomerTier{
		ID:              uuid.New(),
		Name:            "Original",
		Level:           1,
		DedicatedTeamID: &teamID,
		Metadata:        map[string]interface{}{"key": "value"},
	}

	copied := deepCopyTier(original)

	// Modify original
	original.Name = "Modified"
	original.Metadata["new_key"] = "new_value"

	// Verify copied is unchanged
	assert.Equal(t, "Original", copied.Name)
	assert.NotContains(t, copied.Metadata, "new_key")
}
