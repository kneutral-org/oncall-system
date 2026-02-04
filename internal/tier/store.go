// Package tier provides the customer tier store implementation.
package tier

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CustomerTier represents a customer tier domain model.
type CustomerTier struct {
	ID                     uuid.UUID              `json:"id"`
	Name                   string                 `json:"name"`
	Level                  int                    `json:"level"`
	CriticalResponseMinutes int                   `json:"criticalResponseMinutes"`
	HighResponseMinutes    int                    `json:"highResponseMinutes"`
	MediumResponseMinutes  int                    `json:"mediumResponseMinutes"`
	EscalationMultiplier   float64                `json:"escalationMultiplier"`
	DedicatedTeamID        *uuid.UUID             `json:"dedicatedTeamId,omitempty"`
	Metadata               map[string]interface{} `json:"metadata"`
	CreatedAt              time.Time              `json:"createdAt"`
}

// CustomerTierStore defines the interface for customer tier persistence.
type CustomerTierStore interface {
	// CreateTier creates a new customer tier.
	CreateTier(ctx context.Context, tier *CustomerTier) (*CustomerTier, error)

	// GetTier retrieves a customer tier by ID.
	GetTier(ctx context.Context, id uuid.UUID) (*CustomerTier, error)

	// GetTierByLevel retrieves a customer tier by its level.
	GetTierByLevel(ctx context.Context, level int) (*CustomerTier, error)

	// ListTiers retrieves all customer tiers ordered by level.
	ListTiers(ctx context.Context) ([]*CustomerTier, error)

	// UpdateTier updates an existing customer tier.
	UpdateTier(ctx context.Context, tier *CustomerTier) (*CustomerTier, error)

	// DeleteTier deletes a customer tier.
	DeleteTier(ctx context.Context, id uuid.UUID) error

	// ResolveTier resolves the appropriate tier for a customer based on ID or labels.
	ResolveTier(ctx context.Context, customerID string, labels map[string]string) (*CustomerTier, error)
}

// InMemoryCustomerTierStore is an in-memory implementation of CustomerTierStore.
type InMemoryCustomerTierStore struct {
	mu         sync.RWMutex
	tiers      map[uuid.UUID]*CustomerTier
	levelIndex map[int]uuid.UUID
	// customerTierMapping stores customer_id -> tier_level mapping
	customerTierMapping map[string]int
}

// NewInMemoryCustomerTierStore creates a new in-memory customer tier store.
func NewInMemoryCustomerTierStore() *InMemoryCustomerTierStore {
	store := &InMemoryCustomerTierStore{
		tiers:               make(map[uuid.UUID]*CustomerTier),
		levelIndex:          make(map[int]uuid.UUID),
		customerTierMapping: make(map[string]int),
	}
	// Initialize with default tiers
	store.initializeDefaultTiers()
	return store
}

// initializeDefaultTiers creates default tiers.
func (s *InMemoryCustomerTierStore) initializeDefaultTiers() {
	defaultTiers := []*CustomerTier{
		{
			ID:                     uuid.New(),
			Name:                   "Platinum",
			Level:                  1,
			CriticalResponseMinutes: 5,
			HighResponseMinutes:    15,
			MediumResponseMinutes:  30,
			EscalationMultiplier:   0.5,
			Metadata:               map[string]interface{}{"description": "Highest priority customers with 24/7 dedicated support"},
			CreatedAt:              time.Now(),
		},
		{
			ID:                     uuid.New(),
			Name:                   "Gold",
			Level:                  2,
			CriticalResponseMinutes: 15,
			HighResponseMinutes:    30,
			MediumResponseMinutes:  60,
			EscalationMultiplier:   0.75,
			Metadata:               map[string]interface{}{"description": "Premium customers with priority support"},
			CreatedAt:              time.Now(),
		},
		{
			ID:                     uuid.New(),
			Name:                   "Silver",
			Level:                  3,
			CriticalResponseMinutes: 30,
			HighResponseMinutes:    60,
			MediumResponseMinutes:  120,
			EscalationMultiplier:   1.0,
			Metadata:               map[string]interface{}{"description": "Standard customers with business hours support"},
			CreatedAt:              time.Now(),
		},
		{
			ID:                     uuid.New(),
			Name:                   "Bronze",
			Level:                  4,
			CriticalResponseMinutes: 60,
			HighResponseMinutes:    120,
			MediumResponseMinutes:  240,
			EscalationMultiplier:   1.5,
			Metadata:               map[string]interface{}{"description": "Basic tier customers"},
			CreatedAt:              time.Now(),
		},
	}

	for _, tier := range defaultTiers {
		s.tiers[tier.ID] = tier
		s.levelIndex[tier.Level] = tier.ID
	}
}

// CreateTier creates a new customer tier.
func (s *InMemoryCustomerTierStore) CreateTier(ctx context.Context, tier *CustomerTier) (*CustomerTier, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tier.ID = uuid.New()
	tier.CreatedAt = time.Now()

	if tier.CriticalResponseMinutes == 0 {
		tier.CriticalResponseMinutes = 15
	}
	if tier.HighResponseMinutes == 0 {
		tier.HighResponseMinutes = 30
	}
	if tier.MediumResponseMinutes == 0 {
		tier.MediumResponseMinutes = 60
	}
	if tier.EscalationMultiplier == 0 {
		tier.EscalationMultiplier = 1.0
	}
	if tier.Metadata == nil {
		tier.Metadata = make(map[string]interface{})
	}

	// Deep copy
	stored := deepCopyTier(tier)
	s.tiers[tier.ID] = stored
	s.levelIndex[tier.Level] = tier.ID

	return tier, nil
}

