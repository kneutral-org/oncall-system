-- Migration: Create sites table for location-based routing
-- Sites represent physical or logical locations that can have associated teams and routing rules

CREATE TABLE IF NOT EXISTS sites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Human-readable site name
    name VARCHAR(255) NOT NULL,

    -- Unique site code for programmatic reference (e.g., "NYC-DC1", "LON-POP2")
    code VARCHAR(50) NOT NULL UNIQUE,

    -- Type of site: datacenter, pop, hub, customer_premise
    site_type VARCHAR(50) NOT NULL,

    -- Datacenter tier (1-4) per TIA-942 standard, applicable to datacenters
    tier INTEGER,

    -- Geographic information
    region VARCHAR(100),
    country VARCHAR(100),
    city VARCHAR(100),
    address TEXT,

    -- Timezone for the site (IANA format)
    timezone VARCHAR(100) NOT NULL DEFAULT 'UTC',

    -- Primary team responsible for this site
    primary_team_id UUID REFERENCES teams(id),

    -- Secondary/backup team for escalation
    secondary_team_id UUID REFERENCES teams(id),

    -- Default escalation policy for alerts at this site
    default_escalation_policy_id UUID,

    -- Parent site for hierarchical relationships (e.g., PoP belongs to a region)
    parent_site_id UUID REFERENCES sites(id),

    -- Flexible labels for routing and grouping (JSONB for GIN indexing)
    labels JSONB NOT NULL DEFAULT '{}',

    -- Business hours configuration
    -- Format: {"start": "09:00", "end": "17:00", "days": [1,2,3,4,5]}
    business_hours JSONB,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for unique code lookups
CREATE INDEX IF NOT EXISTS idx_sites_code ON sites(code);

-- Index for filtering by site type
CREATE INDEX IF NOT EXISTS idx_sites_type ON sites(site_type);

-- Index for regional queries
CREATE INDEX IF NOT EXISTS idx_sites_region ON sites(region);

-- GIN index for efficient label queries
CREATE INDEX IF NOT EXISTS idx_sites_labels ON sites USING GIN(labels);

-- Comments for documentation
COMMENT ON TABLE sites IS
    'Physical or logical locations for routing and team assignment';

COMMENT ON COLUMN sites.code IS
    'Unique identifier code used in alerts and routing rules (e.g., NYC-DC1)';

COMMENT ON COLUMN sites.site_type IS
    'Type of site: datacenter (full DC), pop (point of presence), hub (aggregation point), customer_premise (customer location)';

COMMENT ON COLUMN sites.tier IS
    'Datacenter tier per TIA-942 (1-4): 1=basic, 2=redundant components, 3=concurrently maintainable, 4=fault tolerant';

COMMENT ON COLUMN sites.labels IS
    'Key-value labels for flexible categorization and routing rule matching';

COMMENT ON COLUMN sites.business_hours IS
    'JSON object defining business hours: start/end times and active days (0=Sunday)';

COMMENT ON COLUMN sites.parent_site_id IS
    'Reference to parent site for hierarchical organization (e.g., PoP within a region)';
