package customer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryStore_Create(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	customer := &Customer{
		Name:        "Acme Corp",
		AccountID:   "acme-001",
		TierID:      "tier-1",
		Description: "Test customer",
		Domains:     []string{"acme.com", "acme.net"},
		IPRanges:    []string{"10.0.0.0/8"},
		Contacts: []CustomerContact{
			{Name: "John Doe", Email: "john@acme.com", Primary: true},
		},
	}

	created, err := store.Create(ctx, customer)
	require.NoError(t, err)
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, "Acme Corp", created.Name)
	assert.Equal(t, "acme-001", created.AccountID)
	assert.NotZero(t, created.CreatedAt)
}

func TestInMemoryStore_Create_DuplicateAccountID(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	customer1 := &Customer{Name: "Acme Corp", AccountID: "acme-001", TierID: "tier-1"}
	_, err := store.Create(ctx, customer1)
	require.NoError(t, err)

	customer2 := &Customer{Name: "Other Corp", AccountID: "acme-001", TierID: "tier-1"}
	_, err = store.Create(ctx, customer2)
	assert.ErrorIs(t, err, ErrDuplicateAccountID)
}

func TestInMemoryStore_Create_Invalid(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_, err := store.Create(ctx, nil)
	assert.ErrorIs(t, err, ErrInvalidCustomer)

	_, err = store.Create(ctx, &Customer{Name: "Test"})
	assert.ErrorIs(t, err, ErrInvalidCustomer)

	_, err = store.Create(ctx, &Customer{AccountID: "test"})
	assert.ErrorIs(t, err, ErrInvalidCustomer)
}

func TestInMemoryStore_GetByID(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	customer := &Customer{Name: "Acme Corp", AccountID: "acme-001", TierID: "tier-1"}
	created, err := store.Create(ctx, customer)
	require.NoError(t, err)

	retrieved, err := store.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, retrieved.ID)
	assert.Equal(t, "Acme Corp", retrieved.Name)
}

func TestInMemoryStore_GetByID_NotFound(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_, err := store.GetByID(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrCustomerNotFound)
}

func TestInMemoryStore_GetByAccountID(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	customer := &Customer{Name: "Acme Corp", AccountID: "acme-001", TierID: "tier-1"}
	_, err := store.Create(ctx, customer)
	require.NoError(t, err)

	retrieved, err := store.GetByAccountID(ctx, "acme-001")
	require.NoError(t, err)
	assert.Equal(t, "Acme Corp", retrieved.Name)
}

func TestInMemoryStore_GetByDomain(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	customer := &Customer{
		Name:      "Acme Corp",
		AccountID: "acme-001",
		TierID:    "tier-1",
		Domains:   []string{"acme.com", "acme.net"},
	}
	_, err := store.Create(ctx, customer)
	require.NoError(t, err)

	// Find by first domain
	retrieved, err := store.GetByDomain(ctx, "acme.com")
	require.NoError(t, err)
	assert.Equal(t, "Acme Corp", retrieved.Name)

	// Find by second domain
	retrieved, err = store.GetByDomain(ctx, "acme.net")
	require.NoError(t, err)
	assert.Equal(t, "Acme Corp", retrieved.Name)

	// Domain not found
	_, err = store.GetByDomain(ctx, "unknown.com")
	assert.ErrorIs(t, err, ErrCustomerNotFound)
}

func TestInMemoryStore_GetByIPRange(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create customer with IP range
	customer := &Customer{
		Name:      "Acme Corp",
		AccountID: "acme-001",
		TierID:    "tier-1",
		IPRanges:  []string{"10.0.0.0/8", "192.168.1.0/24"},
	}
	_, err := store.Create(ctx, customer)
	require.NoError(t, err)

	// IP within first range
	customers, err := store.GetByIPRange(ctx, "10.1.2.3")
	require.NoError(t, err)
	require.Len(t, customers, 1)
	assert.Equal(t, "Acme Corp", customers[0].Name)

	// IP within second range
	customers, err = store.GetByIPRange(ctx, "192.168.1.50")
	require.NoError(t, err)
	require.Len(t, customers, 1)

	// IP outside ranges
	customers, err = store.GetByIPRange(ctx, "172.16.0.1")
	require.NoError(t, err)
	assert.Len(t, customers, 0)
}

func TestInMemoryStore_GetByIPRange_MultipleCustomers(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create overlapping ranges
	customer1 := &Customer{
		Name:      "Acme Corp",
		AccountID: "acme-001",
		TierID:    "tier-1",
		IPRanges:  []string{"10.0.0.0/8"},
	}
	_, err := store.Create(ctx, customer1)
	require.NoError(t, err)

	customer2 := &Customer{
		Name:      "Beta Corp",
		AccountID: "beta-001",
		TierID:    "tier-1",
		IPRanges:  []string{"10.0.0.0/16"}, // More specific, overlaps
	}
	_, err = store.Create(ctx, customer2)
	require.NoError(t, err)

	// Both should match
	customers, err := store.GetByIPRange(ctx, "10.0.1.1")
	require.NoError(t, err)
	assert.Len(t, customers, 2)
}

