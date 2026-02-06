package equipment

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStore implements Store interface for testing
type mockStore struct {
	equipmentTypes map[string]*EquipmentType
}

func newMockStore() *mockStore {
	return &mockStore{
		equipmentTypes: make(map[string]*EquipmentType),
	}
}

func (m *mockStore) Create(ctx context.Context, eq *EquipmentType) (*EquipmentType, error) {
	m.equipmentTypes[eq.Name] = eq
	return eq, nil
}

func (m *mockStore) GetByID(ctx context.Context, id string) (*EquipmentType, error) {
	for _, eq := range m.equipmentTypes {
		if eq.ID == id {
			return eq, nil
		}
	}
	return nil, ErrEquipmentTypeNotFound
}

func (m *mockStore) GetByName(ctx context.Context, name string) (*EquipmentType, error) {
	eq, ok := m.equipmentTypes[name]
	if !ok {
		return nil, ErrEquipmentTypeNotFound
	}
	return eq, nil
}

func (m *mockStore) List(ctx context.Context, filter *ListEquipmentTypesFilter) ([]*EquipmentType, string, error) {
	var result []*EquipmentType
	for _, eq := range m.equipmentTypes {
		result = append(result, eq)
	}
	return result, "", nil
}

func (m *mockStore) Update(ctx context.Context, eq *EquipmentType) (*EquipmentType, error) {
	if _, ok := m.equipmentTypes[eq.Name]; !ok {
		return nil, ErrEquipmentTypeNotFound
	}
	m.equipmentTypes[eq.Name] = eq
	return eq, nil
}

func (m *mockStore) Delete(ctx context.Context, id string) error {
	for name, eq := range m.equipmentTypes {
		if eq.ID == id {
			delete(m.equipmentTypes, name)
			return nil
		}
	}
	return ErrEquipmentTypeNotFound
}

func (m *mockStore) AddEquipmentType(eq *EquipmentType) {
	m.equipmentTypes[eq.Name] = eq
}

func TestResolver_Resolve_DirectLabel(t *testing.T) {
	store := newMockStore()
	store.AddEquipmentType(&EquipmentType{
		ID:          "eq-1",
		Name:        "router",
		Category:    CategoryNetwork,
		Criticality: 5,
	})

	resolver := NewResolver(store, DefaultResolverConfig())
	defer resolver.Stop()
	ctx := context.Background()

	t.Run("success - equipment_type label", func(t *testing.T) {
		labels := map[string]string{
			"equipment_type": "router",
		}

		result, err := resolver.Resolve(ctx, labels)
		require.NoError(t, err)
		assert.NotNil(t, result.EquipmentType)
		assert.Equal(t, "router", result.EquipmentType.Name)
		assert.Equal(t, ResolutionMethodDirectLabel, result.ResolutionMethod)
		assert.Equal(t, "router", result.MatchedValue)
	})

	t.Run("success - equipment_type label with uppercase", func(t *testing.T) {
		labels := map[string]string{
			"equipment_type": "ROUTER",
		}

		result, err := resolver.Resolve(ctx, labels)
		require.NoError(t, err)
		assert.NotNil(t, result.EquipmentType)
		assert.Equal(t, "router", result.EquipmentType.Name)
	})
}

func TestResolver_Resolve_DeviceTypeLabel(t *testing.T) {
	store := newMockStore()
	store.AddEquipmentType(&EquipmentType{
		ID:          "eq-2",
		Name:        "switch",
		Category:    CategoryNetwork,
		Criticality: 4,
	})

	resolver := NewResolver(store, DefaultResolverConfig())
	defer resolver.Stop()
	ctx := context.Background()

	t.Run("success - device_type label", func(t *testing.T) {
		labels := map[string]string{
			"device_type": "switch",
		}

		result, err := resolver.Resolve(ctx, labels)
		require.NoError(t, err)
		assert.NotNil(t, result.EquipmentType)
		assert.Equal(t, "switch", result.EquipmentType.Name)
		assert.Equal(t, ResolutionMethodDeviceType, result.ResolutionMethod)
	})
}

func TestResolver_Resolve_JobPattern(t *testing.T) {
	store := newMockStore()
	store.AddEquipmentType(&EquipmentType{
		ID:          "eq-3",
		Name:        "firewall",
		Category:    CategorySecurity,
		Criticality: 5,
	})

	resolver := NewResolver(store, DefaultResolverConfig())
	defer resolver.Stop()
	ctx := context.Background()

	t.Run("success - job pattern contains firewall", func(t *testing.T) {
		labels := map[string]string{
			"job": "firewall-monitoring",
		}

		result, err := resolver.Resolve(ctx, labels)
		require.NoError(t, err)
		assert.NotNil(t, result.EquipmentType)
		assert.Equal(t, "firewall", result.EquipmentType.Name)
		assert.Equal(t, ResolutionMethodJobPattern, result.ResolutionMethod)
	})

	t.Run("success - job pattern contains Firewall (case insensitive)", func(t *testing.T) {
		labels := map[string]string{
			"job": "Firewall_Metrics",
		}

		result, err := resolver.Resolve(ctx, labels)
		require.NoError(t, err)
		assert.NotNil(t, result.EquipmentType)
	})
}

