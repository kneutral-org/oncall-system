// Package equipment provides equipment type management and resolution for alerts.
package equipment

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
	// ErrEquipmentTypeNotFound is returned when an equipment type cannot be found.
	ErrEquipmentTypeNotFound = errors.New("equipment type not found")
	// ErrInvalidEquipmentType is returned when an equipment type is invalid.
	ErrInvalidEquipmentType = errors.New("invalid equipment type")
	// ErrDuplicateName is returned when an equipment type name already exists.
	ErrDuplicateName = errors.New("duplicate equipment type name")
)

// Store defines the interface for equipment type persistence operations.
type Store interface {
	// Create creates a new equipment type.
	Create(ctx context.Context, eq *EquipmentType) (*EquipmentType, error)

	// GetByID retrieves an equipment type by its ID.
	GetByID(ctx context.Context, id string) (*EquipmentType, error)

	// GetByName retrieves an equipment type by its name.
	GetByName(ctx context.Context, name string) (*EquipmentType, error)

	// List retrieves equipment types with optional filters.
	List(ctx context.Context, filter *ListEquipmentTypesFilter) ([]*EquipmentType, string, error)

	// Update updates an existing equipment type.
	Update(ctx context.Context, eq *EquipmentType) (*EquipmentType, error)

	// Delete deletes an equipment type by ID.
	Delete(ctx context.Context, id string) error
}

// PostgresStore implements Store using PostgreSQL.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore creates a new PostgresStore.
func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

// Create creates a new equipment type.
func (s *PostgresStore) Create(ctx context.Context, eq *EquipmentType) (*EquipmentType, error) {
	if eq == nil || eq.Name == "" {
		return nil, ErrInvalidEquipmentType
	}

	if eq.ID == "" {
		eq.ID = uuid.New().String()
	}

	now := time.Now()
	eq.CreatedAt = now
	eq.UpdatedAt = now

	// Validate criticality
	if eq.Criticality < 1 || eq.Criticality > 5 {
		eq.Criticality = 3 // Default to medium criticality
	}

	metadataJSON, _ := json.Marshal(eq.Metadata)
	routingRulesJSON, _ := json.Marshal(eq.RoutingRules)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO equipment_types (id, name, category, vendor, criticality, default_team_id,
									 escalation_policy, routing_rules, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, eq.ID, eq.Name, eq.Category, nullableString(eq.Vendor), eq.Criticality,
		nullableString(eq.DefaultTeamID), nullableString(eq.EscalationPolicy),
		routingRulesJSON, metadataJSON, eq.CreatedAt, eq.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicateName
		}
		return nil, fmt.Errorf("insert equipment type: %w", err)
	}

	return eq, nil
}

// GetByID retrieves an equipment type by its ID.
func (s *PostgresStore) GetByID(ctx context.Context, id string) (*EquipmentType, error) {
	eq := &EquipmentType{}
	var vendor, defaultTeamID, escalationPolicy sql.NullString
	var metadataJSON, routingRulesJSON []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, category, vendor, criticality, default_team_id,
			   escalation_policy, routing_rules, metadata, created_at, updated_at
		FROM equipment_types WHERE id = $1
	`, id).Scan(
		&eq.ID, &eq.Name, &eq.Category, &vendor, &eq.Criticality, &defaultTeamID,
		&escalationPolicy, &routingRulesJSON, &metadataJSON, &eq.CreatedAt, &eq.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEquipmentTypeNotFound
		}
		return nil, fmt.Errorf("query equipment type by id: %w", err)
	}

	// Handle nullable fields
	if vendor.Valid {
		eq.Vendor = vendor.String
	}
	if defaultTeamID.Valid {
		eq.DefaultTeamID = defaultTeamID.String
	}
	if escalationPolicy.Valid {
		eq.EscalationPolicy = escalationPolicy.String
	}

	// Parse JSONB fields
	if metadataJSON != nil {
		if err := json.Unmarshal(metadataJSON, &eq.Metadata); err != nil {
			eq.Metadata = make(map[string]string)
		}
	} else {
		eq.Metadata = make(map[string]string)
	}

	if routingRulesJSON != nil {
		if err := json.Unmarshal(routingRulesJSON, &eq.RoutingRules); err != nil {
			eq.RoutingRules = []string{}
		}
	}

	return eq, nil
}

// GetByName retrieves an equipment type by its name.
func (s *PostgresStore) GetByName(ctx context.Context, name string) (*EquipmentType, error) {
	eq := &EquipmentType{}
	var vendor, defaultTeamID, escalationPolicy sql.NullString
	var metadataJSON, routingRulesJSON []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, category, vendor, criticality, default_team_id,
			   escalation_policy, routing_rules, metadata, created_at, updated_at
		FROM equipment_types WHERE LOWER(name) = LOWER($1)
	`, name).Scan(
		&eq.ID, &eq.Name, &eq.Category, &vendor, &eq.Criticality, &defaultTeamID,
		&escalationPolicy, &routingRulesJSON, &metadataJSON, &eq.CreatedAt, &eq.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEquipmentTypeNotFound
		}
		return nil, fmt.Errorf("query equipment type by name: %w", err)
	}

	// Handle nullable fields
	if vendor.Valid {
		eq.Vendor = vendor.String
	}
	if defaultTeamID.Valid {
		eq.DefaultTeamID = defaultTeamID.String
	}
	if escalationPolicy.Valid {
		eq.EscalationPolicy = escalationPolicy.String
	}

	// Parse JSONB fields
	if metadataJSON != nil {
		if err := json.Unmarshal(metadataJSON, &eq.Metadata); err != nil {
			eq.Metadata = make(map[string]string)
		}
	} else {
		eq.Metadata = make(map[string]string)
	}

	if routingRulesJSON != nil {
		if err := json.Unmarshal(routingRulesJSON, &eq.RoutingRules); err != nil {
			eq.RoutingRules = []string{}
		}
	}

	return eq, nil
}

