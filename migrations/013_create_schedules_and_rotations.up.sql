-- Migration: Create schedules, rotations, rotation_members, and schedule_overrides tables
-- This migration establishes the on-call scheduling system

-- Schedules table
-- Represents a named on-call schedule that can contain multiple rotations
CREATE TABLE IF NOT EXISTS schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Schedule name for display
    name VARCHAR(255) NOT NULL,

    -- Optional description
    description TEXT,

    -- Timezone for interpreting time-based settings (IANA timezone)
    timezone VARCHAR(100) NOT NULL DEFAULT 'UTC',

    -- Optional team association
    team_id UUID REFERENCES teams(id),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for finding schedules by team
CREATE INDEX IF NOT EXISTS idx_schedules_team ON schedules(team_id);

-- Rotations table
-- Defines rotation patterns within a schedule
CREATE TABLE IF NOT EXISTS rotations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Parent schedule
    schedule_id UUID NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,

    -- Optional name for the rotation (e.g., "Primary", "Secondary")
    name VARCHAR(255),

    -- Priority for overlapping rotations (higher = more important)
    priority INTEGER NOT NULL DEFAULT 0,

    -- Type of rotation: daily, weekly, custom
    rotation_type VARCHAR(50) NOT NULL,

    -- When the rotation pattern starts
    start_time TIMESTAMPTZ NOT NULL,

    -- Length of each shift in hours (for daily/custom rotations)
    shift_length_hours INTEGER,

    -- Time of day when handoff occurs (for weekly rotations)
    handoff_time TIME,

    -- Day of week for handoff (0=Sunday, 6=Saturday) for weekly rotations
    handoff_day INTEGER,

    -- Time restriction: start time (e.g., 09:00 for business hours only)
    time_restriction_start TIME,

    -- Time restriction: end time (e.g., 17:00 for business hours only)
    time_restriction_end TIME,

    -- Days of week when rotation is active (array of 0-6)
    time_restriction_days INTEGER[],

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for finding rotations by schedule
CREATE INDEX IF NOT EXISTS idx_rotations_schedule ON rotations(schedule_id);

-- Rotation members table
-- Associates users with rotations and defines their order in the rotation
CREATE TABLE IF NOT EXISTS rotation_members (
    -- Parent rotation
    rotation_id UUID NOT NULL REFERENCES rotations(id) ON DELETE CASCADE,

    -- User participating in this rotation (external reference to kneutral-api)
    user_id UUID NOT NULL,

    -- Position in the rotation order (0-indexed)
    position INTEGER NOT NULL,

    PRIMARY KEY (rotation_id, user_id)
);

-- Schedule overrides table
-- Allows temporary assignment of on-call responsibility to specific users
CREATE TABLE IF NOT EXISTS schedule_overrides (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Schedule being overridden
    schedule_id UUID NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,

    -- User taking over on-call during this period
    user_id UUID NOT NULL,

    -- Override period start
    start_time TIMESTAMPTZ NOT NULL,

    -- Override period end
    end_time TIMESTAMPTZ NOT NULL,

    -- Optional reason for the override (e.g., "Covering for vacation")
    reason VARCHAR(255),

    -- User who created the override
    created_by UUID,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for efficient lookup of active overrides
CREATE INDEX IF NOT EXISTS idx_overrides_schedule_time ON schedule_overrides(schedule_id, start_time, end_time);

-- Comments for documentation
COMMENT ON TABLE schedules IS
    'On-call schedules that can contain multiple rotation patterns';

COMMENT ON COLUMN schedules.timezone IS
    'IANA timezone (e.g., America/New_York) for interpreting schedule times';

COMMENT ON TABLE rotations IS
    'Rotation patterns within a schedule defining shift handoffs';

COMMENT ON COLUMN rotations.rotation_type IS
    'Type of rotation: daily (shifts within a day), weekly (week-long shifts), custom (variable shifts)';

COMMENT ON COLUMN rotations.priority IS
    'Higher priority rotations take precedence when multiple rotations overlap';

COMMENT ON COLUMN rotations.time_restriction_days IS
    'Array of day numbers (0=Sunday, 6=Saturday) when this rotation is active';

COMMENT ON TABLE rotation_members IS
    'Users participating in a rotation with their position in the order';

COMMENT ON TABLE schedule_overrides IS
    'Temporary overrides that assign on-call to a specific user for a time period';
