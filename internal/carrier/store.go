// Package carrier provides carrier management and BGP alert handling for the on-call system.
package carrier

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

var (
	// ErrNotFound is returned when a carrier is not found.
	ErrNotFound = errors.New("carrier not found")
	// ErrInvalidCarrier is returned when a carrier is invalid.
	ErrInvalidCarrier = errors.New("invalid carrier")
	// ErrDuplicateASN is returned when a carrier with the same ASN already exists.
	ErrDuplicateASN = errors.New("carrier with this ASN already exists")
	// ErrDuplicateName is returned when a carrier name already exists.
	ErrDuplicateName = errors.New("carrier name already exists")
)

// CarrierType represents the type of carrier relationship.
type CarrierType string

const (
	CarrierTypeTransit   CarrierType = "transit"
	CarrierTypePeering   CarrierType = "peering"
	CarrierTypeCustomer  CarrierType = "customer"
	CarrierTypeProvider  CarrierType = "provider"
	CarrierTypeIXP       CarrierType = "ixp"
)

// Contact represents a carrier contact.
type Contact struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Role     string `json:"role"`
	Primary  bool   `json:"primary"`
}

// Carrier represents a network carrier/peer.
type Carrier struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	ASN              int               `json:"asn"`
	Type             CarrierType       `json:"type"`
	Priority         int               `json:"priority"`
	Contacts         []Contact         `json:"contacts"`
	EscalationPolicy string            `json:"escalationPolicy"`
	NOCEmail         string            `json:"nocEmail"`
	NOCPhone         string            `json:"nocPhone"`
	NOCPortalURL     string            `json:"nocPortalUrl"`
	TeamID           string            `json:"teamId"`
	AutoTicket       bool              `json:"autoTicket"`
	TicketProviderID string            `json:"ticketProviderId"`
	Metadata         map[string]string `json:"metadata"`
	CreatedAt        time.Time         `json:"createdAt"`
	UpdatedAt        time.Time         `json:"updatedAt"`
}

// CarrierFilter defines filter criteria for listing carriers.
type CarrierFilter struct {
	Type         CarrierType
	NameContains string
	PageSize     int
	PageToken    string
}

