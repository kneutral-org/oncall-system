-- Migration: Create routing_audit_logs table for routing decision audit trail
-- This table stores a complete record of routing decisions for debugging and compliance

CREATE TABLE IF NOT EXISTS routing_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- When the routing decision was made
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- The alert that was routed
    alert_id UUID NOT NULL,

    -- Alert fingerprint for correlation with external systems
    alert_fingerprint VARCHAR(255),

    -- Array of rule evaluation results
    -- Format: [{"rule_id": "uuid", "rule_name": "string", "matched": bool, "conditions_evaluated": [...]}]
    evaluations JSONB NOT NULL DEFAULT '[]',

    -- Actions that were actually executed
    -- Format: [{"action_type": "string", "parameters": {...}, "result": "success|failed", "error": "..."}]
    final_actions JSONB NOT NULL DEFAULT '[]',

    -- Time taken to process the routing decision in milliseconds
    processing_time_ms INTEGER,

    -- Version of the routing engine for debugging
    routing_engine_version VARCHAR(50)
);

-- Index for finding audit logs by alert
CREATE INDEX IF NOT EXISTS idx_audit_alert ON routing_audit_logs(alert_id);

-- Index for finding audit logs by fingerprint (for correlation)
CREATE INDEX IF NOT EXISTS idx_audit_fingerprint ON routing_audit_logs(alert_fingerprint);

-- Index for time-based queries and retention management
CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON routing_audit_logs(timestamp);

-- Comments for documentation
COMMENT ON TABLE routing_audit_logs IS
    'Audit trail of routing decisions for debugging, analytics, and compliance';

COMMENT ON COLUMN routing_audit_logs.alert_fingerprint IS
    'Unique identifier for the alert instance, used for correlation across systems';

COMMENT ON COLUMN routing_audit_logs.evaluations IS
    'JSON array of rule evaluations showing which rules matched and why';

COMMENT ON COLUMN routing_audit_logs.final_actions IS
    'JSON array of actions that were executed and their results';

COMMENT ON COLUMN routing_audit_logs.processing_time_ms IS
    'Total time to evaluate all rules and execute actions, in milliseconds';

COMMENT ON COLUMN routing_audit_logs.routing_engine_version IS
    'Version of the routing engine for debugging version-specific issues';
