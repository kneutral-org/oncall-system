// Package site provides the site store implementation.
package site

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Verify InMemorySiteStore implements SiteStore interface
var _ SiteStore = (*InMemorySiteStore)(nil)

func TestInMemorySiteStore_CreateSite(t *testing.T) {
	store := NewInMemorySiteStore()
	ctx := context.Background()

	teamID := uuid.New()
	site := &Site{
		Name:          "DC1 Primary",
		Code:          "DC1",
		SiteType:      SiteTypeDatacenter,
		Region:        "US-East",
		Country:       "USA",
		City:          "New York",
		Timezone:      "America/New_York",
		Tier:          1,
		PrimaryTeamID: &teamID,
		BusinessHours: &BusinessHours{
			DaysOfWeek: []int{1, 2, 3, 4, 5},
			StartTime:  "09:00",
			EndTime:    "17:00",
			Timezone:   "America/New_York",
		},
		Metadata: map[string]interface{}{"rack_count": 100},
	}

	created, err := store.CreateSite(ctx, site)
	require.NoError(t, err)
	require.NotNil(t, created)

	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, "DC1 Primary", created.Name)
	assert.Equal(t, "DC1", created.Code)
	assert.Equal(t, SiteTypeDatacenter, created.SiteType)
	assert.Equal(t, "US-East", created.Region)
	assert.Equal(t, "USA", created.Country)
	assert.Equal(t, "New York", created.City)
	assert.Equal(t, "America/New_York", created.Timezone)
	assert.Equal(t, 1, created.Tier)
	assert.Equal(t, teamID, *created.PrimaryTeamID)
	assert.NotNil(t, created.BusinessHours)
	assert.Equal(t, 100, created.Metadata["rack_count"])
	assert.False(t, created.CreatedAt.IsZero())
	assert.False(t, created.UpdatedAt.IsZero())
}

func TestInMemorySiteStore_CreateSiteDefaults(t *testing.T) {
	store := NewInMemorySiteStore()
	ctx := context.Background()

	site := &Site{
		Name: "Minimal Site",
		Code: "MIN",
	}

	created, err := store.CreateSite(ctx, site)
	require.NoError(t, err)
	require.NotNil(t, created)

	assert.Equal(t, SiteTypeDatacenter, created.SiteType)
	assert.Equal(t, "UTC", created.Timezone)
	assert.Equal(t, 3, created.Tier)
	assert.NotNil(t, created.Metadata)
}

func TestInMemorySiteStore_GetSite(t *testing.T) {
	store := NewInMemorySiteStore()
	ctx := context.Background()

	site := &Site{
		Name: "Test Site",
		Code: "TST",
	}

	created, err := store.CreateSite(ctx, site)
	require.NoError(t, err)

	// Get existing site
	fetched, err := store.GetSite(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, "Test Site", fetched.Name)

	// Get non-existing site
	fetched, err = store.GetSite(ctx, uuid.New())
	require.NoError(t, err)
	assert.Nil(t, fetched)
}

func TestInMemorySiteStore_GetSiteByCode(t *testing.T) {
	store := NewInMemorySiteStore()
	ctx := context.Background()

	site := &Site{
		Name: "Test Site",
		Code: "TST-001",
	}

	created, err := store.CreateSite(ctx, site)
	require.NoError(t, err)

	// Get by code (case insensitive)
	fetched, err := store.GetSiteByCode(ctx, "TST-001")
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, created.ID, fetched.ID)

	// Get by code (lowercase)
	fetched, err = store.GetSiteByCode(ctx, "tst-001")
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, created.ID, fetched.ID)

	// Get non-existing code
	fetched, err = store.GetSiteByCode(ctx, "NONEXISTENT")
	require.NoError(t, err)
	assert.Nil(t, fetched)
}