// Store defines the interface for carrier persistence.
type Store interface {
	// Create creates a new carrier.
	Create(ctx context.Context, carrier *Carrier) (*Carrier, error)

	// GetByID retrieves a carrier by its ID.
	GetByID(ctx context.Context, id string) (*Carrier, error)

	// GetByASN retrieves a carrier by its Autonomous System Number.
	GetByASN(ctx context.Context, asn int) (*Carrier, error)

	// GetByName retrieves a carrier by its name.
	GetByName(ctx context.Context, name string) (*Carrier, error)

	// List retrieves carriers based on filter criteria.
	List(ctx context.Context, filter *CarrierFilter) ([]*Carrier, error)

	// Update updates an existing carrier.
	Update(ctx context.Context, carrier *Carrier) (*Carrier, error)

	// Delete deletes a carrier by ID.
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

// Create creates a new carrier in the database.
func (s *PostgresStore) Create(ctx context.Context, carrier *Carrier) (*Carrier, error) {
	if carrier == nil {
		return nil, ErrInvalidCarrier
	}

	if carrier.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidCarrier)
	}

	if carrier.ASN <= 0 {
		return nil, fmt.Errorf("%w: valid ASN is required", ErrInvalidCarrier)
	}

	// Generate ID if not provided
	if carrier.ID == "" {
		carrier.ID = uuid.New().String()
	}

	now := time.Now()
	carrier.CreatedAt = now
	carrier.UpdatedAt = now

	// Marshal contacts and metadata
	contactsJSON, err := json.Marshal(carrier.Contacts)
	if err != nil {
		return nil, fmt.Errorf("marshal contacts: %w", err)
	}

	metadataJSON, err := json.Marshal(carrier.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO carriers (
			id, name, asn, carrier_type, priority, contacts, escalation_policy_id,
			noc_email, noc_phone, noc_portal_url, team_id, auto_ticket, ticket_provider_id,
			metadata, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`, carrier.ID, carrier.Name, carrier.ASN, string(carrier.Type), carrier.Priority,
		contactsJSON, nullableString(carrier.EscalationPolicy),
		nullableString(carrier.NOCEmail), nullableString(carrier.NOCPhone),
		nullableString(carrier.NOCPortalURL), nullableString(carrier.TeamID),
		carrier.AutoTicket, nullableString(carrier.TicketProviderID),
		metadataJSON, now, now)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			if strings.Contains(err.Error(), "asn") {
				return nil, ErrDuplicateASN
			}
			return nil, ErrDuplicateName
		}
		return nil, fmt.Errorf("insert carrier: %w", err)
	}

	return carrier, nil
}

// GetByID retrieves a carrier by its ID.
func (s *PostgresStore) GetByID(ctx context.Context, id string) (*Carrier, error) {
	return s.getCarrier(ctx, "id = $1", id)
}

// GetByASN retrieves a carrier by its Autonomous System Number.
func (s *PostgresStore) GetByASN(ctx context.Context, asn int) (*Carrier, error) {
	return s.getCarrier(ctx, "asn = $1", asn)
}

// GetByName retrieves a carrier by its name.
func (s *PostgresStore) GetByName(ctx context.Context, name string) (*Carrier, error) {
	return s.getCarrier(ctx, "name = $1", name)
}

// getCarrier is a helper to retrieve a carrier with a given condition.
func (s *PostgresStore) getCarrier(ctx context.Context, condition string, arg interface{}) (*Carrier, error) {
	carrier := &Carrier{}

	var carrierType string
	var contactsJSON, metadataJSON []byte
	var escalationPolicy, nocEmail, nocPhone, nocPortalURL, teamID, ticketProviderID sql.NullString

	query := fmt.Sprintf(`
		SELECT id, name, asn, carrier_type, priority, contacts, escalation_policy_id,
			noc_email, noc_phone, noc_portal_url, team_id, auto_ticket, ticket_provider_id,
			metadata, created_at, updated_at
		FROM carriers WHERE %s
	`, condition)

	err := s.db.QueryRowContext(ctx, query, arg).Scan(
		&carrier.ID, &carrier.Name, &carrier.ASN, &carrierType, &carrier.Priority,
		&contactsJSON, &escalationPolicy, &nocEmail, &nocPhone, &nocPortalURL,
		&teamID, &carrier.AutoTicket, &ticketProviderID, &metadataJSON,
		&carrier.CreatedAt, &carrier.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query carrier: %w", err)
	}

	carrier.Type = CarrierType(carrierType)
	carrier.EscalationPolicy = escalationPolicy.String
	carrier.NOCEmail = nocEmail.String
	carrier.NOCPhone = nocPhone.String
	carrier.NOCPortalURL = nocPortalURL.String
	carrier.TeamID = teamID.String
	carrier.TicketProviderID = ticketProviderID.String

	if contactsJSON != nil {
		if err := json.Unmarshal(contactsJSON, &carrier.Contacts); err != nil {
			return nil, fmt.Errorf("unmarshal contacts: %w", err)
		}
	}

	if metadataJSON != nil {
		if err := json.Unmarshal(metadataJSON, &carrier.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
	}

	return carrier, nil
}

// List retrieves carriers based on filter criteria.
func (s *PostgresStore) List(ctx context.Context, filter *CarrierFilter) ([]*Carrier, error) {
	query := `SELECT id, name, asn, carrier_type, priority, contacts, escalation_policy_id,
		noc_email, noc_phone, noc_portal_url, team_id, auto_ticket, ticket_provider_id,
		metadata, created_at, updated_at
		FROM carriers WHERE 1=1`
	args := []interface{}{}
	argIndex := 1

	if filter != nil {
		if filter.Type != "" {
			query += fmt.Sprintf(" AND carrier_type = $%d", argIndex)
			args = append(args, string(filter.Type))
			argIndex++
		}

		if filter.NameContains != "" {
			query += fmt.Sprintf(" AND name ILIKE $%d", argIndex)
			args = append(args, "%"+filter.NameContains+"%")
			argIndex++
		}
	}

	query += " ORDER BY priority ASC, name ASC"

	pageSize := 50
	if filter != nil && filter.PageSize > 0 && filter.PageSize <= 100 {
		pageSize = filter.PageSize
	}
	query += fmt.Sprintf(" LIMIT $%d", argIndex)
	args = append(args, pageSize+1)
	argIndex++

	if filter != nil && filter.PageToken != "" {
		offset := decodePageToken(filter.PageToken)
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query carriers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var carriers []*Carrier
	for rows.Next() {
		carrier := &Carrier{}
		var carrierType string
		var contactsJSON, metadataJSON []byte
		var escalationPolicy, nocEmail, nocPhone, nocPortalURL, teamID, ticketProviderID sql.NullString

		if err := rows.Scan(
			&carrier.ID, &carrier.Name, &carrier.ASN, &carrierType, &carrier.Priority,
			&contactsJSON, &escalationPolicy, &nocEmail, &nocPhone, &nocPortalURL,
			&teamID, &carrier.AutoTicket, &ticketProviderID, &metadataJSON,
			&carrier.CreatedAt, &carrier.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan carrier: %w", err)
		}

		carrier.Type = CarrierType(carrierType)
		carrier.EscalationPolicy = escalationPolicy.String
		carrier.NOCEmail = nocEmail.String
		carrier.NOCPhone = nocPhone.String
		carrier.NOCPortalURL = nocPortalURL.String
		carrier.TeamID = teamID.String
		carrier.TicketProviderID = ticketProviderID.String

		if contactsJSON != nil {
			_ = json.Unmarshal(contactsJSON, &carrier.Contacts)
		}

		if metadataJSON != nil {
			_ = json.Unmarshal(metadataJSON, &carrier.Metadata)
		}

		carriers = append(carriers, carrier)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return carriers, nil
}

// Update updates an existing carrier.
func (s *PostgresStore) Update(ctx context.Context, carrier *Carrier) (*Carrier, error) {
	if carrier == nil || carrier.ID == "" {
		return nil, ErrInvalidCarrier
	}

	carrier.UpdatedAt = time.Now()

	contactsJSON, err := json.Marshal(carrier.Contacts)
	if err != nil {
		return nil, fmt.Errorf("marshal contacts: %w", err)
	}

	metadataJSON, err := json.Marshal(carrier.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE carriers SET
			name = $1, asn = $2, carrier_type = $3, priority = $4, contacts = $5,
			escalation_policy_id = $6, noc_email = $7, noc_phone = $8, noc_portal_url = $9,
			team_id = $10, auto_ticket = $11, ticket_provider_id = $12, metadata = $13, updated_at = $14
		WHERE id = $15
	`, carrier.Name, carrier.ASN, string(carrier.Type), carrier.Priority, contactsJSON,
		nullableString(carrier.EscalationPolicy), nullableString(carrier.NOCEmail),
		nullableString(carrier.NOCPhone), nullableString(carrier.NOCPortalURL),
		nullableString(carrier.TeamID), carrier.AutoTicket,
		nullableString(carrier.TicketProviderID), metadataJSON, carrier.UpdatedAt, carrier.ID)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			if strings.Contains(err.Error(), "asn") {
				return nil, ErrDuplicateASN
			}
			return nil, ErrDuplicateName
		}
		return nil, fmt.Errorf("update carrier: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return carrier, nil
}

