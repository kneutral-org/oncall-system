-- Migration: Create maintenance_windows table for planned maintenance periods
-- Maintenance windows allow suppressing or annotating alerts during scheduled work

CREATE TABLE IF NOT EXISTS maintenance_windows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Human-readable name for the maintenance window
    name VARCHAR(255) NOT NULL,

    -- Optional description of the maintenance work
    description TEXT,

    -- Scheduled start time of the maintenance window
    start_time TIMESTAMPTZ NOT NULL,

    -- Scheduled end time of the maintenance window
    end_time TIMESTAMPTZ NOT NULL,

    -- Current status: scheduled, active, completed, cancelled
    status VARCHAR(50) NOT NULL DEFAULT 'scheduled',

    -- Action to take during maintenance: suppress, annotate, route_to_team
    action VARCHAR(50) NOT NULL DEFAULT 'annotate',

    -- Scope of the maintenance window (which alerts are affected)
    -- Format: {"sites": ["site-id-1"], "services": ["svc-1"], "labels": {"env": "prod"}}
    scope JSONB NOT NULL DEFAULT '{}',

    -- External ticket reference (e.g., JIRA, ServiceNow)
    ticket_id VARCHAR(255),

    -- URL to the external ticket
    ticket_url TEXT,

    -- User who created the maintenance window
    created_by UUID,

    -- User who approved the maintenance window (for change management)
    approved_by UUID,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Ensure status is one of the valid values
    CONSTRAINT valid_status CHECK (status IN ('scheduled', 'active', 'completed', 'cancelled')),

    -- Ensure action is one of the valid values
    CONSTRAINT valid_action CHECK (action IN ('suppress', 'annotate', 'route_to_team'))
);

-- Index for filtering by status
CREATE INDEX IF NOT EXISTS idx_maint_status ON maintenance_windows(status);

-- Partial index for efficient lookup of scheduled/active windows by time
CREATE INDEX IF NOT EXISTS idx_maint_time ON maintenance_windows(start_time, end_time)
    WHERE status IN ('scheduled', 'active');

-- GIN index for scope queries
CREATE INDEX IF NOT EXISTS idx_maint_scope ON maintenance_windows USING GIN(scope);

-- Comments for documentation
COMMENT ON TABLE maintenance_windows IS
    'Scheduled maintenance periods that affect alert handling';

COMMENT ON COLUMN maintenance_windows.status IS
    'Window status: scheduled (future), active (in progress), completed (past), cancelled (aborted)';

COMMENT ON COLUMN maintenance_windows.action IS
    'Action during maintenance: suppress (silence alerts), annotate (add maintenance label), route_to_team (send to specific team)';

COMMENT ON COLUMN maintenance_windows.scope IS
    'JSON defining which alerts are affected: sites, services, and/or label matchers';

COMMENT ON COLUMN maintenance_windows.ticket_id IS
    'External ticket/change reference (e.g., JIRA-1234, CHG0012345)';

COMMENT ON COLUMN maintenance_windows.approved_by IS
    'User who approved the maintenance for change management compliance';
