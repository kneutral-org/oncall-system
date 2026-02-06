package carrier

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

func setupTestBGPHandler(t *testing.T) (*BGPHandler, *InMemoryStore) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create test carriers with different types and priorities
	carriers := []*Carrier{
		{
			Name:             "Level3",
			ASN:              3356,
			Type:             CarrierTypeTransit,
			Priority:         1,
			EscalationPolicy: "transit-escalation",
			Contacts: []Contact{
				{Name: "NOC", Email: "noc@level3.com", Phone: "+1-555-0001", Primary: true},
			},
		},
		{
			Name:     "Cogent",
			ASN:      174,
			Type:     CarrierTypeTransit,
			Priority: 2,
		},
		{
			Name:     "Hurricane Electric",
			ASN:      6939,
			Type:     CarrierTypePeering,
			Priority: 3,
		},
		{
			Name:     "Customer Corp",
			ASN:      65001,
			Type:     CarrierTypeCustomer,
			Priority: 1,
		},
		{
			Name:     "IXP Exchange",
			ASN:      65500,
			Type:     CarrierTypeIXP,
			Priority: 5,
		},
	}
	for _, c := range carriers {
		_, err := store.Create(ctx, c)
		require.NoError(t, err)
	}

	logger := zerolog.Nop()
	handler := NewBGPHandler(store, logger)
	return handler, store
}

