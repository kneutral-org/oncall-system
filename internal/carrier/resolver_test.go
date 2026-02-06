package carrier

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

func setupTestResolver(t *testing.T) (*DefaultResolver, *InMemoryStore) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create test carriers
	carriers := []*Carrier{
		{Name: "Level3", ASN: 3356, Type: CarrierTypeTransit, Priority: 1},
		{Name: "Cogent", ASN: 174, Type: CarrierTypeTransit, Priority: 2},
		{Name: "Hurricane Electric", ASN: 6939, Type: CarrierTypePeering, Priority: 3},
	}
	for _, c := range carriers {
		_, err := store.Create(ctx, c)
		require.NoError(t, err)
	}

	resolver := NewResolver(store)
	return resolver, store
}

func TestResolver_ResolveFromAlert(t *testing.T) {
	resolver, _ := setupTestResolver(t)
	ctx := context.Background()

	t.Run("resolve by asn label", func(t *testing.T) {
		alert := &routingv1.Alert{
			Id:      "alert-1",
			Summary: "BGP Session Down",
			Labels: map[string]string{
				"asn": "3356",
			},
		}

		carrier, err := resolver.ResolveFromAlert(ctx, alert)
		require.NoError(t, err)
		assert.Equal(t, "Level3", carrier.Name)
		assert.Equal(t, 3356, carrier.ASN)
	})

	t.Run("resolve by peer_asn label", func(t *testing.T) {
		alert := &routingv1.Alert{
			Id:      "alert-2",
			Summary: "BGP Peer Unreachable",
			Labels: map[string]string{
				"peer_asn": "174",
			},
		}

		carrier, err := resolver.ResolveFromAlert(ctx, alert)
		require.NoError(t, err)
		assert.Equal(t, "Cogent", carrier.Name)
	})

	t.Run("resolve by remote_asn label", func(t *testing.T) {
		alert := &routingv1.Alert{
			Id:      "alert-3",
			Summary: "Route Flapping",
			Labels: map[string]string{
				"remote_asn": "6939",
			},
		}

		carrier, err := resolver.ResolveFromAlert(ctx, alert)
		require.NoError(t, err)
		assert.Equal(t, "Hurricane Electric", carrier.Name)
	})

	t.Run("resolve by ASN with AS prefix", func(t *testing.T) {
		alert := &routingv1.Alert{
			Id:      "alert-4",
			Summary: "BGP Alert",
			Labels: map[string]string{
				"asn": "AS3356",
			},
		}

		carrier, err := resolver.ResolveFromAlert(ctx, alert)
		require.NoError(t, err)
		assert.Equal(t, "Level3", carrier.Name)
	})

	t.Run("resolve by carrier name label", func(t *testing.T) {
		alert := &routingv1.Alert{
			Id:      "alert-5",
			Summary: "BGP Alert",
			Labels: map[string]string{
				"carrier": "Level3",
			},
		}

		carrier, err := resolver.ResolveFromAlert(ctx, alert)
		require.NoError(t, err)
		assert.Equal(t, 3356, carrier.ASN)
	})

	t.Run("resolve by peer_name label", func(t *testing.T) {
		alert := &routingv1.Alert{
			Id:      "alert-6",
			Summary: "BGP Alert",
			Labels: map[string]string{
				"peer_name": "Cogent",
			},
		}

		carrier, err := resolver.ResolveFromAlert(ctx, alert)
		require.NoError(t, err)
		assert.Equal(t, 174, carrier.ASN)
	})

	t.Run("return error for nil alert", func(t *testing.T) {
		_, err := resolver.ResolveFromAlert(ctx, nil)
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("return error for alert with no labels", func(t *testing.T) {
		alert := &routingv1.Alert{
			Id:      "alert-7",
			Summary: "Some Alert",
		}

		_, err := resolver.ResolveFromAlert(ctx, alert)
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("return error for unknown ASN", func(t *testing.T) {
		alert := &routingv1.Alert{
			Id:      "alert-8",
			Summary: "BGP Alert",
			Labels: map[string]string{
				"asn": "99999",
			},
		}

		_, err := resolver.ResolveFromAlert(ctx, alert)
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestResolver_Resolve(t *testing.T) {
	resolver, _ := setupTestResolver(t)
	ctx := context.Background()

	t.Run("resolve by ASN", func(t *testing.T) {
		carrier, err := resolver.Resolve(ctx, 3356, "")
		require.NoError(t, err)
		assert.Equal(t, "Level3", carrier.Name)
	})

	t.Run("resolve by name", func(t *testing.T) {
		carrier, err := resolver.Resolve(ctx, 0, "Cogent")
		require.NoError(t, err)
		assert.Equal(t, 174, carrier.ASN)
	})

	t.Run("ASN takes precedence over name", func(t *testing.T) {
		carrier, err := resolver.Resolve(ctx, 3356, "Cogent")
		require.NoError(t, err)
		assert.Equal(t, "Level3", carrier.Name) // ASN wins
	})

	t.Run("return error for no match", func(t *testing.T) {
		_, err := resolver.Resolve(ctx, 0, "")
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestParseASN(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"12345", 12345},
		{"AS12345", 12345},
		{"as12345", 12345},
		{"ASN12345", 12345},
		{"  AS12345  ", 12345},
		{"0", 0},              // Invalid
		{"-1", 0},             // Invalid
		{"invalid", 0},        // Invalid
		{"", 0},               // Invalid
		{"4294967296", 0},     // Too large
		{"4294967295", 4294967295}, // Max valid
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseASN(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