func TestResolver_Resolve_HostnamePattern(t *testing.T) {
	store := newMockStore()
	store.AddEquipmentType(&EquipmentType{
		ID:          "eq-1",
		Name:        "router",
		Category:    CategoryNetwork,
		Criticality: 5,
	})
	store.AddEquipmentType(&EquipmentType{
		ID:          "eq-2",
		Name:        "switch",
		Category:    CategoryNetwork,
		Criticality: 4,
	})
	store.AddEquipmentType(&EquipmentType{
		ID:          "eq-3",
		Name:        "firewall",
		Category:    CategorySecurity,
		Criticality: 5,
	})
	store.AddEquipmentType(&EquipmentType{
		ID:          "eq-4",
		Name:        "server",
		Category:    CategoryCompute,
		Criticality: 3,
	})
	store.AddEquipmentType(&EquipmentType{
		ID:          "eq-5",
		Name:        "load_balancer",
		Category:    CategoryNetwork,
		Criticality: 5,
	})

	resolver := NewResolver(store, DefaultResolverConfig())
	defer resolver.Stop()
	ctx := context.Background()

	tests := []struct {
		name         string
		instance     string
		expectedType string
	}{
		{"router with rtr prefix", "rtr-nyc-01", "router"},
		{"router with rt prefix", "rt-nyc-02", "router"},
		{"switch with sw prefix", "sw-dc1-core-01", "switch"},
		{"firewall with fw prefix", "fw-edge-01:9100", "firewall"},
		{"server with srv prefix", "srv-app-001", "server"},
		{"load balancer with lb prefix", "lb-frontend-01", "load_balancer"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			labels := map[string]string{
				"instance": tc.instance,
			}

			result, err := resolver.Resolve(ctx, labels)
			require.NoError(t, err)
			assert.NotNil(t, result.EquipmentType)
			assert.Equal(t, tc.expectedType, result.EquipmentType.Name)
			assert.Equal(t, ResolutionMethodHostnamePrefix, result.ResolutionMethod)
		})
	}
}

func TestResolver_Resolve_Priority(t *testing.T) {
	store := newMockStore()
	store.AddEquipmentType(&EquipmentType{
		ID:          "eq-1",
		Name:        "router",
		Category:    CategoryNetwork,
		Criticality: 5,
	})
	store.AddEquipmentType(&EquipmentType{
		ID:          "eq-2",
		Name:        "switch",
		Category:    CategoryNetwork,
		Criticality: 4,
	})

	resolver := NewResolver(store, DefaultResolverConfig())
	defer resolver.Stop()
	ctx := context.Background()

	t.Run("equipment_type label takes priority", func(t *testing.T) {
		labels := map[string]string{
			"equipment_type": "router",
			"device_type":    "switch",
			"job":            "switch-monitoring",
			"instance":       "sw-core-01",
		}

		result, err := resolver.Resolve(ctx, labels)
		require.NoError(t, err)
		assert.Equal(t, "router", result.EquipmentType.Name)
		assert.Equal(t, ResolutionMethodDirectLabel, result.ResolutionMethod)
	})

	t.Run("device_type takes priority over job", func(t *testing.T) {
		labels := map[string]string{
			"device_type": "router",
			"job":         "switch-monitoring",
			"instance":    "sw-core-01",
		}

		result, err := resolver.Resolve(ctx, labels)
		require.NoError(t, err)
		assert.Equal(t, "router", result.EquipmentType.Name)
		assert.Equal(t, ResolutionMethodDeviceType, result.ResolutionMethod)
	})
}

func TestResolver_Resolve_NotFound(t *testing.T) {
	store := newMockStore()
	resolver := NewResolver(store, DefaultResolverConfig())
	defer resolver.Stop()
	ctx := context.Background()

	t.Run("no labels", func(t *testing.T) {
		_, err := resolver.Resolve(ctx, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNoEquipmentResolved)
	})

	t.Run("empty labels", func(t *testing.T) {
		_, err := resolver.Resolve(ctx, map[string]string{})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNoEquipmentResolved)
	})

	t.Run("equipment type not in store", func(t *testing.T) {
		labels := map[string]string{
			"equipment_type": "unknown_device",
		}

		_, err := resolver.Resolve(ctx, labels)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNoEquipmentResolved)
	})

	t.Run("hostname pattern not matching known type", func(t *testing.T) {
		labels := map[string]string{
			"instance": "unknown-device-01",
		}

		_, err := resolver.Resolve(ctx, labels)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNoEquipmentResolved)
	})
}