func TestInMemorySiteStore_ListSites(t *testing.T) {
	store := NewInMemorySiteStore()
	ctx := context.Background()

	// Create test sites
	sites := []*Site{
		{Name: "DC1", Code: "DC1", SiteType: SiteTypeDatacenter, Region: "US-East", Country: "USA", Tier: 1},
		{Name: "DC2", Code: "DC2", SiteType: SiteTypeDatacenter, Region: "US-West", Country: "USA", Tier: 2},
		{Name: "POP1", Code: "POP1", SiteType: SiteTypePOP, Region: "EU-West", Country: "UK", Tier: 3},
		{Name: "Office1", Code: "OFF1", SiteType: SiteTypeOffice, Region: "US-East", Country: "USA", Tier: 4},
	}

	for _, s := range sites {
		_, err := store.CreateSite(ctx, s)
		require.NoError(t, err)
	}

	// List all sites
	result, err := store.ListSites(ctx, ListSitesParams{})
	require.NoError(t, err)
	assert.Len(t, result, 4)

	// List by type
	result, err = store.ListSites(ctx, ListSitesParams{SiteTypeFilter: []SiteType{SiteTypeDatacenter}})
	require.NoError(t, err)
	assert.Len(t, result, 2)

	// List by region
	result, err = store.ListSites(ctx, ListSitesParams{RegionFilter: "US-East"})
	require.NoError(t, err)
	assert.Len(t, result, 2)

	// List by country
	result, err = store.ListSites(ctx, ListSitesParams{CountryFilter: "USA"})
	require.NoError(t, err)
	assert.Len(t, result, 3)

	// List by tier
	result, err = store.ListSites(ctx, ListSitesParams{TierFilter: []int{1, 2}})
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestInMemorySiteStore_ListSitesWithPagination(t *testing.T) {
	store := NewInMemorySiteStore()
	ctx := context.Background()

	// Create test sites
	for i := 0; i < 10; i++ {
		_, err := store.CreateSite(ctx, &Site{
			Name: "Site",
			Code: "S" + string(rune('0'+i)),
		})
		require.NoError(t, err)
	}

	// List with limit
	result, err := store.ListSites(ctx, ListSitesParams{Limit: 5})
	require.NoError(t, err)
	assert.Len(t, result, 5)
}

func TestInMemorySiteStore_UpdateSite(t *testing.T) {
	store := NewInMemorySiteStore()
	ctx := context.Background()

	site := &Site{
		Name:     "Test Site",
		Code:     "TST",
		SiteType: SiteTypeDatacenter,
		Region:   "US-East",
	}

	created, err := store.CreateSite(ctx, site)
	require.NoError(t, err)

	// Update the site
	created.Name = "Updated Site"
	created.Code = "TST-UPDATED"
	created.Region = "US-West"
	teamID := uuid.New()
	created.PrimaryTeamID = &teamID

	updated, err := store.UpdateSite(ctx, created)
	require.NoError(t, err)
	require.NotNil(t, updated)

	assert.Equal(t, "Updated Site", updated.Name)
	assert.Equal(t, "TST-UPDATED", updated.Code)
	assert.Equal(t, "US-West", updated.Region)
	assert.Equal(t, teamID, *updated.PrimaryTeamID)
	assert.True(t, updated.UpdatedAt.After(updated.CreatedAt))

	// Verify code index was updated
	fetched, err := store.GetSiteByCode(ctx, "TST-UPDATED")
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, updated.ID, fetched.ID)

	// Old code should not work
	fetched, err = store.GetSiteByCode(ctx, "TST")
	require.NoError(t, err)
	assert.Nil(t, fetched)

	// Update non-existing site
	nonExisting := &Site{ID: uuid.New(), Name: "Non-existing", Code: "NE"}
	result, err := store.UpdateSite(ctx, nonExisting)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestInMemorySiteStore_DeleteSite(t *testing.T) {
	store := NewInMemorySiteStore()
	ctx := context.Background()

	site := &Site{
		Name: "Test Site",
		Code: "TST",
	}

	created, err := store.CreateSite(ctx, site)
	require.NoError(t, err)

	// Delete the site
	err = store.DeleteSite(ctx, created.ID)
	require.NoError(t, err)

	// Verify site is deleted
	fetched, err := store.GetSite(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, fetched)

	// Verify code index is also deleted
	fetched, err = store.GetSiteByCode(ctx, "TST")
	require.NoError(t, err)
	assert.Nil(t, fetched)
}

func TestSiteType_Constants(t *testing.T) {
	assert.Equal(t, SiteType("datacenter"), SiteTypeDatacenter)
	assert.Equal(t, SiteType("pop"), SiteTypePOP)
	assert.Equal(t, SiteType("office"), SiteTypeOffice)
	assert.Equal(t, SiteType("colocation"), SiteTypeColocation)
}

func TestDeepCopySite(t *testing.T) {
	teamID := uuid.New()
	original := &Site{
		ID:            uuid.New(),
		Name:          "Original",
		Code:          "ORIG",
		PrimaryTeamID: &teamID,
		BusinessHours: &BusinessHours{
			DaysOfWeek: []int{1, 2, 3},
			StartTime:  "09:00",
			EndTime:    "17:00",
		},
		Metadata: map[string]interface{}{"key": "value"},
	}

	copied := deepCopySite(original)

	// Modify original
	original.Name = "Modified"
	original.BusinessHours.DaysOfWeek = append(original.BusinessHours.DaysOfWeek, 4)
	original.Metadata["new_key"] = "new_value"

	// Verify copied is unchanged
	assert.Equal(t, "Original", copied.Name)
	assert.Len(t, copied.BusinessHours.DaysOfWeek, 3)
	assert.NotContains(t, copied.Metadata, "new_key")
}
