-- name: CreateRoutingRule :one
INSERT INTO routing_rules (
    name,
    description,
    priority,
    enabled,
    conditions,
    actions,
    terminal,
    time_condition,
    tags,
    created_by,
    updated_by
) VALUES (
    @name, @description, @priority, @enabled, @conditions, @actions, @terminal, @time_condition, @tags, @created_by, @created_by
)
RETURNING *;

-- name: GetRoutingRule :one
SELECT * FROM routing_rules
WHERE id = @id;

-- name: ListRoutingRules :many
SELECT * FROM routing_rules
WHERE
    (COALESCE(cardinality(@enabled_filter::boolean[]), 0) = 0 OR enabled = ANY(@enabled_filter::boolean[]))
    AND (COALESCE(cardinality(@tags_filter::text[]), 0) = 0 OR tags && @tags_filter::text[])
ORDER BY priority ASC, created_at ASC
LIMIT @lim OFFSET @off;

-- name: ListRoutingRulesCount :one
SELECT COUNT(*) FROM routing_rules
WHERE
    (COALESCE(cardinality(@enabled_filter::boolean[]), 0) = 0 OR enabled = ANY(@enabled_filter::boolean[]))
    AND (COALESCE(cardinality(@tags_filter::text[]), 0) = 0 OR tags && @tags_filter::text[]);

-- name: UpdateRoutingRule :one
UPDATE routing_rules
SET
    name = COALESCE(NULLIF(@name, ''), name),
    description = COALESCE(@description, description),
    priority = COALESCE(NULLIF(@priority::int, 0), priority),
    enabled = @enabled,
    conditions = COALESCE(@conditions, conditions),
    actions = COALESCE(@actions, actions),
    terminal = @terminal,
    time_condition = @time_condition,
    tags = COALESCE(@tags, tags),
    updated_by = @updated_by,
    updated_at = NOW()
WHERE id = @id
RETURNING *;

-- name: DeleteRoutingRule :exec
DELETE FROM routing_rules
WHERE id = @id;

-- name: UpdateRoutingRulePriority :exec
UPDATE routing_rules
SET priority = @priority, updated_at = NOW()
WHERE id = @id;

-- name: GetEnabledRulesByPriority :many
SELECT * FROM routing_rules
WHERE enabled = true
ORDER BY priority ASC, created_at ASC;

-- name: GetRoutingRulesByTags :many
SELECT * FROM routing_rules
WHERE tags && @tags::text[]
ORDER BY priority ASC, created_at ASC;
