// Package routing provides the routing engine for the alerting system.
package routing

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

var (
	// ErrNotFound is returned when a routing rule is not found.
	ErrNotFound = errors.New("routing rule not found")
	// ErrDuplicatePriority is returned when a priority conflict occurs.
	ErrDuplicatePriority = errors.New("duplicate priority")
	// ErrInvalidRule is returned when a rule is invalid.
	ErrInvalidRule = errors.New("invalid routing rule")
)

// Store defines the interface for routing rule persistence.
type Store interface {
	// CreateRule creates a new routing rule.
	CreateRule(ctx context.Context, rule *routingv1.RoutingRule) (*routingv1.RoutingRule, error)

	// GetRule retrieves a routing rule by ID.
	GetRule(ctx context.Context, id string) (*routingv1.RoutingRule, error)

	// ListRules retrieves routing rules with optional filters.
	ListRules(ctx context.Context, req *routingv1.ListRoutingRulesRequest) (*routingv1.ListRoutingRulesResponse, error)

	// UpdateRule updates an existing routing rule.
	UpdateRule(ctx context.Context, rule *routingv1.RoutingRule) (*routingv1.RoutingRule, error)

	// DeleteRule deletes a routing rule by ID.
	DeleteRule(ctx context.Context, id string) error

	// ReorderRules updates the priorities of multiple rules.
	ReorderRules(ctx context.Context, priorities map[string]int32) ([]*routingv1.RoutingRule, error)

	// GetAuditLogs retrieves routing audit logs.
	GetAuditLogs(ctx context.Context, req *routingv1.GetRoutingAuditLogsRequest) (*routingv1.GetRoutingAuditLogsResponse, error)

	// CreateAuditLog creates a new audit log entry.
	CreateAuditLog(ctx context.Context, log *routingv1.RoutingAuditLog) error

	// GetEnabledRulesByPriority retrieves all enabled rules ordered by priority.
	GetEnabledRulesByPriority(ctx context.Context) ([]*routingv1.RoutingRule, error)
}

// PostgresStore implements Store using PostgreSQL.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore creates a new PostgresStore.
func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

