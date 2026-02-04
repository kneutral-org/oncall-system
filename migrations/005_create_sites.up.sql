-- Create sites table for datacenter/POP management
CREATE TABLE sites (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    code VARCHAR(50) NOT NULL UNIQUE,
    site_type VARCHAR(50) NOT NULL DEFAULT 'datacenter',
    region VARCHAR(100),
    country VARCHAR(100),
    city VARCHAR(100),
    timezone VARCHAR(64) NOT NULL DEFAULT 'UTC',
    tier INT NOT NULL DEFAULT 3,
    primary_team_id UUID REFERENCES teams(id) ON DELETE SET NULL,
    secondary_team_id UUID REFERENCES teams(id) ON DELETE SET NULL,
    escalation_policy_id UUID REFERENCES escalation_policies(id) ON DELETE SET NULL,
    business_hours JSONB,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create indexes for common queries
CREATE INDEX idx_sites_code ON sites(code);
CREATE INDEX idx_sites_type ON sites(site_type);
CREATE INDEX idx_sites_region ON sites(region);
CREATE INDEX idx_sites_country ON sites(country);
CREATE INDEX idx_sites_tier ON sites(tier);

-- Add trigger for updated_at
CREATE TRIGGER sites_updated_at
    BEFORE UPDATE ON sites
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_updated_at();

-- Create function if it doesn't exist
CREATE OR REPLACE FUNCTION trigger_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
