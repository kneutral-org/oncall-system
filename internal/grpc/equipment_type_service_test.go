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

	"github.com/kneutral-org/alerting-system/internal/equipment"
	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// mockEquipmentStore implements equipment.Store for testing
type mockEquipmentStore struct {
	equipmentTypes map[string]*equipment.EquipmentType
}

func newMockEquipmentStore() *mockEquipmentStore {
	return &mockEquipmentStore{
		equipmentTypes: make(map[string]*equipment.EquipmentType),
	}
}

func (m *mockEquipmentStore) Create(ctx context.Context, eq *equipment.EquipmentType) (*equipment.EquipmentType, error) {
	if eq.Name == "" {
		return nil, equipment.ErrInvalidEquipmentType
	}
	if _, exists := m.equipmentTypes[eq.Name]; exists {
		return nil, equipment.ErrDuplicateName
	}
	if eq.ID == "" {
		eq.ID = "generated-id"
	}
	eq.CreatedAt = time.Now()
	eq.UpdatedAt = time.Now()
	m.equipmentTypes[eq.Name] = eq
	return eq, nil
}

func (m *mockEquipmentStore) GetByID(ctx context.Context, id string) (*equipment.EquipmentType, error) {
	for _, eq := range m.equipmentTypes {
		if eq.ID == id {
			return eq, nil
		}
	}
	return nil, equipment.ErrEquipmentTypeNotFound
}

func (m *mockEquipmentStore) GetByName(ctx context.Context, name string) (*equipment.EquipmentType, error) {
	eq, ok := m.equipmentTypes[name]
	if !ok {
		return nil, equipment.ErrEquipmentTypeNotFound
	}
	return eq, nil
}

func (m *mockEquipmentStore) List(ctx context.Context, filter *equipment.ListEquipmentTypesFilter) ([]*equipment.EquipmentType, string, error) {
	var result []*equipment.EquipmentType
	for _, eq := range m.equipmentTypes {
		if filter != nil {
			if filter.Category != "" && eq.Category != filter.Category {
				continue
			}
			if filter.Vendor != "" && eq.Vendor != filter.Vendor {
				continue
			}
		}
		result = append(result, eq)
	}
	return result, "", nil
}

func (m *mockEquipmentStore) Update(ctx context.Context, eq *equipment.EquipmentType) (*equipment.EquipmentType, error) {
	if eq.ID == "" {
		return nil, equipment.ErrInvalidEquipmentType
	}
	for name, existing := range m.equipmentTypes {
		if existing.ID == eq.ID {
			eq.UpdatedAt = time.Now()
			delete(m.equipmentTypes, name)
			m.equipmentTypes[eq.Name] = eq
			return eq, nil
		}
	}
	return nil, equipment.ErrEquipmentTypeNotFound
}

func (m *mockEquipmentStore) Delete(ctx context.Context, id string) error {
	for name, eq := range m.equipmentTypes {
		if eq.ID == id {
			delete(m.equipmentTypes, name)
			return nil
		}
	}
	return equipment.ErrEquipmentTypeNotFound
}

func (m *mockEquipmentStore) AddEquipmentType(eq *equipment.EquipmentType) {
	m.equipmentTypes[eq.Name] = eq
}

// mockEquipmentResolver implements equipment.Resolver for testing
type mockEquipmentResolver struct {
	store            *mockEquipmentStore
	invalidatedNames []string
}

func newMockEquipmentResolver(store *mockEquipmentStore) *mockEquipmentResolver {
	return &mockEquipmentResolver{
		store:            store,
		invalidatedNames: []string{},
	}
}

func (m *mockEquipmentResolver) Resolve(ctx context.Context, labels map[string]string) (*equipment.ResolvedEquipment, error) {
	// Check equipment_type label first
	if name, ok := labels["equipment_type"]; ok {
		eq, err := m.store.GetByName(ctx, name)
		if err != nil {
			return nil, equipment.ErrNoEquipmentResolved
		}
		return &equipment.ResolvedEquipment{
			EquipmentType:    eq,
			ResolutionMethod: equipment.ResolutionMethodDirectLabel,
			MatchedValue:     name,
		}, nil
	}
	return nil, equipment.ErrNoEquipmentResolved
}

func (m *mockEquipmentResolver) InvalidateCache(name string) {
	m.invalidatedNames = append(m.invalidatedNames, name)
}

func (m *mockEquipmentResolver) Stop() {}

