// Package routing provides the routing rule store implementation.
package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kneutral-org/alerting-system/internal/sqlc"
)

// RoutingRule represents a routing rule domain model.
type RoutingRule struct {
	ID            uuid.UUID          `json:"id"`
	Name          string             `json:"name"`
	Description   string             `json:"description,omitempty"`
	Priority      int32              `json:"priority"`
	Enabled       bool               `json:"enabled"`
	Conditions    []RoutingCondition `json:"conditions"`
	Actions       []RoutingAction    `json:"actions"`
	Terminal      bool               `json:"terminal"`
	TimeCondition *TimeCondition     `json:"timeCondition,omitempty"`
	Tags          []string           `json:"tags"`
	CreatedBy     *uuid.UUID         `json:"createdBy,omitempty"`
	CreatedAt     time.Time          `json:"createdAt"`
	UpdatedBy     *uuid.UUID         `json:"updatedBy,omitempty"`
	UpdatedAt     time.Time          `json:"updatedAt"`
}

// RoutingCondition represents a condition for matching alerts.
type RoutingCondition struct {
	Type     string   `json:"type"`
	Field    string   `json:"field,omitempty"`
	Operator string   `json:"operator"`
	Values   []string `json:"values"`
}

// RoutingAction represents an action to take when conditions match.
type RoutingAction struct {
	Type   string                 `json:"type"`
	Config map[string]interface{} `json:"config"`
}

// TimeCondition represents time-based conditions for a rule.
type TimeCondition struct {
	Timezone     string   `json:"timezone,omitempty"`
	DaysOfWeek   []int    `json:"daysOfWeek,omitempty"`
	StartTime    string   `json:"startTime,omitempty"`
	EndTime      string   `json:"endTime,omitempty"`
	ExcludeDates []string `json:"excludeDates,omitempty"`
}

// ListRulesParams contains parameters for listing routing rules.
type ListRulesParams struct {
	EnabledFilter []bool
	TagsFilter    []string
	Limit         int32
	Offset        int32
}

// RoutingRuleStore defines the interface for routing rule persistence.
type RoutingRuleStore interface {
	// CreateRule creates a new routing rule.
	CreateRule(ctx context.Context, rule *RoutingRule) (*RoutingRule, error)

	// GetRule retrieves a routing rule by ID.
	GetRule(ctx context.Context, id uuid.UUID) (*RoutingRule, error)

	// ListRules retrieves routing rules based on filter criteria.
	ListRules(ctx context.Context, params ListRulesParams) ([]*RoutingRule, error)

	// UpdateRule updates an existing routing rule.
	UpdateRule(ctx context.Context, rule *RoutingRule) (*RoutingRule, error)

	// DeleteRule deletes a routing rule.
	DeleteRule(ctx context.Context, id uuid.UUID) error

	// ReorderRules updates priorities for multiple rules.
	ReorderRules(ctx context.Context, priorities map[uuid.UUID]int32) error

	// GetEnabledRulesByPriority retrieves all enabled rules ordered by priority.
	GetEnabledRulesByPriority(ctx context.Context) ([]*RoutingRule, error)
}

// PostgresRoutingRuleStore is the PostgreSQL implementation of RoutingRuleStore.
type PostgresRoutingRuleStore struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

// NewPostgresRoutingRuleStore creates a new PostgreSQL routing rule store.
func NewPostgresRoutingRuleStore(pool *pgxpool.Pool) *PostgresRoutingRuleStore {
	return &PostgresRoutingRuleStore{
		pool:    pool,
		queries: sqlc.New(pool),
	}
}

// CreateRule creates a new routing rule.
func (s *PostgresRoutingRuleStore) CreateRule(ctx context.Context, rule *RoutingRule) (*RoutingRule, error) {
	conditionsJSON, err := json.Marshal(rule.Conditions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal conditions: %w", err)
	}

	actionsJSON, err := json.Marshal(rule.Actions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal actions: %w", err)
	}

	var timeConditionJSON []byte
	if rule.TimeCondition != nil {
		timeConditionJSON, err = json.Marshal(rule.TimeCondition)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal time condition: %w", err)
		}
	}

	params := sqlc.CreateRoutingRuleParams{
		Name:          rule.Name,
		Description:   toPgText(rule.Description),
		Priority:      rule.Priority,
		Enabled:       rule.Enabled,
		Conditions:    conditionsJSON,
		Actions:       actionsJSON,
		Terminal:      rule.Terminal,
		TimeCondition: timeConditionJSON,
		Tags:          rule.Tags,
		CreatedBy:     toPgUUID(rule.CreatedBy),
	}

	result, err := s.queries.CreateRoutingRule(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create routing rule: %w", err)
	}

	return sqlcToRoutingRule(&result)
}