// Delete deletes a carrier by ID.
func (s *PostgresStore) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM carriers WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete carrier: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// ToProto converts a Carrier to its protobuf representation.
func (c *Carrier) ToProto() *routingv1.CarrierConfig {
	if c == nil {
		return nil
	}

	return &routingv1.CarrierConfig{
		Id:               c.ID,
		Name:             c.Name,
		Asn:              fmt.Sprintf("%d", c.ASN),
		NocEmail:         c.NOCEmail,
		NocPhone:         c.NOCPhone,
		NocPortalUrl:     c.NOCPortalURL,
		TeamId:           c.TeamID,
		AutoTicket:       c.AutoTicket,
		TicketProviderId: c.TicketProviderID,
	}
}

// FromProto creates a Carrier from its protobuf representation.
func FromProto(pb *routingv1.CarrierConfig) *Carrier {
	if pb == nil {
		return nil
	}

	var asn int
	_, _ = fmt.Sscanf(pb.Asn, "%d", &asn)

	return &Carrier{
		ID:               pb.Id,
		Name:             pb.Name,
		ASN:              asn,
		NOCEmail:         pb.NocEmail,
		NOCPhone:         pb.NocPhone,
		NOCPortalURL:     pb.NocPortalUrl,
		TeamID:           pb.TeamId,
		AutoTicket:       pb.AutoTicket,
		TicketProviderID: pb.TicketProviderId,
	}
}

// Helper functions

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func decodePageToken(token string) int {
	var offset int
	_, _ = fmt.Sscanf(token, "%d", &offset)
	return offset
}

// Ensure PostgresStore implements Store
var _ Store = (*PostgresStore)(nil)

// Silence unused import warning for timestamppb
var _ = timestamppb.Now
