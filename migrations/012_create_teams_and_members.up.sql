-- Migration: Create teams and team_members tables for on-call team management
-- This migration establishes the foundation for team-based routing and escalation

-- Teams table
-- Represents organizational units that can be assigned to schedules and routing rules
CREATE TABLE IF NOT EXISTS teams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Team name must be unique across the organization
    name VARCHAR(255) NOT NULL UNIQUE,

    -- Optional description for the team
    description TEXT,

    -- Default escalation policy for alerts routed to this team
    -- FK will be added later when escalation_policies table exists
    default_escalation_policy_id UUID,

    -- Default notification channel for team communications
    -- FK will be added later when notification_channels table exists
    default_notification_channel_id UUID,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Team members table
-- Associates users with teams and defines their role within the team
CREATE TABLE IF NOT EXISTS team_members (
    -- Foreign key to the teams table
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,

    -- User ID references users in kneutral-api (external reference)
    user_id UUID NOT NULL,

    -- Role within the team: admin, member, observer
    role VARCHAR(50) NOT NULL DEFAULT 'member',

    -- When the user joined the team
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (team_id, user_id)
);

-- Index for efficient lookup of teams by user
CREATE INDEX IF NOT EXISTS idx_team_members_user ON team_members(user_id);

-- Comments for documentation
COMMENT ON TABLE teams IS
    'Organizational units for on-call rotation and alert routing';

COMMENT ON COLUMN teams.name IS
    'Unique team name visible in the UI and API';

COMMENT ON COLUMN teams.default_escalation_policy_id IS
    'Default escalation policy applied when alerts are routed to this team';

COMMENT ON COLUMN teams.default_notification_channel_id IS
    'Default channel for team-wide notifications';

COMMENT ON TABLE team_members IS
    'Association between teams and users with role information';

COMMENT ON COLUMN team_members.role IS
    'Member role: admin (manage team), member (participate in rotations), observer (view only)';
