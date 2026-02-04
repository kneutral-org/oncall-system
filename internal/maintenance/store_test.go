// Package maintenance provides the maintenance window store implementation.
package maintenance

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Verify InMemoryMaintenanceWindowStore implements MaintenanceWindowStore interface
var _ MaintenanceWindowStore = (*InMemoryMaintenanceWindowStore)(nil)

func TestInMemoryMaintenanceWindowStore_CreateWindow(t *testing.T) {
	store := NewInMemoryMaintenanceWindowStore()
	ctx := context.Background()

	createdBy := uuid.New()
	window := &MaintenanceWindow{
		Name:             "Network Maintenance",
		Description:      "Monthly network maintenance",
		StartTime:        time.Now().Add(1 * time.Hour),
		EndTime:          time.Now().Add(3 * time.Hour),
		AffectedSites:    []string{"DC1", "DC2"},
		AffectedServices: []string{"network", "dns"},
		AffectedLabels:   map[string]string{"env": "production"},
		Action:           ActionSuppress,
		Status:           StatusScheduled,
		ChangeTicketID:   "CHG-12345",
		CreatedBy:        &createdBy,
	}

	created, err := store.CreateWindow(ctx, window)
	require.NoError(t, err)
	require.NotNil(t, created)

	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, "Network Maintenance", created.Name)
	assert.Equal(t, "Monthly network maintenance", created.Description)
	assert.ElementsMatch(t, []string{"DC1", "DC2"}, created.AffectedSites)
	assert.ElementsMatch(t, []string{"network", "dns"}, created.AffectedServices)
	assert.Equal(t, "production", created.AffectedLabels["env"])
	assert.Equal(t, ActionSuppress, created.Action)
	assert.Equal(t, StatusScheduled, created.Status)
	assert.Equal(t, "CHG-12345", created.ChangeTicketID)
	assert.Equal(t, createdBy, *created.CreatedBy)
	assert.False(t, created.CreatedAt.IsZero())
}

func TestInMemoryMaintenanceWindowStore_CreateWindowDefaults(t *testing.T) {
	store := NewInMemoryMaintenanceWindowStore()
	ctx := context.Background()

	window := &MaintenanceWindow{
		Name:      "Minimal Maintenance",
		StartTime: time.Now().Add(1 * time.Hour),
		EndTime:   time.Now().Add(2 * time.Hour),
	}

	created, err := store.CreateWindow(ctx, window)
	require.NoError(t, err)
	require.NotNil(t, created)

	assert.Equal(t, ActionSuppress, created.Action)
	assert.Equal(t, StatusScheduled, created.Status)
	assert.NotNil(t, created.AffectedSites)
	assert.NotNil(t, created.AffectedServices)
	assert.NotNil(t, created.AffectedLabels)
}

func TestInMemoryMaintenanceWindowStore_GetWindow(t *testing.T) {
	store := NewInMemoryMaintenanceWindowStore()
	ctx := context.Background()

	window := &MaintenanceWindow{
		Name:      "Test Maintenance",
		StartTime: time.Now().Add(1 * time.Hour),
		EndTime:   time.Now().Add(2 * time.Hour),
	}

	created, err := store.CreateWindow(ctx, window)
	require.NoError(t, err)

	// Get existing window
	fetched, err := store.GetWindow(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, "Test Maintenance", fetched.Name)

	// Get non-existing window
	fetched, err = store.GetWindow(ctx, uuid.New())
	require.NoError(t, err)
	assert.Nil(t, fetched)
}