// GetRule retrieves a routing rule by ID.
func (s *PostgresRoutingRuleStore) GetRule(ctx context.Context, id uuid.UUID) (*RoutingRule, error) {
	result, err := s.queries.GetRoutingRule(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get routing rule: %w", err)
	}

	return sqlcToRoutingRule(&result)
}

// ListRules retrieves routing rules based on filter criteria.
func (s *PostgresRoutingRuleStore) ListRules(ctx context.Context, params ListRulesParams) ([]*RoutingRule, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}

	result, err := s.queries.ListRoutingRules(ctx, sqlc.ListRoutingRulesParams{
		EnabledFilter: params.EnabledFilter,
		TagsFilter:    params.TagsFilter,
		Lim:           limit,
		Off:           params.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list routing rules: %w", err)
	}

	rules := make([]*RoutingRule, 0, len(result))
	for _, r := range result {
		rule, err := sqlcToRoutingRule(&r)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

// UpdateRule updates an existing routing rule.
func (s *PostgresRoutingRuleStore) UpdateRule(ctx context.Context, rule *RoutingRule) (*RoutingRule, error) {
	conditionsJSON, err := json.Marshal(rule.Conditions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal conditions: %w", err)
	}

	actionsJSON, err := json.Marshal(rule.Actions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal actions: %w", err)
	}

	var timeConditionJSON []byte
	if rule.TimeCondition != nil {
		timeConditionJSON, err = json.Marshal(rule.TimeCondition)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal time condition: %w", err)
		}
	}

	params := sqlc.UpdateRoutingRuleParams{
		ID:            rule.ID,
		Name:          rule.Name,
		Description:   toPgText(rule.Description),
		Priority:      rule.Priority,
		Enabled:       rule.Enabled,
		Conditions:    conditionsJSON,
		Actions:       actionsJSON,
		Terminal:      rule.Terminal,
		TimeCondition: timeConditionJSON,
		Tags:          rule.Tags,
		UpdatedBy:     toPgUUID(rule.UpdatedBy),
	}

	result, err := s.queries.UpdateRoutingRule(ctx, params)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to update routing rule: %w", err)
	}

	return sqlcToRoutingRule(&result)
}

// DeleteRule deletes a routing rule.
func (s *PostgresRoutingRuleStore) DeleteRule(ctx context.Context, id uuid.UUID) error {
	err := s.queries.DeleteRoutingRule(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete routing rule: %w", err)
	}
	return nil
}

// ReorderRules updates priorities for multiple rules.
func (s *PostgresRoutingRuleStore) ReorderRules(ctx context.Context, priorities map[uuid.UUID]int32) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.queries.WithTx(tx)

	for id, priority := range priorities {
		err := qtx.UpdateRoutingRulePriority(ctx, sqlc.UpdateRoutingRulePriorityParams{
			ID:       id,
			Priority: priority,
		})
		if err != nil {
			return fmt.Errorf("failed to update priority for rule %s: %w", id, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetEnabledRulesByPriority retrieves all enabled rules ordered by priority.
func (s *PostgresRoutingRuleStore) GetEnabledRulesByPriority(ctx context.Context) ([]*RoutingRule, error) {
	result, err := s.queries.GetEnabledRulesByPriority(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled rules: %w", err)
	}

	rules := make([]*RoutingRule, 0, len(result))
	for _, r := range result {
		rule, err := sqlcToRoutingRule(&r)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

// Helper functions

func sqlcToRoutingRule(r *sqlc.RoutingRule) (*RoutingRule, error) {
	rule := &RoutingRule{
		ID:        r.ID,
		Name:      r.Name,
		Priority:  r.Priority,
		Enabled:   r.Enabled,
		Terminal:  r.Terminal,
		Tags:      r.Tags,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}

	if r.Description.Valid {
		rule.Description = r.Description.String
	}

	if r.CreatedBy.Valid {
		id := uuid.UUID(r.CreatedBy.Bytes)
		rule.CreatedBy = &id
	}

	if r.UpdatedBy.Valid {
		id := uuid.UUID(r.UpdatedBy.Bytes)
		rule.UpdatedBy = &id
	}

	if len(r.Conditions) > 0 {
		if err := json.Unmarshal(r.Conditions, &rule.Conditions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal conditions: %w", err)
		}
	}

	if len(r.Actions) > 0 {
		if err := json.Unmarshal(r.Actions, &rule.Actions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal actions: %w", err)
		}
	}

	if len(r.TimeCondition) > 0 {
		rule.TimeCondition = &TimeCondition{}
		if err := json.Unmarshal(r.TimeCondition, rule.TimeCondition); err != nil {
			return nil, fmt.Errorf("failed to unmarshal time condition: %w", err)
		}
	}

	return rule, nil
}

func toPgText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

func toPgUUID(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}