func TestBGPHandler_IsBGPAlert(t *testing.T) {
	handler, _ := setupTestBGPHandler(t)

	tests := []struct {
		name     string
		alert    *routingv1.Alert
		expected bool
	}{
		{
			name:     "nil alert",
			alert:    nil,
			expected: false,
		},
		{
			name: "BGP in summary",
			alert: &routingv1.Alert{
				Id:      "1",
				Summary: "BGP Session Down",
			},
			expected: true,
		},
		{
			name: "peer in summary",
			alert: &routingv1.Alert{
				Id:      "2",
				Summary: "Peer Connection Lost",
			},
			expected: true,
		},
		{
			name: "session in summary",
			alert: &routingv1.Alert{
				Id:      "3",
				Summary: "Session Timeout",
			},
			expected: true,
		},
		{
			name: "ASN label present",
			alert: &routingv1.Alert{
				Id:      "4",
				Summary: "Network Alert",
				Labels: map[string]string{
					"asn": "3356",
				},
			},
			expected: true,
		},
		{
			name: "peer_asn label present",
			alert: &routingv1.Alert{
				Id:      "5",
				Summary: "Network Alert",
				Labels: map[string]string{
					"peer_asn": "174",
				},
			},
			expected: true,
		},
		{
			name: "bgp_ prefixed label",
			alert: &routingv1.Alert{
				Id:      "6",
				Summary: "Network Alert",
				Labels: map[string]string{
					"bgp_state": "Idle",
				},
			},
			expected: true,
		},
		{
			name: "alertname contains bgp",
			alert: &routingv1.Alert{
				Id:      "7",
				Summary: "Network Alert",
				Labels: map[string]string{
					"alertname": "BGPSessionDown",
				},
			},
			expected: true,
		},
		{
			name: "unrelated alert",
			alert: &routingv1.Alert{
				Id:      "8",
				Summary: "CPU High",
				Labels: map[string]string{
					"host": "server1",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.IsBGPAlert(tt.alert)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBGPHandler_HandleBGPAlert(t *testing.T) {
	handler, _ := setupTestBGPHandler(t)
	ctx := context.Background()

	t.Run("transit carrier alert - critical impact", func(t *testing.T) {
		alert := &routingv1.Alert{
			Id:      "alert-1",
			Summary: "BGP Session Down",
			Labels: map[string]string{
				"asn":       "3356",
				"remote_ip": "192.0.2.1",
				"state":     "Idle",
				"prefixes":  "5000",
			},
		}

		info, err := handler.HandleBGPAlert(ctx, alert)
		require.NoError(t, err)

		assert.NotNil(t, info.Carrier)
		assert.Equal(t, "Level3", info.Carrier.Name)
		assert.Equal(t, 3356, info.ASN)
		assert.Equal(t, "192.0.2.1", info.RemoteIP)
		assert.Equal(t, "Idle", info.SessionState)
		assert.Equal(t, 5000, info.AffectedPrefixes)
		assert.True(t, info.IsTransit)
		assert.Equal(t, ImpactLevelCritical, info.ImpactLevel)
		assert.True(t, info.TriggerEscalation)
		assert.Equal(t, "transit-escalation", info.EscalationPolicyID)
	})

	t.Run("transit carrier alert - high impact", func(t *testing.T) {
		alert := &routingv1.Alert{
			Id:      "alert-2",
			Summary: "BGP Flapping",
			Labels: map[string]string{
				"asn":      "174",
				"prefixes": "500",
			},
		}

		info, err := handler.HandleBGPAlert(ctx, alert)
		require.NoError(t, err)

		assert.Equal(t, "Cogent", info.Carrier.Name)
		assert.True(t, info.IsTransit)
		assert.Equal(t, ImpactLevelHigh, info.ImpactLevel)
		assert.True(t, info.TriggerEscalation)
	})

	t.Run("peering carrier alert - medium impact", func(t *testing.T) {
		alert := &routingv1.Alert{
			Id:      "alert-3",
			Summary: "BGP Session Down",
			Labels: map[string]string{
				"asn":      "6939",
				"prefixes": "2000",
			},
		}

		info, err := handler.HandleBGPAlert(ctx, alert)
		require.NoError(t, err)

		assert.Equal(t, "Hurricane Electric", info.Carrier.Name)
		assert.False(t, info.IsTransit)
		assert.Equal(t, ImpactLevelMedium, info.ImpactLevel)
	})

	t.Run("peering carrier alert - high impact from many prefixes", func(t *testing.T) {
		alert := &routingv1.Alert{
			Id:      "alert-4",
			Summary: "BGP Session Down",
			Labels: map[string]string{
				"asn":      "6939",
				"prefixes": "10000",
			},
		}

		info, err := handler.HandleBGPAlert(ctx, alert)
		require.NoError(t, err)

		assert.Equal(t, ImpactLevelHigh, info.ImpactLevel)
	})

	t.Run("customer carrier alert - high priority customer", func(t *testing.T) {
		alert := &routingv1.Alert{
			Id:      "alert-5",
			Summary: "BGP Session Down",
			Labels: map[string]string{
				"asn": "65001",
			},
		}

		info, err := handler.HandleBGPAlert(ctx, alert)
		require.NoError(t, err)

		assert.Equal(t, "Customer Corp", info.Carrier.Name)
		assert.Equal(t, ImpactLevelHigh, info.ImpactLevel)
		assert.True(t, info.TriggerEscalation)
	})

	t.Run("unknown carrier - impact based on prefixes", func(t *testing.T) {
		alert := &routingv1.Alert{
			Id:      "alert-6",
			Summary: "BGP Session Down",
			Labels: map[string]string{
				"asn":      "99999",
				"prefixes": "6000",
			},
		}

		info, err := handler.HandleBGPAlert(ctx, alert)
		require.NoError(t, err)

		assert.Nil(t, info.Carrier)
		assert.Equal(t, 99999, info.ASN)
		assert.Equal(t, ImpactLevelCritical, info.ImpactLevel)
	})

	t.Run("alert without ASN", func(t *testing.T) {
		alert := &routingv1.Alert{
			Id:      "alert-7",
			Summary: "BGP Session Down",
			Labels: map[string]string{
				"remote_ip": "192.0.2.1",
			},
		}

		info, err := handler.HandleBGPAlert(ctx, alert)
		require.NoError(t, err)

		assert.Nil(t, info.Carrier)
		assert.Equal(t, 0, info.ASN)
		assert.Equal(t, ImpactLevelLow, info.ImpactLevel)
	})

	t.Run("nil alert returns error", func(t *testing.T) {
		_, err := handler.HandleBGPAlert(ctx, nil)
		assert.Error(t, err)
	})
}

func TestBGPHandler_GetCarrierContacts(t *testing.T) {
	handler, _ := setupTestBGPHandler(t)

	t.Run("nil carrier returns nil", func(t *testing.T) {
		contacts := handler.GetCarrierContacts(nil)
		assert.Nil(t, contacts)
	})

	t.Run("carrier with no contacts returns nil", func(t *testing.T) {
		carrier := &Carrier{Name: "Test", ASN: 12345}
		contacts := handler.GetCarrierContacts(carrier)
		assert.Nil(t, contacts)
	})

	t.Run("primary contacts first", func(t *testing.T) {
		carrier := &Carrier{
			Name: "Test",
			ASN:  12345,
			Contacts: []Contact{
				{Name: "Secondary", Email: "secondary@test.com", Primary: false},
				{Name: "Primary", Email: "primary@test.com", Primary: true},
				{Name: "Third", Email: "third@test.com", Primary: false},
			},
		}
		contacts := handler.GetCarrierContacts(carrier)
		assert.Len(t, contacts, 3)
		assert.Equal(t, "Primary", contacts[0].Name)
		assert.True(t, contacts[0].Primary)
	})
}

func TestImpactLevel_DetermineImpactLevel(t *testing.T) {
	handler, _ := setupTestBGPHandler(t)

	tests := []struct {
		name             string
		carrier          *Carrier
		affectedPrefixes int
		expected         ImpactLevel
	}{
		{
			name:             "transit priority 1 - critical",
			carrier:          &Carrier{Type: CarrierTypeTransit, Priority: 1},
			affectedPrefixes: 100,
			expected:         ImpactLevelCritical,
		},
		{
			name:             "transit high prefixes - critical",
			carrier:          &Carrier{Type: CarrierTypeTransit, Priority: 3},
			affectedPrefixes: 2000,
			expected:         ImpactLevelCritical,
		},
		{
			name:             "transit normal - high",
			carrier:          &Carrier{Type: CarrierTypeTransit, Priority: 3},
			affectedPrefixes: 100,
			expected:         ImpactLevelHigh,
		},
		{
			name:             "provider priority 2 - high",
			carrier:          &Carrier{Type: CarrierTypeProvider, Priority: 2},
			affectedPrefixes: 100,
			expected:         ImpactLevelHigh,
		},
		{
			name:             "provider normal - medium",
			carrier:          &Carrier{Type: CarrierTypeProvider, Priority: 5},
			affectedPrefixes: 100,
			expected:         ImpactLevelMedium,
		},
		{
			name:             "peering many prefixes - high",
			carrier:          &Carrier{Type: CarrierTypePeering, Priority: 5},
			affectedPrefixes: 6000,
			expected:         ImpactLevelHigh,
		},
		{
			name:             "peering normal - low",
			carrier:          &Carrier{Type: CarrierTypePeering, Priority: 5},
			affectedPrefixes: 100,
			expected:         ImpactLevelLow,
		},
		{
			name:             "customer high priority - high",
			carrier:          &Carrier{Type: CarrierTypeCustomer, Priority: 1},
			affectedPrefixes: 10,
			expected:         ImpactLevelHigh,
		},
		{
			name:             "customer normal - medium",
			carrier:          &Carrier{Type: CarrierTypeCustomer, Priority: 5},
			affectedPrefixes: 10,
			expected:         ImpactLevelMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.determineImpactLevel(tt.carrier, tt.affectedPrefixes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestImpactLevel_DetermineImpactLevelWithoutCarrier(t *testing.T) {
	handler, _ := setupTestBGPHandler(t)

	tests := []struct {
		prefixes int
		expected ImpactLevel
	}{
		{10000, ImpactLevelCritical},
		{6000, ImpactLevelCritical},
		{5000, ImpactLevelHigh},
		{2000, ImpactLevelHigh},
		{1000, ImpactLevelMedium},
		{500, ImpactLevelMedium},
		{50, ImpactLevelLow},
		{0, ImpactLevelLow},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := handler.determineImpactLevelWithoutCarrier(tt.prefixes)
			assert.Equal(t, tt.expected, result)
		})
	}
}