// List retrieves equipment types with optional filters.
func (s *PostgresStore) List(ctx context.Context, filter *ListEquipmentTypesFilter) ([]*EquipmentType, string, error) {
	query := `SELECT id, name, category, vendor, criticality, default_team_id,
			   escalation_policy, routing_rules, metadata, created_at, updated_at
			   FROM equipment_types WHERE 1=1`
	args := []interface{}{}
	argIndex := 1

	if filter != nil {
		if filter.Category != "" {
			query += fmt.Sprintf(" AND category = $%d", argIndex)
			args = append(args, filter.Category)
			argIndex++
		}

		if filter.Vendor != "" {
			query += fmt.Sprintf(" AND vendor = $%d", argIndex)
			args = append(args, filter.Vendor)
			argIndex++
		}

		if filter.Criticality > 0 {
			query += fmt.Sprintf(" AND criticality = $%d", argIndex)
			args = append(args, filter.Criticality)
			argIndex++
		}
	}

	query += " ORDER BY criticality DESC, name ASC"

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
		return nil, "", fmt.Errorf("query equipment types: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var equipmentTypes []*EquipmentType
	for rows.Next() {
		eq := &EquipmentType{}
		var vendor, defaultTeamID, escalationPolicy sql.NullString
		var metadataJSON, routingRulesJSON []byte

		if err := rows.Scan(
			&eq.ID, &eq.Name, &eq.Category, &vendor, &eq.Criticality, &defaultTeamID,
			&escalationPolicy, &routingRulesJSON, &metadataJSON, &eq.CreatedAt, &eq.UpdatedAt,
		); err != nil {
			return nil, "", fmt.Errorf("scan equipment type: %w", err)
		}

		// Handle nullable fields
		if vendor.Valid {
			eq.Vendor = vendor.String
		}
		if defaultTeamID.Valid {
			eq.DefaultTeamID = defaultTeamID.String
		}
		if escalationPolicy.Valid {
			eq.EscalationPolicy = escalationPolicy.String
		}

		// Parse JSONB fields
		if metadataJSON != nil {
			if err := json.Unmarshal(metadataJSON, &eq.Metadata); err != nil {
				eq.Metadata = make(map[string]string)
			}
		} else {
			eq.Metadata = make(map[string]string)
		}

		if routingRulesJSON != nil {
			if err := json.Unmarshal(routingRulesJSON, &eq.RoutingRules); err != nil {
				eq.RoutingRules = []string{}
			}
		}

		equipmentTypes = append(equipmentTypes, eq)
	}

	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	// Pagination token
	var nextPageToken string
	if len(equipmentTypes) > pageSize {
		equipmentTypes = equipmentTypes[:pageSize]
		offset := 0
		if filter != nil && filter.PageToken != "" {
			_, _ = fmt.Sscanf(filter.PageToken, "%d", &offset)
		}
		nextPageToken = fmt.Sprintf("%d", offset+pageSize)
	}

	return equipmentTypes, nextPageToken, nil
}

// Update updates an existing equipment type.
func (s *PostgresStore) Update(ctx context.Context, eq *EquipmentType) (*EquipmentType, error) {
	if eq == nil || eq.ID == "" {
		return nil, ErrInvalidEquipmentType
	}

	eq.UpdatedAt = time.Now()

	// Validate criticality
	if eq.Criticality < 1 || eq.Criticality > 5 {
		eq.Criticality = 3 // Default to medium criticality
	}

	metadataJSON, _ := json.Marshal(eq.Metadata)
	routingRulesJSON, _ := json.Marshal(eq.RoutingRules)

	result, err := s.db.ExecContext(ctx, `
		UPDATE equipment_types SET name = $1, category = $2, vendor = $3, criticality = $4,
								   default_team_id = $5, escalation_policy = $6,
								   routing_rules = $7, metadata = $8, updated_at = $9
		WHERE id = $10
	`, eq.Name, eq.Category, nullableString(eq.Vendor), eq.Criticality,
		nullableString(eq.DefaultTeamID), nullableString(eq.EscalationPolicy),
		routingRulesJSON, metadataJSON, eq.UpdatedAt, eq.ID)
	if err != nil {
		return nil, fmt.Errorf("update equipment type: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, ErrEquipmentTypeNotFound
	}

	return eq, nil
}

// Delete deletes an equipment type by ID.
func (s *PostgresStore) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM equipment_types WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete equipment type: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrEquipmentTypeNotFound
	}

	return nil
}

// Helper function to create sql.NullString from string
func nullableString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// Helper function to check for unique constraint violation
func isUniqueViolation(err error) bool {
	// PostgreSQL unique violation error code is 23505
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