func TestInMemoryMaintenanceWindowStore_ListWindows(t *testing.T) {
	store := NewInMemoryMaintenanceWindowStore()
	ctx := context.Background()

	// Create test windows
	windows := []*MaintenanceWindow{
		{Name: "MW1", StartTime: time.Now().Add(1 * time.Hour), EndTime: time.Now().Add(2 * time.Hour), Status: StatusScheduled, AffectedSites: []string{"DC1"}},
		{Name: "MW2", StartTime: time.Now().Add(1 * time.Hour), EndTime: time.Now().Add(2 * time.Hour), Status: StatusActive, AffectedSites: []string{"DC2"}},
		{Name: "MW3", StartTime: time.Now().Add(1 * time.Hour), EndTime: time.Now().Add(2 * time.Hour), Status: StatusCompleted, AffectedSites: []string{"DC1"}},
		{Name: "MW4", StartTime: time.Now().Add(1 * time.Hour), EndTime: time.Now().Add(2 * time.Hour), Status: StatusCancelled, AffectedSites: []string{"DC3"}},
	}

	for _, w := range windows {
		_, err := store.CreateWindow(ctx, w)
		require.NoError(t, err)
	}

	// List all windows
	result, err := store.ListWindows(ctx, ListParams{})
	require.NoError(t, err)
	assert.Len(t, result, 4)

	// List by status
	result, err = store.ListWindows(ctx, ListParams{StatusFilter: []MaintenanceStatus{StatusScheduled, StatusActive}})
	require.NoError(t, err)
	assert.Len(t, result, 2)

	// List by sites
	result, err = store.ListWindows(ctx, ListParams{SitesFilter: []string{"DC1"}})
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestInMemoryMaintenanceWindowStore_ListActiveWindows(t *testing.T) {
	store := NewInMemoryMaintenanceWindowStore()
	ctx := context.Background()

	now := time.Now()

	// Create test windows
	windows := []*MaintenanceWindow{
		// Active now (scheduled status, within time range)
		{Name: "Active1", StartTime: now.Add(-1 * time.Hour), EndTime: now.Add(1 * time.Hour), Status: StatusScheduled},
		// Active now (active status, within time range)
		{Name: "Active2", StartTime: now.Add(-1 * time.Hour), EndTime: now.Add(1 * time.Hour), Status: StatusActive},
		// Future (scheduled, but not started yet)
		{Name: "Future", StartTime: now.Add(1 * time.Hour), EndTime: now.Add(2 * time.Hour), Status: StatusScheduled},
		// Past (ended)
		{Name: "Past", StartTime: now.Add(-3 * time.Hour), EndTime: now.Add(-1 * time.Hour), Status: StatusCompleted},
		// Within time range but cancelled
		{Name: "Cancelled", StartTime: now.Add(-1 * time.Hour), EndTime: now.Add(1 * time.Hour), Status: StatusCancelled},
	}

	for _, w := range windows {
		_, err := store.CreateWindow(ctx, w)
		require.NoError(t, err)
	}

	// List active windows
	result, err := store.ListActiveWindows(ctx)
	require.NoError(t, err)
	assert.Len(t, result, 2)

	// Verify they are the active ones
	names := make([]string, len(result))
	for i, w := range result {
		names[i] = w.Name
	}
	assert.ElementsMatch(t, []string{"Active1", "Active2"}, names)
}

func TestInMemoryMaintenanceWindowStore_UpdateWindow(t *testing.T) {
	store := NewInMemoryMaintenanceWindowStore()
	ctx := context.Background()

	window := &MaintenanceWindow{
		Name:      "Test Maintenance",
		StartTime: time.Now().Add(1 * time.Hour),
		EndTime:   time.Now().Add(2 * time.Hour),
		Status:    StatusScheduled,
	}

	created, err := store.CreateWindow(ctx, window)
	require.NoError(t, err)

	// Update the window
	created.Name = "Updated Maintenance"
	created.Status = StatusActive
	created.Description = "Updated description"

	updated, err := store.UpdateWindow(ctx, created)
	require.NoError(t, err)
	require.NotNil(t, updated)

	assert.Equal(t, "Updated Maintenance", updated.Name)
	assert.Equal(t, StatusActive, updated.Status)
	assert.Equal(t, "Updated description", updated.Description)

	// Update non-existing window
	nonExisting := &MaintenanceWindow{ID: uuid.New(), Name: "Non-existing"}
	result, err := store.UpdateWindow(ctx, nonExisting)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestInMemoryMaintenanceWindowStore_DeleteWindow(t *testing.T) {
	store := NewInMemoryMaintenanceWindowStore()
	ctx := context.Background()

	window := &MaintenanceWindow{
		Name:      "Test Maintenance",
		StartTime: time.Now().Add(1 * time.Hour),
		EndTime:   time.Now().Add(2 * time.Hour),
	}

	created, err := store.CreateWindow(ctx, window)
	require.NoError(t, err)

	// Delete the window
	err = store.DeleteWindow(ctx, created.ID)
	require.NoError(t, err)

	// Verify window is deleted
	fetched, err := store.GetWindow(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, fetched)
}

func TestInMemoryMaintenanceWindowStore_CheckAlertInMaintenance(t *testing.T) {
	store := NewInMemoryMaintenanceWindowStore()
	ctx := context.Background()

	now := time.Now()

	// Create active maintenance windows
	windows := []*MaintenanceWindow{
		{
			Name:          "Site Maintenance",
			StartTime:     now.Add(-1 * time.Hour),
			EndTime:       now.Add(1 * time.Hour),
			Status:        StatusScheduled,
			AffectedSites: []string{"DC1", "DC2"},
		},
		{
			Name:             "Service Maintenance",
			StartTime:        now.Add(-1 * time.Hour),
			EndTime:          now.Add(1 * time.Hour),
			Status:           StatusActive,
			AffectedServices: []string{"database", "cache"},
		},
		{
			Name:           "Label Maintenance",
			StartTime:      now.Add(-1 * time.Hour),
			EndTime:        now.Add(1 * time.Hour),
			Status:         StatusScheduled,
			AffectedLabels: map[string]string{"env": "staging", "team": "platform"},
		},
	}

	for _, w := range windows {
		_, err := store.CreateWindow(ctx, w)
		require.NoError(t, err)
	}

	// Test: Alert matches site
	window, inMaintenance := store.CheckAlertInMaintenance(ctx, map[string]string{"site": "DC1", "severity": "critical"})
	assert.True(t, inMaintenance)
	assert.NotNil(t, window)
	assert.Equal(t, "Site Maintenance", window.Name)

	// Test: Alert matches service
	window, inMaintenance = store.CheckAlertInMaintenance(ctx, map[string]string{"service": "database", "severity": "warning"})
	assert.True(t, inMaintenance)
	assert.NotNil(t, window)
	assert.Equal(t, "Service Maintenance", window.Name)

	// Test: Alert matches labels
	window, inMaintenance = store.CheckAlertInMaintenance(ctx, map[string]string{"env": "staging", "team": "platform", "alertname": "test"})
	assert.True(t, inMaintenance)
	assert.NotNil(t, window)
	assert.Equal(t, "Label Maintenance", window.Name)

	// Test: Alert does not match (partial label match should not work)
	window, inMaintenance = store.CheckAlertInMaintenance(ctx, map[string]string{"env": "staging", "team": "other"})
	assert.False(t, inMaintenance)
	assert.Nil(t, window)

	// Test: Alert does not match any maintenance window
	window, inMaintenance = store.CheckAlertInMaintenance(ctx, map[string]string{"site": "DC99", "service": "unknown"})
	assert.False(t, inMaintenance)
	assert.Nil(t, window)
}

func TestMaintenanceAction_Constants(t *testing.T) {
	assert.Equal(t, MaintenanceAction("suppress"), ActionSuppress)
	assert.Equal(t, MaintenanceAction("silence"), ActionSilence)
	assert.Equal(t, MaintenanceAction("redirect"), ActionRedirect)
}

func TestMaintenanceStatus_Constants(t *testing.T) {
	assert.Equal(t, MaintenanceStatus("scheduled"), StatusScheduled)
	assert.Equal(t, MaintenanceStatus("active"), StatusActive)
	assert.Equal(t, MaintenanceStatus("completed"), StatusCompleted)
	assert.Equal(t, MaintenanceStatus("cancelled"), StatusCancelled)
}

func TestDeepCopyWindow(t *testing.T) {
	createdBy := uuid.New()
	original := &MaintenanceWindow{
		ID:               uuid.New(),
		Name:             "Original",
		AffectedSites:    []string{"DC1", "DC2"},
		AffectedServices: []string{"service1"},
		AffectedLabels:   map[string]string{"key": "value"},
		CreatedBy:        &createdBy,
	}

	copied := deepCopyWindow(original)

	// Modify original
	original.Name = "Modified"
	original.AffectedSites = append(original.AffectedSites, "DC3")
	original.AffectedLabels["new_key"] = "new_value"

	// Verify copied is unchanged
	assert.Equal(t, "Original", copied.Name)
	assert.Len(t, copied.AffectedSites, 2)
	assert.NotContains(t, copied.AffectedLabels, "new_key")
}