func TestResolver_Cache(t *testing.T) {
	store := newMockStore()
	store.AddEquipmentType(&EquipmentType{
		ID:          "eq-1",
		Name:        "router",
		Category:    CategoryNetwork,
		Criticality: 5,
	})

	config := DefaultResolverConfig()
	config.CacheTTL = 100 * time.Millisecond
	resolver := NewResolver(store, config)
	defer resolver.Stop()
	ctx := context.Background()

	labels := map[string]string{
		"equipment_type": "router",
	}

	// First call - should hit store
	result1, err := resolver.Resolve(ctx, labels)
	require.NoError(t, err)
	assert.NotNil(t, result1.EquipmentType)

	// Delete from store - cache should still work
	delete(store.equipmentTypes, "router")

	// Second call - should hit cache
	result2, err := resolver.Resolve(ctx, labels)
	require.NoError(t, err)
	assert.NotNil(t, result2.EquipmentType)
	assert.Equal(t, result1.EquipmentType.ID, result2.EquipmentType.ID)

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Third call - should fail since store is empty
	_, err = resolver.Resolve(ctx, labels)
	require.Error(t, err)
}

func TestResolver_InvalidateCache(t *testing.T) {
	store := newMockStore()
	store.AddEquipmentType(&EquipmentType{
		ID:          "eq-1",
		Name:        "router",
		Category:    CategoryNetwork,
		Criticality: 5,
	})

	resolver := NewResolver(store, DefaultResolverConfig())
	defer resolver.Stop()
	ctx := context.Background()

	labels := map[string]string{
		"equipment_type": "router",
	}

	// First call - should hit store and cache
	result1, err := resolver.Resolve(ctx, labels)
	require.NoError(t, err)
	assert.NotNil(t, result1.EquipmentType)

	// Invalidate cache
	resolver.InvalidateCache("router")

	// Delete from store
	delete(store.equipmentTypes, "router")

	// Should fail since cache was invalidated and store is empty
	_, err = resolver.Resolve(ctx, labels)
	require.Error(t, err)
}

func TestExtractFromHostname(t *testing.T) {
	tests := []struct {
		hostname string
		expected string
		found    bool
	}{
		{"rtr-nyc-01", "router", true},
		{"rtr_nyc_01", "router", true},
		{"rt-core-001", "router", true},
		{"router-edge-01", "router", true},
		{"sw-dc1-tor-01", "switch", true},
		{"switch-core-01", "switch", true},
		{"fw-perimeter-01", "firewall", true},
		{"firewall-dmz-01", "firewall", true},
		{"srv-app-001", "server", true},
		{"server-db-001", "server", true},
		{"lb-frontend-01", "load_balancer", true},
		{"loadbalancer-web-01", "load_balancer", true},
		{"ap-floor3-01", "access_point", true},
		{"wap-office-01", "access_point", true},
		{"nas-backup-01", "storage", true},
		{"san-primary-01", "storage", true},
		{"pdu-rack01-a", "pdu", true},
		{"ups-room1-01", "ups", true},
		{"unknown-device-01", "", false},
		{"myserver-01", "", false},
		{"rtr-nyc-01:9100", "router", true}, // with port
		{"SW-CORE-01", "switch", true},      // uppercase
	}

	for _, tc := range tests {
		t.Run(tc.hostname, func(t *testing.T) {
			result, found := extractFromHostname(tc.hostname)
			assert.Equal(t, tc.found, found)
			if found {
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestExtractFromJobName(t *testing.T) {
	tests := []struct {
		job      string
		expected string
		found    bool
	}{
		{"router-monitoring", "router", true},
		{"switch_metrics", "switch", true},
		{"firewall-alerts", "firewall", true},
		{"server-health", "server", true},
		{"load-balancer-status", "load_balancer", true},
		{"loadbalancer_metrics", "load_balancer", true},
		{"storage-iops", "storage", true},
		{"pdu-power", "pdu", true},
		{"ups-status", "ups", true},
		{"unknown-service", "", false},
		{"ROUTER_MONITORING", "router", true}, // case insensitive
		{"FireWall-Status", "firewall", true}, // mixed case
	}

	for _, tc := range tests {
		t.Run(tc.job, func(t *testing.T) {
			result, found := extractFromJobName(tc.job)
			assert.Equal(t, tc.found, found)
			if found {
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestNormalizeEquipmentName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Router", "router"},
		{"  router  ", "router"},
		{"Core Router", "core_router"},
		{"load-balancer", "load_balancer"},
		{"FIREWALL", "firewall"},
		{"Access Point", "access_point"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := normalizeEquipmentName(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
