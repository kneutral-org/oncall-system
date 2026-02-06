// Package customer provides customer and tier management for alert routing.
package customer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrTierNotFound is returned when a customer tier cannot be found.
	ErrTierNotFound = errors.New("customer tier not found")
	// ErrInvalidTier is returned when a tier is invalid.
	ErrInvalidTier = errors.New("invalid customer tier")
	// ErrDuplicateTierName is returned when a tier name already exists.
	ErrDuplicateTierName = errors.New("duplicate tier name")
	// ErrDuplicateTierLevel is returned when a tier level already exists.
	ErrDuplicateTierLevel = errors.New("duplicate tier level")
)

// TierStore defines the interface for customer tier persistence.
type TierStore interface {
	// Create creates a new customer tier.
	Create(ctx context.Context, tier *CustomerTier) (*CustomerTier, error)

	// GetByID retrieves a customer tier by ID.
	GetByID(ctx context.Context, id string) (*CustomerTier, error)

	// GetByName retrieves a customer tier by name.
	GetByName(ctx context.Context, name string) (*CustomerTier, error)

	// GetByLevel retrieves a customer tier by level.
	GetByLevel(ctx context.Context, level int) (*CustomerTier, error)

	// List retrieves customer tiers with optional filters.
	List(ctx context.Context, filter *ListCustomerTiersFilter) ([]*CustomerTier, string, error)

	// Update updates an existing customer tier.
	Update(ctx context.Context, tier *CustomerTier) (*CustomerTier, error)

	// Delete deletes a customer tier by ID.
	Delete(ctx context.Context, id string) error
}

// PostgresTierStore implements TierStore using PostgreSQL.
type PostgresTierStore struct {
	db *sql.DB
}

// NewPostgresTierStore creates a new PostgresTierStore.
func NewPostgresTierStore(db *sql.DB) *PostgresTierStore {
	return &PostgresTierStore{db: db}
}

// Create creates a new customer tier in the database.
func (s *PostgresTierStore) Create(ctx context.Context, tier *CustomerTier) (*CustomerTier, error) {
	if tier == nil || tier.Name == "" {
		return nil, ErrInvalidTier
	}

	if tier.ID == "" {
		tier.ID = uuid.New().String()
	}

	now := time.Now()
	tier.CreatedAt = now
	tier.UpdatedAt = now

	// Default escalation multiplier
	if tier.EscalationMultiplier == 0 {
		tier.EscalationMultiplier = 1.0
	}

	metadataJSON, _ := json.Marshal(tier.Metadata)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO customer_tiers (
			id, name, level, description,
			critical_response_ms, high_response_ms, medium_response_ms, low_response_ms,
			escalation_multiplier, severity_boost, dedicated_team_id,
			metadata, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`,
		tier.ID, tier.Name, tier.Level, tier.Description,
		tier.CriticalResponseTime.Milliseconds(),
		tier.HighResponseTime.Milliseconds(),
		tier.MediumResponseTime.Milliseconds(),
		tier.LowResponseTime.Milliseconds(),
		tier.EscalationMultiplier, tier.SeverityBoost, tier.DedicatedTeamID,
		metadataJSON, tier.CreatedAt, tier.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			if contains(err.Error(), "name") {
				return nil, ErrDuplicateTierName
			}
			if contains(err.Error(), "level") {
				return nil, ErrDuplicateTierLevel
			}
			return nil, ErrDuplicateTierName
		}
		return nil, fmt.Errorf("insert customer tier: %w", err)
	}

	return tier, nil
}

// GetByID retrieves a customer tier by ID.
func (s *PostgresTierStore) GetByID(ctx context.Context, id string) (*CustomerTier, error) {
	return s.getByField(ctx, "id", id)
}

// GetByName retrieves a customer tier by name.
func (s *PostgresTierStore) GetByName(ctx context.Context, name string) (*CustomerTier, error) {
	return s.getByField(ctx, "name", name)
}

// GetByLevel retrieves a customer tier by level.
func (s *PostgresTierStore) GetByLevel(ctx context.Context, level int) (*CustomerTier, error) {
	tier := &CustomerTier{}
	var description sql.NullString
	var dedicatedTeamID sql.NullString
	var metadataJSON []byte
	var criticalMs, highMs, mediumMs, lowMs int64

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, level, description,
			   critical_response_ms, high_response_ms, medium_response_ms, low_response_ms,
			   escalation_multiplier, severity_boost, dedicated_team_id,
			   metadata, created_at, updated_at
		FROM customer_tiers WHERE level = $1
	`, level).Scan(
		&tier.ID, &tier.Name, &tier.Level, &description,
		&criticalMs, &highMs, &mediumMs, &lowMs,
		&tier.EscalationMultiplier, &tier.SeverityBoost, &dedicatedTeamID,
		&metadataJSON, &tier.CreatedAt, &tier.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTierNotFound
		}
		return nil, fmt.Errorf("query customer tier by level: %w", err)
	}

	tier.Description = description.String
	if dedicatedTeamID.Valid {
		tier.DedicatedTeamID = &dedicatedTeamID.String
	}
	tier.CriticalResponseTime = time.Duration(criticalMs) * time.Millisecond
	tier.HighResponseTime = time.Duration(highMs) * time.Millisecond
	tier.MediumResponseTime = time.Duration(mediumMs) * time.Millisecond
	tier.LowResponseTime = time.Duration(lowMs) * time.Millisecond

	if metadataJSON != nil {
		_ = json.Unmarshal(metadataJSON, &tier.Metadata)
	}
	if tier.Metadata == nil {
		tier.Metadata = make(map[string]string)
	}

	return tier, nil
}

