// Package site provides site resolution and enrichment for alerts.
package site

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
	// ErrSiteNotFound is returned when a site cannot be found.
	ErrSiteNotFound = errors.New("site not found")
	// ErrTeamNotFound is returned when a team cannot be found.
	ErrTeamNotFound = errors.New("team not found")
	// ErrInvalidSite is returned when a site is invalid.
	ErrInvalidSite = errors.New("invalid site")
	// ErrDuplicateCode is returned when a site code already exists.
	ErrDuplicateCode = errors.New("duplicate site code")
)

// Store defines the interface for site persistence operations.
type Store interface {
	// GetByCode retrieves a site by its unique code.
	GetByCode(ctx context.Context, code string) (*Site, error)

	// GetByID retrieves a site by its ID.
	GetByID(ctx context.Context, id string) (*Site, error)

	// List retrieves sites with optional filters.
	List(ctx context.Context, filter *ListSitesFilter) ([]*Site, string, error)

	// Create creates a new site.
	Create(ctx context.Context, site *Site) (*Site, error)

	// Update updates an existing site.
	Update(ctx context.Context, site *Site) (*Site, error)

	// Delete deletes a site by ID.
	Delete(ctx context.Context, id string) error

	// GetTeamByID retrieves a team by its ID.
	GetTeamByID(ctx context.Context, id string) (*Team, error)
}

// PostgresStore implements Store using PostgreSQL.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore creates a new PostgresStore.
func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

// GetByCode retrieves a site by its unique code.
func (s *PostgresStore) GetByCode(ctx context.Context, code string) (*Site, error) {
	site := &Site{}
	var tier sql.NullInt32
	var region, country, city, address sql.NullString
	var primaryTeamID, secondaryTeamID, defaultEscPolicyID, parentSiteID sql.NullString
	var labelsJSON, bhJSON []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, code, site_type, tier, region, country, city, address, timezone,
			   primary_team_id, secondary_team_id, default_escalation_policy_id, parent_site_id,
			   labels, business_hours, created_at, updated_at
		FROM sites WHERE code = $1
	`, code).Scan(
		&site.ID, &site.Name, &site.Code, &site.SiteType, &tier,
		&region, &country, &city, &address, &site.Timezone,
		&primaryTeamID, &secondaryTeamID, &defaultEscPolicyID, &parentSiteID,
		&labelsJSON, &bhJSON, &site.CreatedAt, &site.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSiteNotFound
		}
		return nil, fmt.Errorf("query site by code: %w", err)
	}

	// Handle nullable fields
	if tier.Valid {
		t := int(tier.Int32)
		site.Tier = &t
	}
	if region.Valid {
		site.Region = region.String
	}
	if country.Valid {
		site.Country = country.String
	}
	if city.Valid {
		site.City = city.String
	}
	if address.Valid {
		site.Address = address.String
	}
	if primaryTeamID.Valid {
		site.PrimaryTeamID = &primaryTeamID.String
	}
	if secondaryTeamID.Valid {
		site.SecondaryTeamID = &secondaryTeamID.String
	}
	if defaultEscPolicyID.Valid {
		site.DefaultEscalationPolicyID = &defaultEscPolicyID.String
	}
	if parentSiteID.Valid {
		site.ParentSiteID = &parentSiteID.String
	}

	// Parse JSONB fields
	if labelsJSON != nil {
		if err := json.Unmarshal(labelsJSON, &site.Labels); err != nil {
			site.Labels = make(map[string]string)
		}
	} else {
		site.Labels = make(map[string]string)
	}

	if bhJSON != nil {
		var bh BusinessHours
		if err := json.Unmarshal(bhJSON, &bh); err == nil {
			site.BusinessHours = &bh
		}
	}

	return site, nil
}

// GetByID retrieves a site by its ID.
func (s *PostgresStore) GetByID(ctx context.Context, id string) (*Site, error) {
	site := &Site{}
	var tier sql.NullInt32
	var region, country, city, address sql.NullString
	var primaryTeamID, secondaryTeamID, defaultEscPolicyID, parentSiteID sql.NullString
	var labelsJSON, bhJSON []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, code, site_type, tier, region, country, city, address, timezone,
			   primary_team_id, secondary_team_id, default_escalation_policy_id, parent_site_id,
			   labels, business_hours, created_at, updated_at
		FROM sites WHERE id = $1
	`, id).Scan(
		&site.ID, &site.Name, &site.Code, &site.SiteType, &tier,
		&region, &country, &city, &address, &site.Timezone,
		&primaryTeamID, &secondaryTeamID, &defaultEscPolicyID, &parentSiteID,
		&labelsJSON, &bhJSON, &site.CreatedAt, &site.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSiteNotFound
		}
		return nil, fmt.Errorf("query site by id: %w", err)
	}

	// Handle nullable fields (same as GetByCode)
	if tier.Valid {
		t := int(tier.Int32)
		site.Tier = &t
	}
	if region.Valid {
		site.Region = region.String
	}
	if country.Valid {
		site.Country = country.String
	}
	if city.Valid {
		site.City = city.String
	}
	if address.Valid {
		site.Address = address.String
	}
	if primaryTeamID.Valid {
		site.PrimaryTeamID = &primaryTeamID.String
	}
	if secondaryTeamID.Valid {
		site.SecondaryTeamID = &secondaryTeamID.String
	}
	if defaultEscPolicyID.Valid {
		site.DefaultEscalationPolicyID = &defaultEscPolicyID.String
	}
	if parentSiteID.Valid {
		site.ParentSiteID = &parentSiteID.String
	}

	if labelsJSON != nil {
		if err := json.Unmarshal(labelsJSON, &site.Labels); err != nil {
			site.Labels = make(map[string]string)
		}
	} else {
		site.Labels = make(map[string]string)
	}

	if bhJSON != nil {
		var bh BusinessHours
		if err := json.Unmarshal(bhJSON, &bh); err == nil {
			site.BusinessHours = &bh
		}
	}

	return site, nil
}

