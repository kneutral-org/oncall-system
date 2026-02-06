package customer

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestResolver(t *testing.T) (*DefaultResolver, *InMemoryStore, *InMemoryTierStore) {
	customerStore := NewInMemoryStore()
	tierStore := NewInMemoryTierStore()

	config := ResolverConfig{
		CacheTTL: time.Minute,
		Logger:   zerolog.Nop(),
	}

	resolver := NewResolver(customerStore, tierStore, config)
	t.Cleanup(func() {
		resolver.Stop()
	})

	return resolver, customerStore, tierStore
}

func TestResolver_Resolve_DirectLabel(t *testing.T) {
	resolver, customerStore, _ := setupTestResolver(t)
	ctx := context.Background()

	// Create a customer
	customer, err := customerStore.Create(ctx, &Customer{
		Name:      "Acme Corp",
		AccountID: "acme-001",
		TierID:    "tier-1",
	})
	require.NoError(t, err)

	// Resolve by direct customer label
	labels := map[string]string{
		"customer": customer.ID,
	}

	resolved, err := resolver.Resolve(ctx, labels)
	require.NoError(t, err)
	assert.Equal(t, customer.ID, resolved.ID)
	assert.Equal(t, "Acme Corp", resolved.Name)
}

func TestResolver_Resolve_AccountID(t *testing.T) {
	resolver, customerStore, _ := setupTestResolver(t)
	ctx := context.Background()

	// Create a customer
	customer, err := customerStore.Create(ctx, &Customer{
		Name:      "Acme Corp",
		AccountID: "acme-001",
		TierID:    "tier-1",
	})
	require.NoError(t, err)

	// Resolve by account_id label
	labels := map[string]string{
		"account_id": "acme-001",
	}

	resolved, err := resolver.Resolve(ctx, labels)
	require.NoError(t, err)
	assert.Equal(t, customer.ID, resolved.ID)
}

func TestResolver_Resolve_Domain(t *testing.T) {
	resolver, customerStore, _ := setupTestResolver(t)
	ctx := context.Background()

	// Create a customer with domains
	customer, err := customerStore.Create(ctx, &Customer{
		Name:      "Acme Corp",
		AccountID: "acme-001",
		TierID:    "tier-1",
		Domains:   []string{"acme.com", "acme.net"},
	})
	require.NoError(t, err)

	// Resolve by domain label
	labels := map[string]string{
		"domain": "acme.com",
	}

	resolved, err := resolver.Resolve(ctx, labels)
	require.NoError(t, err)
	assert.Equal(t, customer.ID, resolved.ID)
}

func TestResolver_Resolve_IPRange(t *testing.T) {
	resolver, customerStore, _ := setupTestResolver(t)
	ctx := context.Background()

	// Create a customer with IP ranges
	customer, err := customerStore.Create(ctx, &Customer{
		Name:      "Acme Corp",
		AccountID: "acme-001",
		TierID:    "tier-1",
		IPRanges:  []string{"10.0.0.0/8"},
	})
	require.NoError(t, err)

	// Resolve by client_ip label
	labels := map[string]string{
		"client_ip": "10.1.2.3",
	}

	resolved, err := resolver.Resolve(ctx, labels)
	require.NoError(t, err)
	assert.Equal(t, customer.ID, resolved.ID)
}

func TestResolver_Resolve_Priority(t *testing.T) {
	resolver, customerStore, _ := setupTestResolver(t)
	ctx := context.Background()

	// Create two customers
	customer1, err := customerStore.Create(ctx, &Customer{
		Name:      "Customer One",
		AccountID: "cust-001",
		TierID:    "tier-1",
		Domains:   []string{"one.com"},
	})
	require.NoError(t, err)

	customer2, err := customerStore.Create(ctx, &Customer{
		Name:      "Customer Two",
		AccountID: "cust-002",
		TierID:    "tier-1",
		Domains:   []string{"two.com"},
	})
	require.NoError(t, err)

	// When both customer and domain labels are present, customer takes priority
	labels := map[string]string{
		"customer": customer1.ID,
		"domain":   "two.com",
	}

	resolved, err := resolver.Resolve(ctx, labels)
	require.NoError(t, err)
	assert.Equal(t, customer1.ID, resolved.ID)
	_ = customer2 // avoid unused variable
}

func TestResolver_Resolve_Fallback(t *testing.T) {
	resolver, customerStore, _ := setupTestResolver(t)
	ctx := context.Background()

	// Create a customer with domain but no direct ID match
	customer, err := customerStore.Create(ctx, &Customer{
		Name:      "Acme Corp",
		AccountID: "acme-001",
		TierID:    "tier-1",
		Domains:   []string{"acme.com"},
	})
	require.NoError(t, err)

	// When customer label doesn't match, should fall back to domain
	labels := map[string]string{
		"customer": "nonexistent",
		"domain":   "acme.com",
	}

	resolved, err := resolver.Resolve(ctx, labels)
	require.NoError(t, err)
	assert.Equal(t, customer.ID, resolved.ID)
}

func TestResolver_Resolve_NotFound(t *testing.T) {
	resolver, _, _ := setupTestResolver(t)
	ctx := context.Background()

	// No matching labels
	labels := map[string]string{
		"customer": "nonexistent",
	}

	_, err := resolver.Resolve(ctx, labels)
	assert.ErrorIs(t, err, ErrNoCustomerResolved)
}