// GetTier retrieves a customer tier by ID.
func (s *InMemoryCustomerTierStore) GetTier(ctx context.Context, id uuid.UUID) (*CustomerTier, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tier, ok := s.tiers[id]
	if !ok {
		return nil, nil
	}

	return deepCopyTier(tier), nil
}

// GetTierByLevel retrieves a customer tier by its level.
func (s *InMemoryCustomerTierStore) GetTierByLevel(ctx context.Context, level int) (*CustomerTier, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.levelIndex[level]
	if !ok {
		return nil, nil
	}

	tier, ok := s.tiers[id]
	if !ok {
		return nil, nil
	}

	return deepCopyTier(tier), nil
}

// ListTiers retrieves all customer tiers ordered by level.
func (s *InMemoryCustomerTierStore) ListTiers(ctx context.Context) ([]*CustomerTier, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*CustomerTier, 0, len(s.tiers))
	for _, tier := range s.tiers {
		result = append(result, deepCopyTier(tier))
	}

	// Sort by level
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].Level > result[j].Level {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result, nil
}

// UpdateTier updates an existing customer tier.
func (s *InMemoryCustomerTierStore) UpdateTier(ctx context.Context, tier *CustomerTier) (*CustomerTier, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.tiers[tier.ID]
	if !ok {
		return nil, nil
	}

	// Remove old level index if level changed
	if existing.Level != tier.Level {
		delete(s.levelIndex, existing.Level)
		s.levelIndex[tier.Level] = tier.ID
	}

	tier.CreatedAt = existing.CreatedAt

	stored := deepCopyTier(tier)
	s.tiers[tier.ID] = stored

	return tier, nil
}

// DeleteTier deletes a customer tier.
func (s *InMemoryCustomerTierStore) DeleteTier(ctx context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tier, ok := s.tiers[id]; ok {
		delete(s.levelIndex, tier.Level)
	}
	delete(s.tiers, id)
	return nil
}

// ResolveTier resolves the appropriate tier for a customer based on ID or labels.
func (s *InMemoryCustomerTierStore) ResolveTier(ctx context.Context, customerID string, labels map[string]string) (*CustomerTier, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// First, try to resolve by customer ID
	if customerID != "" {
		if level, ok := s.customerTierMapping[customerID]; ok {
			if id, ok := s.levelIndex[level]; ok {
				if tier, ok := s.tiers[id]; ok {
					return deepCopyTier(tier), nil
				}
			}
		}
	}

	// Next, try to resolve by tier label
	if labels != nil {
		if tierLabel, ok := labels["tier"]; ok {
			// Try to match tier by name
			for _, tier := range s.tiers {
				if tier.Name == tierLabel {
					return deepCopyTier(tier), nil
				}
			}
		}

		// Try to resolve by customer_tier label (as level)
		if tierLevelStr, ok := labels["customer_tier"]; ok {
			level := 0
			for i := 0; i < len(tierLevelStr); i++ {
				if tierLevelStr[i] >= '0' && tierLevelStr[i] <= '9' {
					level = level*10 + int(tierLevelStr[i]-'0')
				}
			}
			if level > 0 {
				if id, ok := s.levelIndex[level]; ok {
					if tier, ok := s.tiers[id]; ok {
						return deepCopyTier(tier), nil
					}
				}
			}
		}
	}

	// Default to level 3 (Silver) if no match found
	if id, ok := s.levelIndex[3]; ok {
		if tier, ok := s.tiers[id]; ok {
			return deepCopyTier(tier), nil
		}
	}

	// If even default doesn't exist, return the lowest tier level
	var lowestTier *CustomerTier
	for _, tier := range s.tiers {
		if lowestTier == nil || tier.Level < lowestTier.Level {
			lowestTier = tier
		}
	}

	if lowestTier != nil {
		return deepCopyTier(lowestTier), nil
	}

	return nil, nil
}

// SetCustomerTierMapping sets the tier level for a customer ID (for testing/admin purposes).
func (s *InMemoryCustomerTierStore) SetCustomerTierMapping(customerID string, level int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.customerTierMapping[customerID] = level
}

// deepCopyTier creates a deep copy of a CustomerTier.
func deepCopyTier(tier *CustomerTier) *CustomerTier {
	copied := *tier

	if tier.DedicatedTeamID != nil {
		id := *tier.DedicatedTeamID
		copied.DedicatedTeamID = &id
	}

	if tier.Metadata != nil {
		copied.Metadata = make(map[string]interface{})
		data, _ := json.Marshal(tier.Metadata)
		_ = json.Unmarshal(data, &copied.Metadata)
	}

	return &copied
}

// Verify InMemoryCustomerTierStore implements CustomerTierStore interface
var _ CustomerTierStore = (*InMemoryCustomerTierStore)(nil)
