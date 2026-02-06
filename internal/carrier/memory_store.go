// Package carrier provides carrier management and BGP alert handling for the on-call system.
package carrier

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InMemoryStore is an in-memory implementation of Store for testing.
type InMemoryStore struct {
	mu       sync.RWMutex
	carriers map[string]*Carrier
}

// NewInMemoryStore creates a new in-memory store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		carriers: make(map[string]*Carrier),
	}
}

// Create creates a new carrier in memory.
func (s *InMemoryStore) Create(ctx context.Context, carrier *Carrier) (*Carrier, error) {
	if carrier == nil {
		return nil, ErrInvalidCarrier
	}

	if carrier.Name == "" {
		return nil, ErrInvalidCarrier
	}

	if carrier.ASN <= 0 {
		return nil, ErrInvalidCarrier
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate ASN
	for _, c := range s.carriers {
		if c.ASN == carrier.ASN {
			return nil, ErrDuplicateASN
		}
		if strings.EqualFold(c.Name, carrier.Name) {
			return nil, ErrDuplicateName
		}
	}

	// Generate ID if not provided
	if carrier.ID == "" {
		carrier.ID = uuid.New().String()
	}

	now := time.Now()
	carrier.CreatedAt = now
	carrier.UpdatedAt = now

	// Deep copy to avoid external modifications
	copy := *carrier
	copy.Contacts = make([]Contact, len(carrier.Contacts))
	for i, c := range carrier.Contacts {
		copy.Contacts[i] = c
	}
	if carrier.Metadata != nil {
		copy.Metadata = make(map[string]string)
		for k, v := range carrier.Metadata {
			copy.Metadata[k] = v
		}
	}

	s.carriers[carrier.ID] = &copy

	return carrier, nil
}

// GetByID retrieves a carrier by its ID.
func (s *InMemoryStore) GetByID(ctx context.Context, id string) (*Carrier, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	carrier, ok := s.carriers[id]
	if !ok {
		return nil, ErrNotFound
	}

	return carrier, nil
}

// GetByASN retrieves a carrier by its Autonomous System Number.
func (s *InMemoryStore) GetByASN(ctx context.Context, asn int) (*Carrier, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, carrier := range s.carriers {
		if carrier.ASN == asn {
			return carrier, nil
		}
	}

	return nil, ErrNotFound
}

// GetByName retrieves a carrier by its name (case-insensitive).
func (s *InMemoryStore) GetByName(ctx context.Context, name string) (*Carrier, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, carrier := range s.carriers {
		if strings.EqualFold(carrier.Name, name) {
			return carrier, nil
		}
	}

	return nil, ErrNotFound
}

// List retrieves carriers based on filter criteria.
func (s *InMemoryStore) List(ctx context.Context, filter *CarrierFilter) ([]*Carrier, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var carriers []*Carrier

	for _, carrier := range s.carriers {
		// Apply filters
		if filter != nil {
			if filter.Type != "" && carrier.Type != filter.Type {
				continue
			}

			if filter.NameContains != "" && !strings.Contains(
				strings.ToLower(carrier.Name),
				strings.ToLower(filter.NameContains),
			) {
				continue
			}
		}

		carriers = append(carriers, carrier)
	}

	// Sort by priority, then by name
	for i := 0; i < len(carriers)-1; i++ {
		for j := i + 1; j < len(carriers); j++ {
			if carriers[i].Priority > carriers[j].Priority ||
				(carriers[i].Priority == carriers[j].Priority && carriers[i].Name > carriers[j].Name) {
				carriers[i], carriers[j] = carriers[j], carriers[i]
			}
		}
	}

	// Apply pagination
	pageSize := 50
	if filter != nil && filter.PageSize > 0 && filter.PageSize <= 100 {
		pageSize = filter.PageSize
	}

	if len(carriers) > pageSize {
		carriers = carriers[:pageSize]
	}

	return carriers, nil
}

// Update updates an existing carrier.
func (s *InMemoryStore) Update(ctx context.Context, carrier *Carrier) (*Carrier, error) {
	if carrier == nil || carrier.ID == "" {
		return nil, ErrInvalidCarrier
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.carriers[carrier.ID]
	if !ok {
		return nil, ErrNotFound
	}

	// Check for duplicate ASN (excluding this carrier)
	for _, c := range s.carriers {
		if c.ID != carrier.ID {
			if c.ASN == carrier.ASN {
				return nil, ErrDuplicateASN
			}
			if strings.EqualFold(c.Name, carrier.Name) {
				return nil, ErrDuplicateName
			}
		}
	}

	carrier.CreatedAt = existing.CreatedAt
	carrier.UpdatedAt = time.Now()

	// Deep copy
	copy := *carrier
	copy.Contacts = make([]Contact, len(carrier.Contacts))
	for i, c := range carrier.Contacts {
		copy.Contacts[i] = c
	}
	if carrier.Metadata != nil {
		copy.Metadata = make(map[string]string)
		for k, v := range carrier.Metadata {
			copy.Metadata[k] = v
		}
	}

	s.carriers[carrier.ID] = &copy

	return carrier, nil
}

// Delete deletes a carrier by ID.
func (s *InMemoryStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.carriers[id]; !ok {
		return ErrNotFound
	}

	delete(s.carriers, id)
	return nil
}

// Ensure InMemoryStore implements Store
var _ Store = (*InMemoryStore)(nil)