func TestResolver_Resolve_NilLabels(t *testing.T) {
	resolver, _, _ := setupTestResolver(t)
	ctx := context.Background()

	_, err := resolver.Resolve(ctx, nil)
	assert.ErrorIs(t, err, ErrNoCustomerResolved)
}

func TestResolver_Resolve_EmptyLabels(t *testing.T) {
	resolver, _, _ := setupTestResolver(t)
	ctx := context.Background()

	_, err := resolver.Resolve(ctx, map[string]string{})
	assert.ErrorIs(t, err, ErrNoCustomerResolved)
}

func TestResolver_GetTierConfig(t *testing.T) {
	resolver, customerStore, tierStore := setupTestResolver(t)
	ctx := context.Background()

	// Create a tier
	tier, err := tierStore.Create(ctx, &CustomerTier{
		Name:                 "Enterprise",
		Level:                1,
		CriticalResponseTime: 5 * time.Minute,
		EscalationMultiplier: 0.5,
		SeverityBoost:        1,
	})
	require.NoError(t, err)

	// Create a customer with the tier
	customer, err := customerStore.Create(ctx, &Customer{
		Name:      "Acme Corp",
		AccountID: "acme-001",
		TierID:    tier.ID,
	})
	require.NoError(t, err)

	// Get tier config
	config, err := resolver.GetTierConfig(ctx, customer.ID)
	require.NoError(t, err)
	assert.NotNil(t, config.Tier)
	assert.Equal(t, tier.ID, config.Tier.ID)
	assert.Equal(t, 0.5, config.EscalationMultiplier)
	assert.Equal(t, 1, config.SeverityBoost)
}

func TestResolver_GetTierConfig_TierNotFound(t *testing.T) {
	resolver, customerStore, _ := setupTestResolver(t)
	ctx := context.Background()

	// Create a customer with non-existent tier
	customer, err := customerStore.Create(ctx, &Customer{
		Name:      "Acme Corp",
		AccountID: "acme-001",
		TierID:    "nonexistent",
	})
	require.NoError(t, err)

	// Get tier config - should return defaults
	config, err := resolver.GetTierConfig(ctx, customer.ID)
	require.NoError(t, err)
	assert.Nil(t, config.Tier)
	assert.Equal(t, 1.0, config.EscalationMultiplier)
	assert.Equal(t, 0, config.SeverityBoost)
}

func TestResolver_ResolveWithTier(t *testing.T) {
	resolver, customerStore, tierStore := setupTestResolver(t)
	ctx := context.Background()

	// Create a tier
	tier, err := tierStore.Create(ctx, &CustomerTier{
		Name:                 "Premium",
		Level:                2,
		EscalationMultiplier: 0.75,
	})
	require.NoError(t, err)

	// Create a customer
	customer, err := customerStore.Create(ctx, &Customer{
		Name:      "Acme Corp",
		AccountID: "acme-001",
		TierID:    tier.ID,
	})
	require.NoError(t, err)

	// Resolve with tier
	labels := map[string]string{
		"customer": customer.ID,
	}

	resolvedCustomer, tierConfig, err := resolver.ResolveWithTier(ctx, labels)
	require.NoError(t, err)
	assert.Equal(t, customer.ID, resolvedCustomer.ID)
	assert.NotNil(t, tierConfig.Tier)
	assert.Equal(t, tier.ID, tierConfig.Tier.ID)
	assert.Equal(t, 0.75, tierConfig.EscalationMultiplier)
}

func TestResolver_Cache(t *testing.T) {
	resolver, customerStore, _ := setupTestResolver(t)
	ctx := context.Background()

	// Create a customer
	customer, err := customerStore.Create(ctx, &Customer{
		Name:      "Acme Corp",
		AccountID: "acme-001",
		TierID:    "tier-1",
	})
	require.NoError(t, err)

	labels := map[string]string{
		"customer": customer.ID,
	}

	// First resolve - populates cache
	_, err = resolver.Resolve(ctx, labels)
	require.NoError(t, err)

	// Delete the customer from store
	err = customerStore.Delete(ctx, customer.ID)
	require.NoError(t, err)

	// Second resolve - should still work from cache
	resolved, err := resolver.Resolve(ctx, labels)
	require.NoError(t, err)
	assert.Equal(t, customer.ID, resolved.ID)

	// Invalidate cache
	resolver.InvalidateCache(customer.ID)

	// Third resolve - should fail now
	_, err = resolver.Resolve(ctx, labels)
	assert.ErrorIs(t, err, ErrNoCustomerResolved)
}

func TestResolver_Resolve_IPRangeInvalid(t *testing.T) {
	resolver, customerStore, _ := setupTestResolver(t)
	ctx := context.Background()

	// Create a customer with invalid IP range (will be skipped during lookup)
	_, err := customerStore.Create(ctx, &Customer{
		Name:      "Acme Corp",
		AccountID: "acme-001",
		TierID:    "tier-1",
		IPRanges:  []string{"invalid-cidr"},
	})
	require.NoError(t, err)

	// Should not find due to invalid CIDR
	labels := map[string]string{
		"client_ip": "10.0.0.1",
	}

	_, err = resolver.Resolve(ctx, labels)
	assert.ErrorIs(t, err, ErrNoCustomerResolved)
}
