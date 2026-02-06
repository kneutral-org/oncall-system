-- Migration: Create routing_rules, routing_conditions, and routing_actions tables
-- This migration establishes the alert routing engine's rule system

-- Routing rules table
-- Defines named rules with priority-based evaluation order
CREATE TABLE IF NOT EXISTS routing_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Human-readable rule name
    name VARCHAR(255) NOT NULL,

    -- Optional description explaining the rule's purpose
    description TEXT,

    -- Evaluation priority (lower = evaluated first, unique to ensure deterministic ordering)
    priority INTEGER NOT NULL,

    -- Whether the rule is active
    enabled BOOLEAN NOT NULL DEFAULT true,

    -- User who created the rule
    created_by UUID,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Ensure unique priority for deterministic rule ordering
    UNIQUE(priority)
);

-- Routing conditions table
-- Defines conditions that must be met for a rule to match
CREATE TABLE IF NOT EXISTS routing_conditions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Parent rule
    rule_id UUID NOT NULL REFERENCES routing_rules(id) ON DELETE CASCADE,

    -- Type of condition: label, severity, site, time, cel
    condition_type VARCHAR(50) NOT NULL,

    -- Field to evaluate (e.g., "labels.team", "severity", "source")
    field VARCHAR(255),

    -- Comparison operator: equals, not_equals, contains, matches, in, not_in, exists
    operator VARCHAR(50) NOT NULL,

    -- Single value for comparison (used with equals, not_equals, contains, matches)
    value TEXT,

    -- Multiple values for comparison (used with in, not_in operators)
    values TEXT[],

    -- CEL expression for complex conditions (used when condition_type = 'cel')
    cel_expression TEXT,

    -- Order of evaluation within the rule (for AND/OR logic)
    position INTEGER NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for efficient lookup of conditions by rule
CREATE INDEX IF NOT EXISTS idx_conditions_rule ON routing_conditions(rule_id);

-- Routing actions table
-- Defines actions to execute when a rule matches
CREATE TABLE IF NOT EXISTS routing_actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Parent rule
    rule_id UUID NOT NULL REFERENCES routing_rules(id) ON DELETE CASCADE,

    -- Type of action: route_to_team, set_priority, add_label, notify, suppress
    action_type VARCHAR(50) NOT NULL,

    -- Action-specific parameters as JSON
    -- Examples:
    -- route_to_team: {"team_id": "uuid", "escalation_policy_id": "uuid"}
    -- set_priority: {"priority": "critical"}
    -- add_label: {"key": "routed", "value": "true"}
    -- notify: {"channel_id": "uuid", "template": "alert_notification"}
    -- suppress: {"duration_minutes": 30, "reason": "Known issue"}
    parameters JSONB NOT NULL DEFAULT '{}',

    -- Order of action execution within the rule
    position INTEGER NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for efficient lookup of actions by rule
CREATE INDEX IF NOT EXISTS idx_actions_rule ON routing_actions(rule_id);

-- Comments for documentation
COMMENT ON TABLE routing_rules IS
    'Named routing rules evaluated in priority order against incoming alerts';

COMMENT ON COLUMN routing_rules.priority IS
    'Evaluation order: lower values are evaluated first. Must be unique.';

COMMENT ON COLUMN routing_rules.enabled IS
    'Whether this rule is active. Disabled rules are skipped during evaluation.';

COMMENT ON TABLE routing_conditions IS
    'Conditions that must be satisfied for a routing rule to match an alert';

COMMENT ON COLUMN routing_conditions.condition_type IS
    'Type of condition: label (match labels), severity (match severity level), site (match site), time (time-based), cel (CEL expression)';

COMMENT ON COLUMN routing_conditions.operator IS
    'Comparison operator: equals, not_equals, contains, matches (regex), in, not_in, exists';

COMMENT ON COLUMN routing_conditions.cel_expression IS
    'Common Expression Language expression for complex conditions (only when condition_type = cel)';

COMMENT ON TABLE routing_actions IS
    'Actions executed when a routing rule matches an alert';

COMMENT ON COLUMN routing_actions.action_type IS
    'Type of action: route_to_team, set_priority, add_label, notify, suppress';

COMMENT ON COLUMN routing_actions.parameters IS
    'JSON parameters specific to the action type';
