-- name: CreateRoutingRule :one
INSERT INTO routing_rules (id, name, description, priority, enabled, created_by, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetRoutingRule :one
SELECT id, name, description, priority, enabled, created_by, created_at, updated_at
FROM routing_rules
WHERE id = $1;

-- name: ListRoutingRules :many
SELECT id, name, description, priority, enabled, created_by, created_at, updated_at
FROM routing_rules
ORDER BY priority ASC
LIMIT $1 OFFSET $2;

-- name: ListEnabledRoutingRules :many
SELECT id, name, description, priority, enabled, created_by, created_at, updated_at
FROM routing_rules
WHERE enabled = true
ORDER BY priority ASC;

-- name: ListRoutingRulesByName :many
SELECT id, name, description, priority, enabled, created_by, created_at, updated_at
FROM routing_rules
WHERE name ILIKE '%' || $1 || '%'
ORDER BY priority ASC
LIMIT $2 OFFSET $3;

-- name: UpdateRoutingRule :one
UPDATE routing_rules
SET name = $2, description = $3, priority = $4, enabled = $5, updated_at = $6
WHERE id = $1
RETURNING *;

-- name: UpdateRoutingRulePriority :exec
UPDATE routing_rules
SET priority = $2, updated_at = $3
WHERE id = $1;

-- name: DeleteRoutingRule :exec
DELETE FROM routing_rules
WHERE id = $1;

-- name: CountRoutingRules :one
SELECT COUNT(*) FROM routing_rules;

-- name: CountEnabledRoutingRules :one
SELECT COUNT(*) FROM routing_rules WHERE enabled = true;

-- name: CreateRoutingCondition :one
INSERT INTO routing_conditions (id, rule_id, condition_type, field, operator, value, values, cel_expression, position, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetRoutingConditions :many
SELECT id, rule_id, condition_type, field, operator, value, values, cel_expression, position, created_at
FROM routing_conditions
WHERE rule_id = $1
ORDER BY position ASC;

-- name: DeleteRoutingConditions :exec
DELETE FROM routing_conditions
WHERE rule_id = $1;

-- name: CreateRoutingAction :one
INSERT INTO routing_actions (id, rule_id, action_type, parameters, position, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetRoutingActions :many
SELECT id, rule_id, action_type, parameters, position, created_at
FROM routing_actions
WHERE rule_id = $1
ORDER BY position ASC;

-- name: DeleteRoutingActions :exec
DELETE FROM routing_actions
WHERE rule_id = $1;

-- name: CreateRoutingAuditLog :one
INSERT INTO routing_audit_logs (id, timestamp, alert_id, alert_fingerprint, evaluations, final_actions, processing_time_ms, routing_engine_version)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetRoutingAuditLogs :many
SELECT id, timestamp, alert_id, alert_fingerprint, evaluations, final_actions, processing_time_ms, routing_engine_version
FROM routing_audit_logs
WHERE ($1::uuid IS NULL OR alert_id = $1)
  AND ($2::timestamptz IS NULL OR timestamp >= $2)
  AND ($3::timestamptz IS NULL OR timestamp <= $3)
ORDER BY timestamp DESC
LIMIT $4 OFFSET $5;

-- name: GetRoutingAuditLogsByAlertID :many
SELECT id, timestamp, alert_id, alert_fingerprint, evaluations, final_actions, processing_time_ms, routing_engine_version
FROM routing_audit_logs
WHERE alert_id = $1
ORDER BY timestamp DESC
LIMIT $2 OFFSET $3;

-- name: GetRoutingAuditLogsByFingerprint :many
SELECT id, timestamp, alert_id, alert_fingerprint, evaluations, final_actions, processing_time_ms, routing_engine_version
FROM routing_audit_logs
WHERE alert_fingerprint = $1
ORDER BY timestamp DESC
LIMIT $2 OFFSET $3;

-- name: CountRoutingAuditLogs :one
SELECT COUNT(*) FROM routing_audit_logs;

-- name: DeleteOldRoutingAuditLogs :exec
DELETE FROM routing_audit_logs
WHERE timestamp < $1;
