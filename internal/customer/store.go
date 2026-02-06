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
	// ErrCustomerNotFound is returned when a customer cannot be found.
	ErrCustomerNotFound = errors.New("customer not found")
	// ErrInvalidCustomer is returned when a customer is invalid.
	ErrInvalidCustomer = errors.New("invalid customer")
	// ErrDuplicateAccountID is returned when an account ID already exists.
	ErrDuplicateAccountID = errors.New("duplicate account ID")
)

// Store defines the interface for customer persistence.
type Store interface {
	// Create creates a new customer.
	Create(ctx context.Context, customer *Customer) (*Customer, error)

	// GetByID retrieves a customer by ID.
	GetByID(ctx context.Context, id string) (*Customer, error)

	// GetByAccountID retrieves a customer by account ID.
	GetByAccountID(ctx context.Context, accountID string) (*Customer, error)

	// GetByDomain retrieves a customer by domain.
	GetByDomain(ctx context.Context, domain string) (*Customer, error)

	// GetByIPRange retrieves customers that contain the given IP in their ranges.
	GetByIPRange(ctx context.Context, ip string) ([]*Customer, error)

	// List retrieves customers with optional filters.
	List(ctx context.Context, filter *ListCustomersFilter) ([]*Customer, string, error)

	// Update updates an existing customer.
	Update(ctx context.Context, customer *Customer) (*Customer, error)

	// Delete deletes a customer by ID.
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

// Create creates a new customer in the database.
func (s *PostgresStore) Create(ctx context.Context, customer *Customer) (*Customer, error) {
	if customer == nil || customer.Name == "" || customer.AccountID == "" {
		return nil, ErrInvalidCustomer
	}

	if customer.ID == "" {
		customer.ID = uuid.New().String()
	}

	now := time.Now()
	customer.CreatedAt = now
	customer.UpdatedAt = now

	domainsJSON, _ := json.Marshal(customer.Domains)
	ipRangesJSON, _ := json.Marshal(customer.IPRanges)
	contactsJSON, _ := json.Marshal(customer.Contacts)
	metadataJSON, _ := json.Marshal(customer.Metadata)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO customers (
			id, name, account_id, tier_id, description,
			domains, ip_ranges, contacts, metadata,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`,
		customer.ID, customer.Name, customer.AccountID, customer.TierID, customer.Description,
		domainsJSON, ipRangesJSON, contactsJSON, metadataJSON,
		customer.CreatedAt, customer.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicateAccountID
		}
		return nil, fmt.Errorf("insert customer: %w", err)
	}

	return customer, nil
}

// GetByID retrieves a customer by ID.
func (s *PostgresStore) GetByID(ctx context.Context, id string) (*Customer, error) {
	return s.getByField(ctx, "id", id)
}

// GetByAccountID retrieves a customer by account ID.
func (s *PostgresStore) GetByAccountID(ctx context.Context, accountID string) (*Customer, error) {
	return s.getByField(ctx, "account_id", accountID)
}

// GetByDomain retrieves a customer by domain.
func (s *PostgresStore) GetByDomain(ctx context.Context, domain string) (*Customer, error) {
	customer := &Customer{}
	var description sql.NullString
	var domainsJSON, ipRangesJSON, contactsJSON, metadataJSON []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, account_id, tier_id, description,
			   domains, ip_ranges, contacts, metadata,
			   created_at, updated_at
		FROM customers
		WHERE domains @> $1::jsonb
	`, fmt.Sprintf(`["%s"]`, domain)).Scan(
		&customer.ID, &customer.Name, &customer.AccountID, &customer.TierID, &description,
		&domainsJSON, &ipRangesJSON, &contactsJSON, &metadataJSON,
		&customer.CreatedAt, &customer.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCustomerNotFound
		}
		return nil, fmt.Errorf("query customer by domain: %w", err)
	}

	customer.Description = description.String
	s.parseJSONFields(customer, domainsJSON, ipRangesJSON, contactsJSON, metadataJSON)

	return customer, nil
}

