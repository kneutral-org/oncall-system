-- Migration: Create customer_tiers and customers tables
-- This migration establishes the customer tier system for SLA-based routing

-- Customer Tiers table
-- Defines service levels with SLA response times and routing configuration
CREATE TABLE IF NOT EXISTS customer_tiers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Tier name (e.g., "Enterprise", "Premium", "Standard")
    name VARCHAR(255) NOT NULL UNIQUE,

    -- Priority level (1 = highest priority)
    level INTEGER NOT NULL UNIQUE,

    -- Optional description
    description TEXT,

    -- SLA response times in milliseconds
    critical_response_ms BIGINT NOT NULL DEFAULT 900000,    -- 15 minutes
    high_response_ms BIGINT NOT NULL DEFAULT 3600000,       -- 1 hour
    medium_response_ms BIGINT NOT NULL DEFAULT 14400000,    -- 4 hours
    low_response_ms BIGINT NOT NULL DEFAULT 86400000,       -- 24 hours

    -- Routing configuration
    -- Escalation multiplier (1.0 = normal, 0.5 = 2x faster escalation)
    escalation_multiplier DECIMAL(4,2) NOT NULL DEFAULT 1.0,

    -- Boost severity by this amount (0 = no boost)
    severity_boost INTEGER NOT NULL DEFAULT 0,

    -- Optional dedicated team for this tier
    dedicated_team_id UUID REFERENCES teams(id) ON DELETE SET NULL,

    -- Metadata for extensibility
    metadata JSONB NOT NULL DEFAULT '{}',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for level lookups
CREATE INDEX IF NOT EXISTS idx_customer_tiers_level ON customer_tiers(level);

-- Customers table
-- Defines customers with tier assignments and lookup methods
CREATE TABLE IF NOT EXISTS customers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Customer name
    name VARCHAR(255) NOT NULL,

    -- Unique account identifier for lookup
    account_id VARCHAR(255) NOT NULL UNIQUE,

    -- Assigned tier
    tier_id UUID NOT NULL REFERENCES customer_tiers(id) ON DELETE RESTRICT,

    -- Optional description
    description TEXT,

    -- Domains for lookup (e.g., ["example.com", "sub.example.com"])
    domains JSONB NOT NULL DEFAULT '[]',

    -- IP ranges in CIDR notation for lookup (e.g., ["10.0.0.0/8", "192.168.1.0/24"])
    ip_ranges JSONB NOT NULL DEFAULT '[]',

    -- Contact information
    contacts JSONB NOT NULL DEFAULT '[]',

    -- Metadata for extensibility
    metadata JSONB NOT NULL DEFAULT '{}',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for various lookup patterns
CREATE INDEX IF NOT EXISTS idx_customers_tier ON customers(tier_id);
CREATE INDEX IF NOT EXISTS idx_customers_domains ON customers USING GIN (domains);
CREATE INDEX IF NOT EXISTS idx_customers_ip_ranges ON customers USING GIN (ip_ranges);

-- Insert default tiers
INSERT INTO customer_tiers (id, name, level, description, critical_response_ms, high_response_ms, medium_response_ms, low_response_ms, escalation_multiplier, severity_boost)
VALUES
    (gen_random_uuid(), 'Enterprise', 1, 'Enterprise tier with fastest SLA', 300000, 1800000, 7200000, 28800000, 0.5, 1),
    (gen_random_uuid(), 'Premium', 2, 'Premium tier with enhanced SLA', 600000, 3600000, 14400000, 57600000, 0.75, 0),
    (gen_random_uuid(), 'Standard', 3, 'Standard tier with default SLA', 900000, 7200000, 28800000, 86400000, 1.0, 0),
    (gen_random_uuid(), 'Basic', 4, 'Basic tier with relaxed SLA', 1800000, 14400000, 43200000, 172800000, 1.5, 0)
ON CONFLICT (name) DO NOTHING;

-- Comments for documentation
COMMENT ON TABLE customer_tiers IS
    'Customer service tiers with SLA configuration and routing adjustments';

COMMENT ON COLUMN customer_tiers.level IS
    'Priority level: 1 = highest priority, higher numbers = lower priority';

COMMENT ON COLUMN customer_tiers.escalation_multiplier IS
    'Multiplier for escalation delays: 0.5 = 2x faster, 1.0 = normal, 2.0 = 2x slower';

COMMENT ON COLUMN customer_tiers.severity_boost IS
    'Amount to boost alert severity: 0 = no change, 1 = boost by one level';

COMMENT ON TABLE customers IS
    'Customer records with tier assignments and lookup identifiers';

COMMENT ON COLUMN customers.account_id IS
    'Unique account identifier used for customer lookup from alert labels';

COMMENT ON COLUMN customers.domains IS
    'JSON array of domains for domain-based customer lookup';

COMMENT ON COLUMN customers.ip_ranges IS
    'JSON array of CIDR ranges for IP-based customer lookup';

COMMENT ON COLUMN customers.contacts IS
    'JSON array of contact objects: {name, email, phone, role, primary}';