func TestInMemoryStore_List(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	customers := []*Customer{
		{Name: "Zebra Corp", AccountID: "zebra-001", TierID: "tier-1"},
		{Name: "Acme Corp", AccountID: "acme-001", TierID: "tier-1"},
		{Name: "Beta Corp", AccountID: "beta-001", TierID: "tier-2"},
	}

	for _, c := range customers {
		_, err := store.Create(ctx, c)
		require.NoError(t, err)
	}

	// List all
	result, _, err := store.List(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, result, 3)

	// Should be sorted by name
	assert.Equal(t, "Acme Corp", result[0].Name)
	assert.Equal(t, "Beta Corp", result[1].Name)
	assert.Equal(t, "Zebra Corp", result[2].Name)
}

func TestInMemoryStore_List_FilterByTier(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	customers := []*Customer{
		{Name: "Acme Corp", AccountID: "acme-001", TierID: "tier-1"},
		{Name: "Beta Corp", AccountID: "beta-001", TierID: "tier-2"},
		{Name: "Gamma Corp", AccountID: "gamma-001", TierID: "tier-1"},
	}

	for _, c := range customers {
		_, err := store.Create(ctx, c)
		require.NoError(t, err)
	}

	// Filter by tier
	result, _, err := store.List(ctx, &ListCustomersFilter{TierID: "tier-1"})
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestInMemoryStore_List_FilterByDomain(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	customers := []*Customer{
		{Name: "Acme Corp", AccountID: "acme-001", TierID: "tier-1", Domains: []string{"acme.com"}},
		{Name: "Beta Corp", AccountID: "beta-001", TierID: "tier-1", Domains: []string{"beta.com"}},
	}

	for _, c := range customers {
		_, err := store.Create(ctx, c)
		require.NoError(t, err)
	}

	// Filter by domain
	result, _, err := store.List(ctx, &ListCustomersFilter{Domain: "acme.com"})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Acme Corp", result[0].Name)
}

func TestInMemoryStore_Update(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	customer := &Customer{Name: "Acme Corp", AccountID: "acme-001", TierID: "tier-1"}
	created, err := store.Create(ctx, customer)
	require.NoError(t, err)

	// Update
	created.Name = "Acme Corporation"
	created.TierID = "tier-2"
	updated, err := store.Update(ctx, created)
	require.NoError(t, err)
	assert.Equal(t, "Acme Corporation", updated.Name)
	assert.Equal(t, "tier-2", updated.TierID)

	// Verify persisted
	retrieved, err := store.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Acme Corporation", retrieved.Name)
}

func TestInMemoryStore_Update_NotFound(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	customer := &Customer{ID: "nonexistent", Name: "Test", AccountID: "test", TierID: "tier-1"}
	_, err := store.Update(ctx, customer)
	assert.ErrorIs(t, err, ErrCustomerNotFound)
}

func TestInMemoryStore_Delete(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	customer := &Customer{Name: "Acme Corp", AccountID: "acme-001", TierID: "tier-1"}
	created, err := store.Create(ctx, customer)
	require.NoError(t, err)

	err = store.Delete(ctx, created.ID)
	require.NoError(t, err)

	_, err = store.GetByID(ctx, created.ID)
	assert.ErrorIs(t, err, ErrCustomerNotFound)
}

func TestInMemoryStore_Delete_NotFound(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	err := store.Delete(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrCustomerNotFound)
}

func TestParseIPRanges(t *testing.T) {
	tests := []struct {
		name    string
		cidrs   []string
		wantErr bool
	}{
		{
			name:    "valid ranges",
			cidrs:   []string{"10.0.0.0/8", "192.168.0.0/16"},
			wantErr: false,
		},
		{
			name:    "empty list",
			cidrs:   []string{},
			wantErr: false,
		},
		{
			name:    "invalid CIDR",
			cidrs:   []string{"invalid"},
			wantErr: true,
		},
		{
			name:    "mixed valid and invalid",
			cidrs:   []string{"10.0.0.0/8", "invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranges, err := ParseIPRanges(tt.cidrs)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, ranges, len(tt.cidrs))
			}
		})
	}
}

func TestContainsIP(t *testing.T) {
	ranges, err := ParseIPRanges([]string{"10.0.0.0/8", "192.168.1.0/24"})
	require.NoError(t, err)

	tests := []struct {
		ip       string
		expected bool
	}{
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"192.168.1.50", true},
		{"192.168.2.1", false},
		{"172.16.0.1", false},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			result := ContainsIP(ranges, tt.ip)
			assert.Equal(t, tt.expected, result)
		})
	}
}