// List retrieves sites with optional filters.
func (s *PostgresStore) List(ctx context.Context, filter *ListSitesFilter) ([]*Site, string, error) {
	query := `SELECT id, name, code, site_type, tier, region, country, city, address, timezone,
			   primary_team_id, secondary_team_id, default_escalation_policy_id, parent_site_id,
			   labels, business_hours, created_at, updated_at
			   FROM sites WHERE 1=1`
	args := []interface{}{}
	argIndex := 1

	if filter != nil {
		if filter.SiteType != "" {
			query += fmt.Sprintf(" AND site_type = $%d", argIndex)
			args = append(args, filter.SiteType)
			argIndex++
		}

		if filter.Region != "" {
			query += fmt.Sprintf(" AND region = $%d", argIndex)
			args = append(args, filter.Region)
			argIndex++
		}

		if filter.ParentSiteID != "" {
			query += fmt.Sprintf(" AND parent_site_id = $%d", argIndex)
			args = append(args, filter.ParentSiteID)
			argIndex++
		}

		if filter.LabelKey != "" && filter.LabelValue != "" {
			query += fmt.Sprintf(" AND labels ->> $%d = $%d", argIndex, argIndex+1)
			args = append(args, filter.LabelKey, filter.LabelValue)
			argIndex += 2
		}
	}

	query += " ORDER BY name ASC"

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
		return nil, "", fmt.Errorf("query sites: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var sites []*Site
	for rows.Next() {
		site := &Site{}
		var tier sql.NullInt32
		var region, country, city, address sql.NullString
		var primaryTeamID, secondaryTeamID, defaultEscPolicyID, parentSiteID sql.NullString
		var labelsJSON, bhJSON []byte

		if err := rows.Scan(
			&site.ID, &site.Name, &site.Code, &site.SiteType, &tier,
			&region, &country, &city, &address, &site.Timezone,
			&primaryTeamID, &secondaryTeamID, &defaultEscPolicyID, &parentSiteID,
			&labelsJSON, &bhJSON, &site.CreatedAt, &site.UpdatedAt,
		); err != nil {
			return nil, "", fmt.Errorf("scan site: %w", err)
		}

		// Handle nullable fields
		if tier.Valid {
			t := int(tier.Int32)
			site.Tier = &t
		}
		if region.Valid {
			site.Region = region.String
		}
		if country.Valid {
			site.Country = country.String
		}
		if city.Valid {
			site.City = city.String
		}
		if address.Valid {
			site.Address = address.String
		}
		if primaryTeamID.Valid {
			site.PrimaryTeamID = &primaryTeamID.String
		}
		if secondaryTeamID.Valid {
			site.SecondaryTeamID = &secondaryTeamID.String
		}
		if defaultEscPolicyID.Valid {
			site.DefaultEscalationPolicyID = &defaultEscPolicyID.String
		}
		if parentSiteID.Valid {
			site.ParentSiteID = &parentSiteID.String
		}

		if labelsJSON != nil {
			if err := json.Unmarshal(labelsJSON, &site.Labels); err != nil {
				site.Labels = make(map[string]string)
			}
		} else {
			site.Labels = make(map[string]string)
		}

		if bhJSON != nil {
			var bh BusinessHours
			if err := json.Unmarshal(bhJSON, &bh); err == nil {
				site.BusinessHours = &bh
			}
		}

		sites = append(sites, site)
	}

	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	// Pagination token
	var nextPageToken string
	if len(sites) > pageSize {
		sites = sites[:pageSize]
		offset := 0
		if filter != nil && filter.PageToken != "" {
			_, _ = fmt.Sscanf(filter.PageToken, "%d", &offset)
		}
		nextPageToken = fmt.Sprintf("%d", offset+pageSize)
	}

	return sites, nextPageToken, nil
}

// Create creates a new site.
func (s *PostgresStore) Create(ctx context.Context, site *Site) (*Site, error) {
	if site == nil || site.Code == "" || site.Name == "" {
		return nil, ErrInvalidSite
	}

	if site.ID == "" {
		site.ID = uuid.New().String()
	}

	now := time.Now()
	site.CreatedAt = now
	site.UpdatedAt = now

	labelsJSON, _ := json.Marshal(site.Labels)
	var bhJSON []byte
	if site.BusinessHours != nil {
		bhJSON, _ = json.Marshal(site.BusinessHours)
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sites (id, name, code, site_type, tier, region, country, city, address, timezone,
						   primary_team_id, secondary_team_id, default_escalation_policy_id, parent_site_id,
						   labels, business_hours, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`, site.ID, site.Name, site.Code, site.SiteType, site.Tier,
		nullableString(site.Region), nullableString(site.Country), nullableString(site.City), nullableString(site.Address),
		site.Timezone, site.PrimaryTeamID, site.SecondaryTeamID, site.DefaultEscalationPolicyID, site.ParentSiteID,
		labelsJSON, bhJSON, site.CreatedAt, site.UpdatedAt)
	if err != nil {
		// Check for unique constraint violation
		if isUniqueViolation(err) {
			return nil, ErrDuplicateCode
		}
		return nil, fmt.Errorf("insert site: %w", err)
	}

	return site, nil
}

// Update updates an existing site.
func (s *PostgresStore) Update(ctx context.Context, site *Site) (*Site, error) {
	if site == nil || site.ID == "" {
		return nil, ErrInvalidSite
	}

	site.UpdatedAt = time.Now()

	labelsJSON, _ := json.Marshal(site.Labels)
	var bhJSON []byte
	if site.BusinessHours != nil {
		bhJSON, _ = json.Marshal(site.BusinessHours)
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE sites SET name = $1, code = $2, site_type = $3, tier = $4,
						 region = $5, country = $6, city = $7, address = $8, timezone = $9,
						 primary_team_id = $10, secondary_team_id = $11, default_escalation_policy_id = $12,
						 parent_site_id = $13, labels = $14, business_hours = $15, updated_at = $16
		WHERE id = $17
	`, site.Name, site.Code, site.SiteType, site.Tier,
		nullableString(site.Region), nullableString(site.Country), nullableString(site.City), nullableString(site.Address),
		site.Timezone, site.PrimaryTeamID, site.SecondaryTeamID, site.DefaultEscalationPolicyID,
		site.ParentSiteID, labelsJSON, bhJSON, site.UpdatedAt, site.ID)
	if err != nil {
		return nil, fmt.Errorf("update site: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, ErrSiteNotFound
	}

	return site, nil
}

// Delete deletes a site by ID.
func (s *PostgresStore) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM sites WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete site: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrSiteNotFound
	}

	return nil
}

// GetTeamByID retrieves a team by its ID.
func (s *PostgresStore) GetTeamByID(ctx context.Context, id string) (*Team, error) {
	team := &Team{}
	var description sql.NullString
	var defaultEscPolicyID, defaultNotifChannelID sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, default_escalation_policy_id, default_notification_channel_id,
			   created_at, updated_at
		FROM teams WHERE id = $1
	`, id).Scan(
		&team.ID, &team.Name, &description, &defaultEscPolicyID, &defaultNotifChannelID,
		&team.CreatedAt, &team.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTeamNotFound
		}
		return nil, fmt.Errorf("query team by id: %w", err)
	}

	if description.Valid {
		team.Description = description.String
	}
	if defaultEscPolicyID.Valid {
		team.DefaultEscalationPolicyID = &defaultEscPolicyID.String
	}
	if defaultNotifChannelID.Valid {
		team.DefaultNotificationChannelID = &defaultNotifChannelID.String
	}

	return team, nil
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
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