// CreateRule creates a new routing rule in the database.
func (s *PostgresStore) CreateRule(ctx context.Context, rule *routingv1.RoutingRule) (*routingv1.RoutingRule, error) {
	if rule == nil {
		return nil, ErrInvalidRule
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Generate ID if not provided
	if rule.Id == "" {
		rule.Id = uuid.New().String()
	}

	now := time.Now()
	rule.CreatedAt = timestamppb.New(now)
	rule.UpdatedAt = timestamppb.New(now)

	// Insert the rule
	_, err = tx.ExecContext(ctx, `
		INSERT INTO routing_rules (id, name, description, priority, enabled, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, rule.Id, rule.Name, rule.Description, rule.Priority, rule.Enabled, rule.CreatedBy, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert rule: %w", err)
	}

	// Insert conditions
	for i, cond := range rule.Conditions {
		condID := uuid.New().String()
		values, _ := json.Marshal(cond.StringList)

		_, err = tx.ExecContext(ctx, `
			INSERT INTO routing_conditions (id, rule_id, condition_type, field, operator, value, values, cel_expression, position, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, condID, rule.Id, cond.Type.String(), cond.Field, cond.Operator.String(), cond.StringValue, values, cond.CelExpression, i, now)
		if err != nil {
			return nil, fmt.Errorf("insert condition: %w", err)
		}
	}

	// Insert actions
	for i, action := range rule.Actions {
		actionID := uuid.New().String()
		params, _ := json.Marshal(action)

		_, err = tx.ExecContext(ctx, `
			INSERT INTO routing_actions (id, rule_id, action_type, parameters, position, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, actionID, rule.Id, action.Type.String(), params, i, now)
		if err != nil {
			return nil, fmt.Errorf("insert action: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return rule, nil
}

// GetRule retrieves a routing rule by ID.
func (s *PostgresStore) GetRule(ctx context.Context, id string) (*routingv1.RoutingRule, error) {
	rule := &routingv1.RoutingRule{}

	var createdAt, updatedAt time.Time
	var description sql.NullString
	var createdBy sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, priority, enabled, created_by, created_at, updated_at
		FROM routing_rules WHERE id = $1
	`, id).Scan(&rule.Id, &rule.Name, &description, &rule.Priority, &rule.Enabled, &createdBy, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query rule: %w", err)
	}

	rule.Description = description.String
	rule.CreatedBy = createdBy.String
	rule.CreatedAt = timestamppb.New(createdAt)
	rule.UpdatedAt = timestamppb.New(updatedAt)

	// Load conditions
	conditions, err := s.loadConditions(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load conditions: %w", err)
	}
	rule.Conditions = conditions

	// Load actions
	actions, err := s.loadActions(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load actions: %w", err)
	}
	rule.Actions = actions

	return rule, nil
}

// loadConditions loads conditions for a rule.
func (s *PostgresStore) loadConditions(ctx context.Context, ruleID string) ([]*routingv1.RoutingCondition, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT condition_type, field, operator, value, values, cel_expression
		FROM routing_conditions WHERE rule_id = $1 ORDER BY position
	`, ruleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conditions []*routingv1.RoutingCondition
	for rows.Next() {
		var condType, operator string
		var field, value, celExpr sql.NullString
		var valuesJSON []byte

		if err := rows.Scan(&condType, &field, &operator, &value, &valuesJSON, &celExpr); err != nil {
			return nil, err
		}

		cond := &routingv1.RoutingCondition{
			Type:          parseConditionType(condType),
			Field:         field.String,
			Operator:      parseConditionOperator(operator),
			StringValue:   value.String,
			CelExpression: celExpr.String,
		}

		if valuesJSON != nil {
			_ = json.Unmarshal(valuesJSON, &cond.StringList)
		}

		conditions = append(conditions, cond)
	}

	return conditions, rows.Err()
}

// loadActions loads actions for a rule.
func (s *PostgresStore) loadActions(ctx context.Context, ruleID string) ([]*routingv1.RoutingAction, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT action_type, parameters
		FROM routing_actions WHERE rule_id = $1 ORDER BY position
	`, ruleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []*routingv1.RoutingAction
	for rows.Next() {
		var actionType string
		var params []byte

		if err := rows.Scan(&actionType, &params); err != nil {
			return nil, err
		}

		action := &routingv1.RoutingAction{
			Type: parseActionType(actionType),
		}

		// Parse parameters back into the action fields
		if params != nil {
			_ = json.Unmarshal(params, action)
		}

		actions = append(actions, action)
	}

	return actions, rows.Err()
}

// ListRules retrieves routing rules with optional filters.
func (s *PostgresStore) ListRules(ctx context.Context, req *routingv1.ListRoutingRulesRequest) (*routingv1.ListRoutingRulesResponse, error) {
	query := `SELECT id, name, description, priority, enabled, created_by, created_at, updated_at FROM routing_rules WHERE 1=1`
	args := []interface{}{}
	argIndex := 1

	if req.EnabledOnly {
		query += fmt.Sprintf(" AND enabled = $%d", argIndex)
		args = append(args, true)
		argIndex++
	}

	if req.NameContains != "" {
		query += fmt.Sprintf(" AND name ILIKE $%d", argIndex)
		args = append(args, "%"+req.NameContains+"%")
		argIndex++
	}

	// Default ordering
	orderBy := "priority ASC"
	if req.OrderBy != "" {
		switch req.OrderBy {
		case "name":
			orderBy = "name ASC"
		case "created_at":
			orderBy = "created_at DESC"
		}
	}
	query += " ORDER BY " + orderBy

	// Pagination
	pageSize := int(req.PageSize)
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}
	query += fmt.Sprintf(" LIMIT $%d", argIndex)
	args = append(args, pageSize+1) // +1 to check if there are more results
	argIndex++

	if req.PageToken != "" {
		offset, _ := decodePageToken(req.PageToken)
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query rules: %w", err)
	}
	defer rows.Close()

	var rules []*routingv1.RoutingRule
	for rows.Next() {
		var rule routingv1.RoutingRule
		var createdAt, updatedAt time.Time
		var description, createdBy sql.NullString

		if err := rows.Scan(&rule.Id, &rule.Name, &description, &rule.Priority, &rule.Enabled, &createdBy, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan rule: %w", err)
		}

		rule.Description = description.String
		rule.CreatedBy = createdBy.String
		rule.CreatedAt = timestamppb.New(createdAt)
		rule.UpdatedAt = timestamppb.New(updatedAt)

		// Load conditions and actions
		conditions, err := s.loadConditions(ctx, rule.Id)
		if err != nil {
			return nil, err
		}
		rule.Conditions = conditions

		actions, err := s.loadActions(ctx, rule.Id)
		if err != nil {
			return nil, err
		}
		rule.Actions = actions

		rules = append(rules, &rule)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Handle pagination
	resp := &routingv1.ListRoutingRulesResponse{
		TotalCount: int32(len(rules)),
	}

	if len(rules) > pageSize {
		rules = rules[:pageSize]
		offset, _ := decodePageToken(req.PageToken)
		resp.NextPageToken = encodePageToken(offset + pageSize)
	}

	resp.Rules = rules
	return resp, nil
}

// UpdateRule updates an existing routing rule.
func (s *PostgresStore) UpdateRule(ctx context.Context, rule *routingv1.RoutingRule) (*routingv1.RoutingRule, error) {
	if rule == nil || rule.Id == "" {
		return nil, ErrInvalidRule
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()
	rule.UpdatedAt = timestamppb.New(now)

	// Update the rule
	result, err := tx.ExecContext(ctx, `
		UPDATE routing_rules SET name = $1, description = $2, priority = $3, enabled = $4, updated_at = $5
		WHERE id = $6
	`, rule.Name, rule.Description, rule.Priority, rule.Enabled, now, rule.Id)
	if err != nil {
		return nil, fmt.Errorf("update rule: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	// Delete existing conditions and actions
	_, err = tx.ExecContext(ctx, "DELETE FROM routing_conditions WHERE rule_id = $1", rule.Id)
	if err != nil {
		return nil, fmt.Errorf("delete conditions: %w", err)
	}

	_, err = tx.ExecContext(ctx, "DELETE FROM routing_actions WHERE rule_id = $1", rule.Id)
	if err != nil {
		return nil, fmt.Errorf("delete actions: %w", err)
	}

	// Re-insert conditions
	for i, cond := range rule.Conditions {
		condID := uuid.New().String()
		values, _ := json.Marshal(cond.StringList)

		_, err = tx.ExecContext(ctx, `
			INSERT INTO routing_conditions (id, rule_id, condition_type, field, operator, value, values, cel_expression, position, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, condID, rule.Id, cond.Type.String(), cond.Field, cond.Operator.String(), cond.StringValue, values, cond.CelExpression, i, now)
		if err != nil {
			return nil, fmt.Errorf("insert condition: %w", err)
		}
	}

	// Re-insert actions
	for i, action := range rule.Actions {
		actionID := uuid.New().String()
		params, _ := json.Marshal(action)

		_, err = tx.ExecContext(ctx, `
			INSERT INTO routing_actions (id, rule_id, action_type, parameters, position, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, actionID, rule.Id, action.Type.String(), params, i, now)
		if err != nil {
			return nil, fmt.Errorf("insert action: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return rule, nil
}

// DeleteRule deletes a routing rule by ID.
func (s *PostgresStore) DeleteRule(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM routing_rules WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete rule: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// ReorderRules updates the priorities of multiple rules.
func (s *PostgresStore) ReorderRules(ctx context.Context, priorities map[string]int32) ([]*routingv1.RoutingRule, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()
	var updatedRules []*routingv1.RoutingRule

	for id, priority := range priorities {
		_, err := tx.ExecContext(ctx, `
			UPDATE routing_rules SET priority = $1, updated_at = $2 WHERE id = $3
		`, priority, now, id)
		if err != nil {
			return nil, fmt.Errorf("update priority for %s: %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	// Fetch updated rules
	for id := range priorities {
		rule, err := s.GetRule(ctx, id)
		if err != nil {
			continue
		}
		updatedRules = append(updatedRules, rule)
	}

	return updatedRules, nil
}

// GetAuditLogs retrieves routing audit logs.
func (s *PostgresStore) GetAuditLogs(ctx context.Context, req *routingv1.GetRoutingAuditLogsRequest) (*routingv1.GetRoutingAuditLogsResponse, error) {
	query := `SELECT id, timestamp, alert_id, alert_fingerprint, evaluations, final_actions, processing_time_ms FROM routing_audit_logs WHERE 1=1`
	args := []interface{}{}
	argIndex := 1

	if req.AlertId != "" {
		query += fmt.Sprintf(" AND alert_id = $%d", argIndex)
		args = append(args, req.AlertId)
		argIndex++
	}

	if req.RuleId != "" {
		query += fmt.Sprintf(" AND evaluations @> '[{\"rule_id\": \"%s\"}]'", req.RuleId)
	}

	if req.StartTime != nil {
		query += fmt.Sprintf(" AND timestamp >= $%d", argIndex)
		args = append(args, req.StartTime.AsTime())
		argIndex++
	}

	if req.EndTime != nil {
		query += fmt.Sprintf(" AND timestamp <= $%d", argIndex)
		args = append(args, req.EndTime.AsTime())
		argIndex++
	}

	query += " ORDER BY timestamp DESC"

	pageSize := int(req.PageSize)
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}
	query += fmt.Sprintf(" LIMIT $%d", argIndex)
	args = append(args, pageSize+1)
	argIndex++

	if req.PageToken != "" {
		offset, _ := decodePageToken(req.PageToken)
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()

	var logs []*routingv1.RoutingAuditLog
	for rows.Next() {
		var log routingv1.RoutingAuditLog
		var timestamp time.Time
		var alertID, fingerprint sql.NullString
		var evaluationsJSON, actionsJSON []byte
		var processingTimeMs sql.NullInt64

		if err := rows.Scan(&log.Id, &timestamp, &alertID, &fingerprint, &evaluationsJSON, &actionsJSON, &processingTimeMs); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}

		log.AlertId = alertID.String
		log.Timestamp = timestamppb.New(timestamp)

		// Parse evaluations
		if evaluationsJSON != nil {
			var evals []map[string]interface{}
			if err := json.Unmarshal(evaluationsJSON, &evals); err == nil {
				for _, e := range evals {
					eval := &routingv1.RuleEvaluation{}
					if ruleID, ok := e["rule_id"].(string); ok {
						eval.RuleId = ruleID
					}
					if ruleName, ok := e["rule_name"].(string); ok {
						eval.RuleName = ruleName
					}
					if matched, ok := e["matched"].(bool); ok {
						eval.Matched = matched
					}
					log.Evaluations = append(log.Evaluations, eval)
				}
			}
		}

		// Parse actions
		if actionsJSON != nil {
			var acts []map[string]interface{}
			if err := json.Unmarshal(actionsJSON, &acts); err == nil {
				for _, a := range acts {
					exec := &routingv1.ActionExecution{}
					if ruleID, ok := a["rule_id"].(string); ok {
						exec.RuleId = ruleID
					}
					if success, ok := a["result"].(string); ok {
						exec.Success = success == "success"
					}
					if errMsg, ok := a["error"].(string); ok {
						exec.ErrorMessage = errMsg
					}
					log.Executions = append(log.Executions, exec)
				}
			}
		}

		logs = append(logs, &log)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	resp := &routingv1.GetRoutingAuditLogsResponse{
		TotalCount: int32(len(logs)),
	}

	if len(logs) > pageSize {
		logs = logs[:pageSize]
		offset, _ := decodePageToken(req.PageToken)
		resp.NextPageToken = encodePageToken(offset + pageSize)
	}

	resp.Logs = logs
	return resp, nil
}

// CreateAuditLog creates a new audit log entry.
func (s *PostgresStore) CreateAuditLog(ctx context.Context, log *routingv1.RoutingAuditLog) error {
	if log.Id == "" {
		log.Id = uuid.New().String()
	}

	evaluationsJSON, _ := json.Marshal(log.Evaluations)
	actionsJSON, _ := json.Marshal(log.Executions)

	var alertSnapshot []byte
	if log.AlertSnapshot != nil {
		alertSnapshot, _ = log.AlertSnapshot.MarshalJSON()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO routing_audit_logs (id, timestamp, alert_id, evaluations, final_actions)
		VALUES ($1, $2, $3, $4, $5)
	`, log.Id, log.Timestamp.AsTime(), log.AlertId, evaluationsJSON, actionsJSON)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}

	// Silence unused variable
	_ = alertSnapshot

	return nil
}

// GetEnabledRulesByPriority retrieves all enabled rules ordered by priority.
func (s *PostgresStore) GetEnabledRulesByPriority(ctx context.Context) ([]*routingv1.RoutingRule, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, priority, enabled, created_by, created_at, updated_at
		FROM routing_rules WHERE enabled = true ORDER BY priority ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query enabled rules: %w", err)
	}
	defer rows.Close()

	var rules []*routingv1.RoutingRule
	for rows.Next() {
		var rule routingv1.RoutingRule
		var createdAt, updatedAt time.Time
		var description, createdBy sql.NullString

		if err := rows.Scan(&rule.Id, &rule.Name, &description, &rule.Priority, &rule.Enabled, &createdBy, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan rule: %w", err)
		}

		rule.Description = description.String
		rule.CreatedBy = createdBy.String
		rule.CreatedAt = timestamppb.New(createdAt)
		rule.UpdatedAt = timestamppb.New(updatedAt)

		// Load conditions and actions
		conditions, err := s.loadConditions(ctx, rule.Id)
		if err != nil {
			return nil, err
		}
		rule.Conditions = conditions

		actions, err := s.loadActions(ctx, rule.Id)
		if err != nil {
			return nil, err
		}
		rule.Actions = actions

		rules = append(rules, &rule)
	}

	return rules, rows.Err()
}

// Helper functions for pagination
func encodePageToken(offset int) string {
	return fmt.Sprintf("%d", offset)
}

func decodePageToken(token string) (int, error) {
	var offset int
	_, err := fmt.Sscanf(token, "%d", &offset)
	return offset, err
}

// Helper functions to parse enum types from strings
func parseConditionType(s string) routingv1.ConditionType {
	if v, ok := routingv1.ConditionType_value[s]; ok {
		return routingv1.ConditionType(v)
	}
	return routingv1.ConditionType_CONDITION_TYPE_UNSPECIFIED
}

func parseConditionOperator(s string) routingv1.ConditionOperator {
	if v, ok := routingv1.ConditionOperator_value[s]; ok {
		return routingv1.ConditionOperator(v)
	}
	return routingv1.ConditionOperator_CONDITION_OPERATOR_UNSPECIFIED
}

func parseActionType(s string) routingv1.ActionType {
	if v, ok := routingv1.ActionType_value[s]; ok {
		return routingv1.ActionType(v)
	}
	return routingv1.ActionType_ACTION_TYPE_UNSPECIFIED
}

// InMemoryStore is an in-memory implementation of Store for testing.
type InMemoryStore struct {
	rules     map[string]*routingv1.RoutingRule
	auditLogs []*routingv1.RoutingAuditLog
	counter   int64
}

// NewInMemoryStore creates a new in-memory store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		rules:     make(map[string]*routingv1.RoutingRule),
		auditLogs: make([]*routingv1.RoutingAuditLog, 0),
	}
}

// CreateRule creates a new routing rule in memory.
func (s *InMemoryStore) CreateRule(ctx context.Context, rule *routingv1.RoutingRule) (*routingv1.RoutingRule, error) {
	if rule == nil {
		return nil, ErrInvalidRule
	}

	if rule.Id == "" {
		s.counter++
		rule.Id = fmt.Sprintf("rule-%d", s.counter)
	}

	now := time.Now()
	rule.CreatedAt = timestamppb.New(now)
	rule.UpdatedAt = timestamppb.New(now)

	// Check for duplicate priority
	for _, r := range s.rules {
		if r.Priority == rule.Priority {
			return nil, ErrDuplicatePriority
		}
	}

	s.rules[rule.Id] = rule
	return rule, nil
}

// GetRule retrieves a routing rule by ID.
func (s *InMemoryStore) GetRule(ctx context.Context, id string) (*routingv1.RoutingRule, error) {
	rule, ok := s.rules[id]
	if !ok {
		return nil, ErrNotFound
	}
	return rule, nil
}

// ListRules retrieves routing rules with optional filters.
func (s *InMemoryStore) ListRules(ctx context.Context, req *routingv1.ListRoutingRulesRequest) (*routingv1.ListRoutingRulesResponse, error) {
	var rules []*routingv1.RoutingRule

	for _, rule := range s.rules {
		if req.EnabledOnly && !rule.Enabled {
			continue
		}
		rules = append(rules, rule)
	}

	// Sort by priority
	for i := 0; i < len(rules)-1; i++ {
		for j := i + 1; j < len(rules); j++ {
			if rules[i].Priority > rules[j].Priority {
				rules[i], rules[j] = rules[j], rules[i]
			}
		}
	}

	return &routingv1.ListRoutingRulesResponse{
		Rules:      rules,
		TotalCount: int32(len(rules)),
	}, nil
}

// UpdateRule updates an existing routing rule.
func (s *InMemoryStore) UpdateRule(ctx context.Context, rule *routingv1.RoutingRule) (*routingv1.RoutingRule, error) {
	if rule == nil || rule.Id == "" {
		return nil, ErrInvalidRule
	}

	existing, ok := s.rules[rule.Id]
	if !ok {
		return nil, ErrNotFound
	}

	// Check for duplicate priority (excluding this rule)
	for _, r := range s.rules {
		if r.Id != rule.Id && r.Priority == rule.Priority {
			return nil, ErrDuplicatePriority
		}
	}

	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = timestamppb.Now()

	s.rules[rule.Id] = rule
	return rule, nil
}

// DeleteRule deletes a routing rule by ID.
func (s *InMemoryStore) DeleteRule(ctx context.Context, id string) error {
	if _, ok := s.rules[id]; !ok {
		return ErrNotFound
	}
	delete(s.rules, id)
	return nil
}

// ReorderRules updates the priorities of multiple rules.
func (s *InMemoryStore) ReorderRules(ctx context.Context, priorities map[string]int32) ([]*routingv1.RoutingRule, error) {
	var updatedRules []*routingv1.RoutingRule

	for id, priority := range priorities {
		rule, ok := s.rules[id]
		if !ok {
			continue
		}
		rule.Priority = priority
		rule.UpdatedAt = timestamppb.Now()
		updatedRules = append(updatedRules, rule)
	}

	return updatedRules, nil
}

// GetAuditLogs retrieves routing audit logs.
func (s *InMemoryStore) GetAuditLogs(ctx context.Context, req *routingv1.GetRoutingAuditLogsRequest) (*routingv1.GetRoutingAuditLogsResponse, error) {
	var logs []*routingv1.RoutingAuditLog

	for _, log := range s.auditLogs {
		if req.AlertId != "" && log.AlertId != req.AlertId {
			continue
		}
		if req.StartTime != nil && log.Timestamp.AsTime().Before(req.StartTime.AsTime()) {
			continue
		}
		if req.EndTime != nil && log.Timestamp.AsTime().After(req.EndTime.AsTime()) {
			continue
		}
		logs = append(logs, log)
	}

	return &routingv1.GetRoutingAuditLogsResponse{
		Logs:       logs,
		TotalCount: int32(len(logs)),
	}, nil
}

// CreateAuditLog creates a new audit log entry.
func (s *InMemoryStore) CreateAuditLog(ctx context.Context, log *routingv1.RoutingAuditLog) error {
	if log.Id == "" {
		log.Id = uuid.New().String()
	}
	s.auditLogs = append(s.auditLogs, log)
	return nil
}

// GetEnabledRulesByPriority retrieves all enabled rules ordered by priority.
func (s *InMemoryStore) GetEnabledRulesByPriority(ctx context.Context) ([]*routingv1.RoutingRule, error) {
	var rules []*routingv1.RoutingRule

	for _, rule := range s.rules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}

	// Sort by priority
	for i := 0; i < len(rules)-1; i++ {
		for j := i + 1; j < len(rules); j++ {
			if rules[i].Priority > rules[j].Priority {
				rules[i], rules[j] = rules[j], rules[i]
			}
		}
	}

	return rules, nil
}

// Ensure InMemoryStore satisfies the Store interface
var _ Store = (*InMemoryStore)(nil)

// Silence unused import warning for structpb
var _ = structpb.NewNullValue