// getByField retrieves a customer tier by a specific field.
func (s *PostgresTierStore) getByField(ctx context.Context, field, value string) (*CustomerTier, error) {
	tier := &CustomerTier{}
	var description sql.NullString
	var dedicatedTeamID sql.NullString
	var metadataJSON []byte
	var criticalMs, highMs, mediumMs, lowMs int64

	query := fmt.Sprintf(`
		SELECT id, name, level, description,
			   critical_response_ms, high_response_ms, medium_response_ms, low_response_ms,
			   escalation_multiplier, severity_boost, dedicated_team_id,
			   metadata, created_at, updated_at
		FROM customer_tiers WHERE %s = $1
	`, field)

	err := s.db.QueryRowContext(ctx, query, value).Scan(
		&tier.ID, &tier.Name, &tier.Level, &description,
		&criticalMs, &highMs, &mediumMs, &lowMs,
		&tier.EscalationMultiplier, &tier.SeverityBoost, &dedicatedTeamID,
		&metadataJSON, &tier.CreatedAt, &tier.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTierNotFound
		}
		return nil, fmt.Errorf("query customer tier by %s: %w", field, err)
	}

	tier.Description = description.String
	if dedicatedTeamID.Valid {
		tier.DedicatedTeamID = &dedicatedTeamID.String
	}
	tier.CriticalResponseTime = time.Duration(criticalMs) * time.Millisecond
	tier.HighResponseTime = time.Duration(highMs) * time.Millisecond
	tier.MediumResponseTime = time.Duration(mediumMs) * time.Millisecond
	tier.LowResponseTime = time.Duration(lowMs) * time.Millisecond

	if metadataJSON != nil {
		_ = json.Unmarshal(metadataJSON, &tier.Metadata)
	}
	if tier.Metadata == nil {
		tier.Metadata = make(map[string]string)
	}

	return tier, nil
}