// GetByIPRange retrieves customers that contain the given IP in their ranges.
func (s *PostgresStore) GetByIPRange(ctx context.Context, ip string) ([]*Customer, error) {
	// For PostgreSQL, we need to check each customer's IP ranges
	// This is a simplified implementation that loads all customers and filters in memory
	// A production implementation might use PostgreSQL's inet type for better performance
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, account_id, tier_id, description,
			   domains, ip_ranges, contacts, metadata,
			   created_at, updated_at
		FROM customers
		WHERE ip_ranges IS NOT NULL AND ip_ranges != '[]'::jsonb
	`)
	if err != nil {
		return nil, fmt.Errorf("query customers by IP range: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var customers []*Customer
	for rows.Next() {
		customer := &Customer{}
		var description sql.NullString
		var domainsJSON, ipRangesJSON, contactsJSON, metadataJSON []byte

		if err := rows.Scan(
			&customer.ID, &customer.Name, &customer.AccountID, &customer.TierID, &description,
			&domainsJSON, &ipRangesJSON, &contactsJSON, &metadataJSON,
			&customer.CreatedAt, &customer.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan customer: %w", err)
		}

		customer.Description = description.String
		s.parseJSONFields(customer, domainsJSON, ipRangesJSON, contactsJSON, metadataJSON)

		// Check if IP is in any of the customer's ranges
		ranges, err := ParseIPRanges(customer.IPRanges)
		if err != nil {
			continue // Skip invalid ranges
		}
		if ContainsIP(ranges, ip) {
			customers = append(customers, customer)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return customers, nil
}

// getByField retrieves a customer by a specific field.
func (s *PostgresStore) getByField(ctx context.Context, field, value string) (*Customer, error) {
	customer := &Customer{}
	var description sql.NullString
	var domainsJSON, ipRangesJSON, contactsJSON, metadataJSON []byte

	query := fmt.Sprintf(`
		SELECT id, name, account_id, tier_id, description,
			   domains, ip_ranges, contacts, metadata,
			   created_at, updated_at
		FROM customers WHERE %s = $1
	`, field)

	err := s.db.QueryRowContext(ctx, query, value).Scan(
		&customer.ID, &customer.Name, &customer.AccountID, &customer.TierID, &description,
		&domainsJSON, &ipRangesJSON, &contactsJSON, &metadataJSON,
		&customer.CreatedAt, &customer.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCustomerNotFound
		}
		return nil, fmt.Errorf("query customer by %s: %w", field, err)
	}

	customer.Description = description.String
	s.parseJSONFields(customer, domainsJSON, ipRangesJSON, contactsJSON, metadataJSON)

	return customer, nil
}

// parseJSONFields parses JSON fields into the customer struct.
func (s *PostgresStore) parseJSONFields(customer *Customer, domainsJSON, ipRangesJSON, contactsJSON, metadataJSON []byte) {
	if domainsJSON != nil {
		_ = json.Unmarshal(domainsJSON, &customer.Domains)
	}
	if ipRangesJSON != nil {
		_ = json.Unmarshal(ipRangesJSON, &customer.IPRanges)
	}
	if contactsJSON != nil {
		_ = json.Unmarshal(contactsJSON, &customer.Contacts)
	}
	if metadataJSON != nil {
		_ = json.Unmarshal(metadataJSON, &customer.Metadata)
	}
	if customer.Metadata == nil {
		customer.Metadata = make(map[string]string)
	}
}

// List retrieves customers with optional filters.
func (s *PostgresStore) List(ctx context.Context, filter *ListCustomersFilter) ([]*Customer, string, error) {
	query := `
		SELECT id, name, account_id, tier_id, description,
			   domains, ip_ranges, contacts, metadata,
			   created_at, updated_at
		FROM customers WHERE 1=1`
	args := []interface{}{}
	argIndex := 1

	if filter != nil {
		if filter.TierID != "" {
			query += fmt.Sprintf(" AND tier_id = $%d", argIndex)
			args = append(args, filter.TierID)
			argIndex++
		}

		if filter.AccountID != "" {
			query += fmt.Sprintf(" AND account_id = $%d", argIndex)
			args = append(args, filter.AccountID)
			argIndex++
		}

		if filter.Domain != "" {
			query += fmt.Sprintf(" AND domains @> $%d::jsonb", argIndex)
			args = append(args, fmt.Sprintf(`["%s"]`, filter.Domain))
			argIndex++
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
		return nil, "", fmt.Errorf("query customers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var customers []*Customer
	for rows.Next() {
		customer := &Customer{}
		var description sql.NullString
		var domainsJSON, ipRangesJSON, contactsJSON, metadataJSON []byte

		if err := rows.Scan(
			&customer.ID, &customer.Name, &customer.AccountID, &customer.TierID, &description,
			&domainsJSON, &ipRangesJSON, &contactsJSON, &metadataJSON,
			&customer.CreatedAt, &customer.UpdatedAt,
		); err != nil {
			return nil, "", fmt.Errorf("scan customer: %w", err)
		}

		customer.Description = description.String
		s.parseJSONFields(customer, domainsJSON, ipRangesJSON, contactsJSON, metadataJSON)

		customers = append(customers, customer)
	}

	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	// Pagination token
	var nextPageToken string
	if len(customers) > pageSize {
		customers = customers[:pageSize]
		offset := 0
		if filter != nil && filter.PageToken != "" {
			_, _ = fmt.Sscanf(filter.PageToken, "%d", &offset)
		}
		nextPageToken = fmt.Sprintf("%d", offset+pageSize)
	}

	return customers, nextPageToken, nil
}

// Update updates an existing customer.
func (s *PostgresStore) Update(ctx context.Context, customer *Customer) (*Customer, error) {
	if customer == nil || customer.ID == "" {
		return nil, ErrInvalidCustomer
	}

	customer.UpdatedAt = time.Now()

	domainsJSON, _ := json.Marshal(customer.Domains)
	ipRangesJSON, _ := json.Marshal(customer.IPRanges)
	contactsJSON, _ := json.Marshal(customer.Contacts)
	metadataJSON, _ := json.Marshal(customer.Metadata)

	result, err := s.db.ExecContext(ctx, `
		UPDATE customers SET
			name = $1, account_id = $2, tier_id = $3, description = $4,
			domains = $5, ip_ranges = $6, contacts = $7, metadata = $8,
			updated_at = $9
		WHERE id = $10
	`,
		customer.Name, customer.AccountID, customer.TierID, customer.Description,
		domainsJSON, ipRangesJSON, contactsJSON, metadataJSON,
		customer.UpdatedAt, customer.ID,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicateAccountID
		}
		return nil, fmt.Errorf("update customer: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, ErrCustomerNotFound
	}

	return customer, nil
}

// Delete deletes a customer by ID.
func (s *PostgresStore) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM customers WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete customer: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrCustomerNotFound
	}

	return nil
}

// InMemoryStore is an in-memory implementation of Store for testing.
type InMemoryStore struct {
	customers map[string]*Customer
	counter   int64
}

// NewInMemoryStore creates a new in-memory store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		customers: make(map[string]*Customer),
	}
}

// Create creates a new customer in memory.
func (s *InMemoryStore) Create(ctx context.Context, customer *Customer) (*Customer, error) {
	if customer == nil || customer.Name == "" || customer.AccountID == "" {
		return nil, ErrInvalidCustomer
	}

	// Check for duplicate account ID
	for _, c := range s.customers {
		if c.AccountID == customer.AccountID {
			return nil, ErrDuplicateAccountID
		}
	}

	if customer.ID == "" {
		s.counter++
		customer.ID = fmt.Sprintf("customer-%d", s.counter)
	}

	now := time.Now()
	customer.CreatedAt = now
	customer.UpdatedAt = now

	// Deep copy
	stored := *customer
	if customer.Domains != nil {
		stored.Domains = make([]string, len(customer.Domains))
		copy(stored.Domains, customer.Domains)
	}
	if customer.IPRanges != nil {
		stored.IPRanges = make([]string, len(customer.IPRanges))
		copy(stored.IPRanges, customer.IPRanges)
	}
	if customer.Contacts != nil {
		stored.Contacts = make([]CustomerContact, len(customer.Contacts))
		copy(stored.Contacts, customer.Contacts)
	}
	if customer.Metadata != nil {
		stored.Metadata = make(map[string]string)
		for k, v := range customer.Metadata {
			stored.Metadata[k] = v
		}
	}
	s.customers[customer.ID] = &stored

	return customer, nil
}

// GetByID retrieves a customer by ID.
func (s *InMemoryStore) GetByID(ctx context.Context, id string) (*Customer, error) {
	customer, ok := s.customers[id]
	if !ok {
		return nil, ErrCustomerNotFound
	}
	return customer, nil
}

// GetByAccountID retrieves a customer by account ID.
func (s *InMemoryStore) GetByAccountID(ctx context.Context, accountID string) (*Customer, error) {
	for _, customer := range s.customers {
		if customer.AccountID == accountID {
			return customer, nil
		}
	}
	return nil, ErrCustomerNotFound
}

// GetByDomain retrieves a customer by domain.
func (s *InMemoryStore) GetByDomain(ctx context.Context, domain string) (*Customer, error) {
	for _, customer := range s.customers {
		for _, d := range customer.Domains {
			if d == domain {
				return customer, nil
			}
		}
	}
	return nil, ErrCustomerNotFound
}

// GetByIPRange retrieves customers that contain the given IP in their ranges.
func (s *InMemoryStore) GetByIPRange(ctx context.Context, ip string) ([]*Customer, error) {
	var customers []*Customer
	for _, customer := range s.customers {
		ranges, err := ParseIPRanges(customer.IPRanges)
		if err != nil {
			continue
		}
		if ContainsIP(ranges, ip) {
			customers = append(customers, customer)
		}
	}
	return customers, nil
}

// List retrieves customers with optional filters.
func (s *InMemoryStore) List(ctx context.Context, filter *ListCustomersFilter) ([]*Customer, string, error) {
	var customers []*Customer
	for _, customer := range s.customers {
		if filter != nil {
			if filter.TierID != "" && customer.TierID != filter.TierID {
				continue
			}
			if filter.AccountID != "" && customer.AccountID != filter.AccountID {
				continue
			}
			if filter.Domain != "" {
				found := false
				for _, d := range customer.Domains {
					if d == filter.Domain {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
		}
		customers = append(customers, customer)
	}

	// Sort by name
	for i := 0; i < len(customers)-1; i++ {
		for j := i + 1; j < len(customers); j++ {
			if customers[i].Name > customers[j].Name {
				customers[i], customers[j] = customers[j], customers[i]
			}
		}
	}

	return customers, "", nil
}

// Update updates an existing customer.
func (s *InMemoryStore) Update(ctx context.Context, customer *Customer) (*Customer, error) {
	if customer == nil || customer.ID == "" {
		return nil, ErrInvalidCustomer
	}

	existing, ok := s.customers[customer.ID]
	if !ok {
		return nil, ErrCustomerNotFound
	}

	// Check for duplicate account ID (excluding this customer)
	for _, c := range s.customers {
		if c.ID != customer.ID && c.AccountID == customer.AccountID {
			return nil, ErrDuplicateAccountID
		}
	}

	customer.CreatedAt = existing.CreatedAt
	customer.UpdatedAt = time.Now()

	// Deep copy
	stored := *customer
	if customer.Domains != nil {
		stored.Domains = make([]string, len(customer.Domains))
		copy(stored.Domains, customer.Domains)
	}
	if customer.IPRanges != nil {
		stored.IPRanges = make([]string, len(customer.IPRanges))
		copy(stored.IPRanges, customer.IPRanges)
	}
	if customer.Contacts != nil {
		stored.Contacts = make([]CustomerContact, len(customer.Contacts))
		copy(stored.Contacts, customer.Contacts)
	}
	if customer.Metadata != nil {
		stored.Metadata = make(map[string]string)
		for k, v := range customer.Metadata {
			stored.Metadata[k] = v
		}
	}
	s.customers[customer.ID] = &stored

	return customer, nil
}

// Delete deletes a customer by ID.
func (s *InMemoryStore) Delete(ctx context.Context, id string) error {
	if _, ok := s.customers[id]; !ok {
		return ErrCustomerNotFound
	}
	delete(s.customers, id)
	return nil
}

// Ensure interfaces are implemented
var _ Store = (*PostgresStore)(nil)
var _ Store = (*InMemoryStore)(nil)