func TestEquipmentTypeService_CreateEquipmentType(t *testing.T) {
	store := newMockEquipmentStore()
	resolver := newMockEquipmentResolver(store)
	logger := zerolog.Nop()
	svc := NewEquipmentTypeService(store, resolver, logger)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		req := &routingv1.CreateEquipmentTypeRequest{
			EquipmentType: &routingv1.EquipmentType{
				Name:          "router",
				Category:      "network",
				SeverityBoost: 5,
			},
		}

		resp, err := svc.CreateEquipmentType(ctx, req)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Id)
		assert.Equal(t, "router", resp.Name)
		assert.Equal(t, "network", resp.Category)
		assert.Equal(t, int32(5), resp.SeverityBoost)
	})

	t.Run("error - nil equipment type", func(t *testing.T) {
		req := &routingv1.CreateEquipmentTypeRequest{}

		_, err := svc.CreateEquipmentType(ctx, req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("error - empty name", func(t *testing.T) {
		req := &routingv1.CreateEquipmentTypeRequest{
			EquipmentType: &routingv1.EquipmentType{
				Category: "network",
			},
		}

		_, err := svc.CreateEquipmentType(ctx, req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("error - duplicate name", func(t *testing.T) {
		req := &routingv1.CreateEquipmentTypeRequest{
			EquipmentType: &routingv1.EquipmentType{
				Name:     "router", // Already created
				Category: "network",
			},
		}

		_, err := svc.CreateEquipmentType(ctx, req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.AlreadyExists, st.Code())
	})
}

func TestEquipmentTypeService_GetEquipmentType(t *testing.T) {
	store := newMockEquipmentStore()
	store.AddEquipmentType(&equipment.EquipmentType{
		ID:          "eq-123",
		Name:        "router",
		Category:    equipment.CategoryNetwork,
		Criticality: 5,
	})
	resolver := newMockEquipmentResolver(store)
	logger := zerolog.Nop()
	svc := NewEquipmentTypeService(store, resolver, logger)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		req := &routingv1.GetEquipmentTypeRequest{Id: "eq-123"}

		resp, err := svc.GetEquipmentType(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "eq-123", resp.Id)
		assert.Equal(t, "router", resp.Name)
	})

	t.Run("error - empty id", func(t *testing.T) {
		req := &routingv1.GetEquipmentTypeRequest{}

		_, err := svc.GetEquipmentType(ctx, req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("error - not found", func(t *testing.T) {
		req := &routingv1.GetEquipmentTypeRequest{Id: "nonexistent"}

		_, err := svc.GetEquipmentType(ctx, req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

func TestEquipmentTypeService_GetEquipmentTypeByName(t *testing.T) {
	store := newMockEquipmentStore()
	store.AddEquipmentType(&equipment.EquipmentType{
		ID:          "eq-123",
		Name:        "switch",
		Category:    equipment.CategoryNetwork,
		Criticality: 4,
	})
	resolver := newMockEquipmentResolver(store)
	logger := zerolog.Nop()
	svc := NewEquipmentTypeService(store, resolver, logger)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		req := &routingv1.GetEquipmentTypeByNameRequest{Name: "switch"}

		resp, err := svc.GetEquipmentTypeByName(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "eq-123", resp.Id)
		assert.Equal(t, "switch", resp.Name)
	})

	t.Run("error - empty name", func(t *testing.T) {
		req := &routingv1.GetEquipmentTypeByNameRequest{}

		_, err := svc.GetEquipmentTypeByName(ctx, req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("error - not found", func(t *testing.T) {
		req := &routingv1.GetEquipmentTypeByNameRequest{Name: "nonexistent"}

		_, err := svc.GetEquipmentTypeByName(ctx, req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

func TestEquipmentTypeService_ListEquipmentTypes(t *testing.T) {
	store := newMockEquipmentStore()
	store.AddEquipmentType(&equipment.EquipmentType{
		ID:          "eq-1",
		Name:        "router",
		Category:    equipment.CategoryNetwork,
		Vendor:      "cisco",
		Criticality: 5,
	})
	store.AddEquipmentType(&equipment.EquipmentType{
		ID:          "eq-2",
		Name:        "firewall",
		Category:    equipment.CategorySecurity,
		Vendor:      "paloalto",
		Criticality: 5,
	})
	resolver := newMockEquipmentResolver(store)
	logger := zerolog.Nop()
	svc := NewEquipmentTypeService(store, resolver, logger)
	ctx := context.Background()

	t.Run("success - no filters", func(t *testing.T) {
		req := &routingv1.ListEquipmentTypesRequest{}

		resp, err := svc.ListEquipmentTypes(ctx, req)
		require.NoError(t, err)
		assert.Len(t, resp.EquipmentTypes, 2)
	})

	t.Run("success - filter by category", func(t *testing.T) {
		req := &routingv1.ListEquipmentTypesRequest{Category: "network"}

		resp, err := svc.ListEquipmentTypes(ctx, req)
		require.NoError(t, err)
		assert.Len(t, resp.EquipmentTypes, 1)
		assert.Equal(t, "router", resp.EquipmentTypes[0].Name)
	})

	t.Run("success - filter by vendor", func(t *testing.T) {
		req := &routingv1.ListEquipmentTypesRequest{Vendor: "paloalto"}

		resp, err := svc.ListEquipmentTypes(ctx, req)
		require.NoError(t, err)
		assert.Len(t, resp.EquipmentTypes, 1)
		assert.Equal(t, "firewall", resp.EquipmentTypes[0].Name)
	})
}

func TestEquipmentTypeService_UpdateEquipmentType(t *testing.T) {
	store := newMockEquipmentStore()
	store.AddEquipmentType(&equipment.EquipmentType{
		ID:          "eq-123",
		Name:        "router",
		Category:    equipment.CategoryNetwork,
		Criticality: 5,
	})
	resolver := newMockEquipmentResolver(store)
	logger := zerolog.Nop()
	svc := NewEquipmentTypeService(store, resolver, logger)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		req := &routingv1.UpdateEquipmentTypeRequest{
			EquipmentType: &routingv1.EquipmentType{
				Id:            "eq-123",
				Name:          "core_router",
				Category:      "network",
				SeverityBoost: 5,
			},
		}

		resp, err := svc.UpdateEquipmentType(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "eq-123", resp.Id)
		assert.Equal(t, "core_router", resp.Name)

		// Check cache was invalidated
		assert.Contains(t, resolver.invalidatedNames, "core_router")
	})

	t.Run("error - nil equipment type", func(t *testing.T) {
		req := &routingv1.UpdateEquipmentTypeRequest{}

		_, err := svc.UpdateEquipmentType(ctx, req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("error - not found", func(t *testing.T) {
		req := &routingv1.UpdateEquipmentTypeRequest{
			EquipmentType: &routingv1.EquipmentType{
				Id:   "nonexistent",
				Name: "test",
			},
		}

		_, err := svc.UpdateEquipmentType(ctx, req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

func TestEquipmentTypeService_DeleteEquipmentType(t *testing.T) {
	store := newMockEquipmentStore()
	store.AddEquipmentType(&equipment.EquipmentType{
		ID:          "eq-123",
		Name:        "router",
		Category:    equipment.CategoryNetwork,
		Criticality: 5,
	})
	resolver := newMockEquipmentResolver(store)
	logger := zerolog.Nop()
	svc := NewEquipmentTypeService(store, resolver, logger)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		req := &routingv1.DeleteEquipmentTypeRequest{Id: "eq-123"}

		resp, err := svc.DeleteEquipmentType(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Success)

		// Check cache was invalidated
		assert.Contains(t, resolver.invalidatedNames, "router")
	})

	t.Run("error - empty id", func(t *testing.T) {
		req := &routingv1.DeleteEquipmentTypeRequest{}

		_, err := svc.DeleteEquipmentType(ctx, req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("error - not found", func(t *testing.T) {
		req := &routingv1.DeleteEquipmentTypeRequest{Id: "nonexistent"}

		_, err := svc.DeleteEquipmentType(ctx, req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

func TestEquipmentTypeService_ResolveEquipmentType(t *testing.T) {
	store := newMockEquipmentStore()
	store.AddEquipmentType(&equipment.EquipmentType{
		ID:          "eq-123",
		Name:        "router",
		Category:    equipment.CategoryNetwork,
		Criticality: 5,
	})
	resolver := newMockEquipmentResolver(store)
	logger := zerolog.Nop()
	svc := NewEquipmentTypeService(store, resolver, logger)
	ctx := context.Background()

	t.Run("success - resolved", func(t *testing.T) {
		req := &routingv1.ResolveEquipmentTypeRequest{
			Labels: map[string]string{
				"equipment_type": "router",
			},
		}

		resp, err := svc.ResolveEquipmentType(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Found)
		assert.NotNil(t, resp.EquipmentType)
		assert.Equal(t, "router", resp.EquipmentType.Name)
		assert.Equal(t, string(equipment.ResolutionMethodDirectLabel), resp.ResolutionMethod)
		assert.Equal(t, "router", resp.MatchedValue)
	})

	t.Run("success - not resolved (empty labels)", func(t *testing.T) {
		req := &routingv1.ResolveEquipmentTypeRequest{
			Labels: nil,
		}

		resp, err := svc.ResolveEquipmentType(ctx, req)
		require.NoError(t, err)
		assert.False(t, resp.Found)
		assert.Equal(t, string(equipment.ResolutionMethodNotResolved), resp.ResolutionMethod)
	})

	t.Run("success - not resolved (no matching type)", func(t *testing.T) {
		req := &routingv1.ResolveEquipmentTypeRequest{
			Labels: map[string]string{
				"equipment_type": "unknown_device",
			},
		}

		resp, err := svc.ResolveEquipmentType(ctx, req)
		require.NoError(t, err)
		assert.False(t, resp.Found)
	})
}