// List retrieves customer tiers with optional filters.
func (s *PostgresTierStore) List(ctx context.Context, filter *ListCustomerTiersFilter) ([]*CustomerTier, string, error) {
	query := `
		SELECT id, name, level, description,
			   critical_response_ms, high_response_ms, medium_response_ms, low_response_ms,
			   escalation_multiplier, severity_boost, dedicated_team_id,
			   metadata, created_at, updated_at
		FROM customer_tiers
		ORDER BY level ASC`

	args := []interface{}{}
	argIndex := 1

	// Pagination
	pageSize := 50
	if filter != nil && filter.PageSize > 0 && filter.PageSize <= 100 {
		pageSize = filter.PageSize
	}
	query += fmt.Sprintf(" LIMIT $%d", argIndex)
	args = append(args, pageSize+1)
	argIndex++

	if filter != nil && filter.PageToken != "" {
		var offset int
		_, _ = fmt.Sscanf(filter.PageToken, "%d", &offset)
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("query customer tiers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tiers []*CustomerTier
	for rows.Next() {
		tier := &CustomerTier{}
		var description sql.NullString
		var dedicatedTeamID sql.NullString
		var metadataJSON []byte
		var criticalMs, highMs, mediumMs, lowMs int64

		if err := rows.Scan(
			&tier.ID, &tier.Name, &tier.Level, &description,
			&criticalMs, &highMs, &mediumMs, &lowMs,
			&tier.EscalationMultiplier, &tier.SeverityBoost, &dedicatedTeamID,
			&metadataJSON, &tier.CreatedAt, &tier.UpdatedAt,
		); err != nil {
			return nil, "", fmt.Errorf("scan customer tier: %w", err)
		}

		tier.Description = description.String
		if dedicatedTeamID.Valid {
			tier.DedicatedTeamID = &dedicatedTeamID.String
		}
		tier.CriticalResponseTime = time.Duration(criticalMs) * time.Millisecond
		tier.HighResponseTime = time.Duration(highMs) * time.Millisecond
		tier.MediumResponseTime = time.Duration(mediumMs) * time.Millisecond
		tier.LowResponseTime = time.Duration(lowMs) * time.Millisecond

		if metadataJSON != nil {
			_ = json.Unmarshal(metadataJSON, &tier.Metadata)
		}
		if tier.Metadata == nil {
			tier.Metadata = make(map[string]string)
		}

		tiers = append(tiers, tier)
	}

	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	// Pagination token
	var nextPageToken string
	if len(tiers) > pageSize {
		tiers = tiers[:pageSize]
		offset := 0
		if filter != nil && filter.PageToken != "" {
			_, _ = fmt.Sscanf(filter.PageToken, "%d", &offset)
		}
		nextPageToken = fmt.Sprintf("%d", offset+pageSize)
	}

	return tiers, nextPageToken, nil
}

// Update updates an existing customer tier.
func (s *PostgresTierStore) Update(ctx context.Context, tier *CustomerTier) (*CustomerTier, error) {
	if tier == nil || tier.ID == "" {
		return nil, ErrInvalidTier
	}

	tier.UpdatedAt = time.Now()

	metadataJSON, _ := json.Marshal(tier.Metadata)

	result, err := s.db.ExecContext(ctx, `
		UPDATE customer_tiers SET
			name = $1, level = $2, description = $3,
			critical_response_ms = $4, high_response_ms = $5, medium_response_ms = $6, low_response_ms = $7,
			escalation_multiplier = $8, severity_boost = $9, dedicated_team_id = $10,
			metadata = $11, updated_at = $12
		WHERE id = $13
	`,
		tier.Name, tier.Level, tier.Description,
		tier.CriticalResponseTime.Milliseconds(),
		tier.HighResponseTime.Milliseconds(),
		tier.MediumResponseTime.Milliseconds(),
		tier.LowResponseTime.Milliseconds(),
		tier.EscalationMultiplier, tier.SeverityBoost, tier.DedicatedTeamID,
		metadataJSON, tier.UpdatedAt, tier.ID,
	)
	if err != nil {
		if isUniqueViolation(err) {
			if contains(err.Error(), "name") {
				return nil, ErrDuplicateTierName
			}
			if contains(err.Error(), "level") {
				return nil, ErrDuplicateTierLevel
			}
		}
		return nil, fmt.Errorf("update customer tier: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, ErrTierNotFound
	}

	return tier, nil
}

// Delete deletes a customer tier by ID.
func (s *PostgresTierStore) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM customer_tiers WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete customer tier: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrTierNotFound
	}

	return nil
}

// InMemoryTierStore is an in-memory implementation of TierStore for testing.
type InMemoryTierStore struct {
	tiers   map[string]*CustomerTier
	counter int64
}

// NewInMemoryTierStore creates a new in-memory tier store.
func NewInMemoryTierStore() *InMemoryTierStore {
	return &InMemoryTierStore{
		tiers: make(map[string]*CustomerTier),
	}
}

// Create creates a new customer tier in memory.
func (s *InMemoryTierStore) Create(ctx context.Context, tier *CustomerTier) (*CustomerTier, error) {
	if tier == nil || tier.Name == "" {
		return nil, ErrInvalidTier
	}

	// Check for duplicate name or level
	for _, t := range s.tiers {
		if t.Name == tier.Name {
			return nil, ErrDuplicateTierName
		}
		if t.Level == tier.Level {
			return nil, ErrDuplicateTierLevel
		}
	}

	if tier.ID == "" {
		s.counter++
		tier.ID = fmt.Sprintf("tier-%d", s.counter)
	}

	now := time.Now()
	tier.CreatedAt = now
	tier.UpdatedAt = now

	if tier.EscalationMultiplier == 0 {
		tier.EscalationMultiplier = 1.0
	}

	// Deep copy
	stored := *tier
	if tier.Metadata != nil {
		stored.Metadata = make(map[string]string)
		for k, v := range tier.Metadata {
			stored.Metadata[k] = v
		}
	}
	s.tiers[tier.ID] = &stored

	return tier, nil
}

// GetByID retrieves a customer tier by ID.
func (s *InMemoryTierStore) GetByID(ctx context.Context, id string) (*CustomerTier, error) {
	tier, ok := s.tiers[id]
	if !ok {
		return nil, ErrTierNotFound
	}
	return tier, nil
}

// GetByName retrieves a customer tier by name.
func (s *InMemoryTierStore) GetByName(ctx context.Context, name string) (*CustomerTier, error) {
	for _, tier := range s.tiers {
		if tier.Name == name {
			return tier, nil
		}
	}
	return nil, ErrTierNotFound
}

// GetByLevel retrieves a customer tier by level.
func (s *InMemoryTierStore) GetByLevel(ctx context.Context, level int) (*CustomerTier, error) {
	for _, tier := range s.tiers {
		if tier.Level == level {
			return tier, nil
		}
	}
	return nil, ErrTierNotFound
}

// List retrieves customer tiers with optional filters.
func (s *InMemoryTierStore) List(ctx context.Context, filter *ListCustomerTiersFilter) ([]*CustomerTier, string, error) {
	var tiers []*CustomerTier
	for _, tier := range s.tiers {
		tiers = append(tiers, tier)
	}

	// Sort by level
	for i := 0; i < len(tiers)-1; i++ {
		for j := i + 1; j < len(tiers); j++ {
			if tiers[i].Level > tiers[j].Level {
				tiers[i], tiers[j] = tiers[j], tiers[i]
			}
		}
	}

	return tiers, "", nil
}

// Update updates an existing customer tier.
func (s *InMemoryTierStore) Update(ctx context.Context, tier *CustomerTier) (*CustomerTier, error) {
	if tier == nil || tier.ID == "" {
		return nil, ErrInvalidTier
	}

	existing, ok := s.tiers[tier.ID]
	if !ok {
		return nil, ErrTierNotFound
	}

	// Check for duplicate name or level (excluding this tier)
	for _, t := range s.tiers {
		if t.ID != tier.ID {
			if t.Name == tier.Name {
				return nil, ErrDuplicateTierName
			}
			if t.Level == tier.Level {
				return nil, ErrDuplicateTierLevel
			}
		}
	}

	tier.CreatedAt = existing.CreatedAt
	tier.UpdatedAt = time.Now()

	// Deep copy
	stored := *tier
	if tier.Metadata != nil {
		stored.Metadata = make(map[string]string)
		for k, v := range tier.Metadata {
			stored.Metadata[k] = v
		}
	}
	s.tiers[tier.ID] = &stored

	return tier, nil
}

// Delete deletes a customer tier by ID.
func (s *InMemoryTierStore) Delete(ctx context.Context, id string) error {
	if _, ok := s.tiers[id]; !ok {
		return ErrTierNotFound
	}
	delete(s.tiers, id)
	return nil
}

// Helper functions
func isUniqueViolation(err error) bool {
	return err != nil && (contains(err.Error(), "23505") || contains(err.Error(), "unique constraint"))
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Ensure interfaces are implemented
var _ TierStore = (*PostgresTierStore)(nil)
var _ TierStore = (*InMemoryTierStore)(nil)
