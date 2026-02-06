package customer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryTierStore_Create(t *testing.T) {
	store := NewInMemoryTierStore()
	ctx := context.Background()

	tier := &CustomerTier{
		Name:                 "Enterprise",
		Level:                1,
		Description:          "Enterprise tier",
		CriticalResponseTime: 5 * time.Minute,
		HighResponseTime:     30 * time.Minute,
		MediumResponseTime:   2 * time.Hour,
		LowResponseTime:      8 * time.Hour,
		EscalationMultiplier: 0.5,
		SeverityBoost:        1,
	}

	created, err := store.Create(ctx, tier)
	require.NoError(t, err)
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, "Enterprise", created.Name)
	assert.Equal(t, 1, created.Level)
	assert.NotZero(t, created.CreatedAt)
	assert.NotZero(t, created.UpdatedAt)
}

func TestInMemoryTierStore_Create_DuplicateName(t *testing.T) {
	store := NewInMemoryTierStore()
	ctx := context.Background()

	tier1 := &CustomerTier{Name: "Enterprise", Level: 1}
	_, err := store.Create(ctx, tier1)
	require.NoError(t, err)

	tier2 := &CustomerTier{Name: "Enterprise", Level: 2}
	_, err = store.Create(ctx, tier2)
	assert.ErrorIs(t, err, ErrDuplicateTierName)
}

func TestInMemoryTierStore_Create_DuplicateLevel(t *testing.T) {
	store := NewInMemoryTierStore()
	ctx := context.Background()

	tier1 := &CustomerTier{Name: "Enterprise", Level: 1}
	_, err := store.Create(ctx, tier1)
	require.NoError(t, err)

	tier2 := &CustomerTier{Name: "Premium", Level: 1}
	_, err = store.Create(ctx, tier2)
	assert.ErrorIs(t, err, ErrDuplicateTierLevel)
}

func TestInMemoryTierStore_Create_Invalid(t *testing.T) {
	store := NewInMemoryTierStore()
	ctx := context.Background()

	_, err := store.Create(ctx, nil)
	assert.ErrorIs(t, err, ErrInvalidTier)

	_, err = store.Create(ctx, &CustomerTier{})
	assert.ErrorIs(t, err, ErrInvalidTier)
}

func TestInMemoryTierStore_GetByID(t *testing.T) {
	store := NewInMemoryTierStore()
	ctx := context.Background()

	tier := &CustomerTier{Name: "Enterprise", Level: 1}
	created, err := store.Create(ctx, tier)
	require.NoError(t, err)

	retrieved, err := store.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, retrieved.ID)
	assert.Equal(t, "Enterprise", retrieved.Name)
}

func TestInMemoryTierStore_GetByID_NotFound(t *testing.T) {
	store := NewInMemoryTierStore()
	ctx := context.Background()

	_, err := store.GetByID(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrTierNotFound)
}

func TestInMemoryTierStore_GetByName(t *testing.T) {
	store := NewInMemoryTierStore()
	ctx := context.Background()

	tier := &CustomerTier{Name: "Enterprise", Level: 1}
	_, err := store.Create(ctx, tier)
	require.NoError(t, err)

	retrieved, err := store.GetByName(ctx, "Enterprise")
	require.NoError(t, err)
	assert.Equal(t, "Enterprise", retrieved.Name)
}

func TestInMemoryTierStore_GetByLevel(t *testing.T) {
	store := NewInMemoryTierStore()
	ctx := context.Background()

	tier := &CustomerTier{Name: "Enterprise", Level: 1}
	_, err := store.Create(ctx, tier)
	require.NoError(t, err)

	retrieved, err := store.GetByLevel(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, retrieved.Level)
	assert.Equal(t, "Enterprise", retrieved.Name)
}

func TestInMemoryTierStore_List(t *testing.T) {
	store := NewInMemoryTierStore()
	ctx := context.Background()

	// Create multiple tiers
	tiers := []*CustomerTier{
		{Name: "Standard", Level: 3},
		{Name: "Enterprise", Level: 1},
		{Name: "Premium", Level: 2},
	}

	for _, tier := range tiers {
		_, err := store.Create(ctx, tier)
		require.NoError(t, err)
	}

	// List all tiers
	result, _, err := store.List(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, result, 3)

	// Verify sorted by level
	assert.Equal(t, 1, result[0].Level)
	assert.Equal(t, 2, result[1].Level)
	assert.Equal(t, 3, result[2].Level)
}

func TestInMemoryTierStore_Update(t *testing.T) {
	store := NewInMemoryTierStore()
	ctx := context.Background()

	tier := &CustomerTier{Name: "Enterprise", Level: 1, Description: "Original"}
	created, err := store.Create(ctx, tier)
	require.NoError(t, err)

	// Update
	created.Description = "Updated"
	created.SeverityBoost = 2
	updated, err := store.Update(ctx, created)
	require.NoError(t, err)
	assert.Equal(t, "Updated", updated.Description)
	assert.Equal(t, 2, updated.SeverityBoost)

	// Verify persisted
	retrieved, err := store.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated", retrieved.Description)
}

func TestInMemoryTierStore_Update_NotFound(t *testing.T) {
	store := NewInMemoryTierStore()
	ctx := context.Background()

	tier := &CustomerTier{ID: "nonexistent", Name: "Test", Level: 1}
	_, err := store.Update(ctx, tier)
	assert.ErrorIs(t, err, ErrTierNotFound)
}

func TestInMemoryTierStore_Delete(t *testing.T) {
	store := NewInMemoryTierStore()
	ctx := context.Background()

	tier := &CustomerTier{Name: "Enterprise", Level: 1}
	created, err := store.Create(ctx, tier)
	require.NoError(t, err)

	err = store.Delete(ctx, created.ID)
	require.NoError(t, err)

	_, err = store.GetByID(ctx, created.ID)
	assert.ErrorIs(t, err, ErrTierNotFound)
}

func TestInMemoryTierStore_Delete_NotFound(t *testing.T) {
	store := NewInMemoryTierStore()
	ctx := context.Background()

	err := store.Delete(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrTierNotFound)
}

func TestInMemoryTierStore_DefaultEscalationMultiplier(t *testing.T) {
	store := NewInMemoryTierStore()
	ctx := context.Background()

	// Create tier without escalation multiplier
	tier := &CustomerTier{Name: "Enterprise", Level: 1}
	created, err := store.Create(ctx, tier)
	require.NoError(t, err)

	// Should default to 1.0
	assert.Equal(t, 1.0, created.EscalationMultiplier)
}
