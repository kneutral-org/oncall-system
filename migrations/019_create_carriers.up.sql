-- Migration: Create carriers table for BGP/carrier-specific routing
-- This migration establishes the carrier configuration for ISP/DC operations

-- Carriers table
-- Defines network carriers/peers with their ASNs and contact information
CREATE TABLE IF NOT EXISTS carriers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Human-readable carrier name (e.g., "Level3", "Cogent", "Lumen")
    name VARCHAR(255) NOT NULL UNIQUE,

    -- Autonomous System Number (must be unique)
    asn INTEGER NOT NULL UNIQUE,

    -- Type of carrier relationship: transit, peering, customer, provider, ixp
    carrier_type VARCHAR(50) NOT NULL DEFAULT 'peering',

    -- Priority for routing decisions (lower = more critical)
    priority INTEGER NOT NULL DEFAULT 5,

    -- Contact information as JSON array
    -- Example: [{"name": "NOC", "email": "noc@carrier.com", "phone": "+1-...", "role": "noc", "primary": true}]
    contacts JSONB NOT NULL DEFAULT '[]',

    -- Default escalation policy for this carrier
    escalation_policy_id UUID,

    -- NOC contact information for direct access
    noc_email VARCHAR(255),
    noc_phone VARCHAR(50),
    noc_portal_url TEXT,

    -- Internal team responsible for this carrier relationship
    team_id UUID,

    -- Auto-create ticket on BGP alerts from this carrier
    auto_ticket BOOLEAN NOT NULL DEFAULT false,
    ticket_provider_id UUID,

    -- Additional metadata as JSON
    metadata JSONB NOT NULL DEFAULT '{}',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT carriers_asn_positive CHECK (asn > 0 AND asn <= 4294967295),
    CONSTRAINT carriers_priority_valid CHECK (priority >= 0 AND priority <= 10),
    CONSTRAINT carriers_type_valid CHECK (carrier_type IN ('transit', 'peering', 'customer', 'provider', 'ixp'))
);

-- Index on ASN for fast lookups (primary lookup method for BGP alerts)
CREATE INDEX IF NOT EXISTS idx_carriers_asn ON carriers(asn);

-- Index on carrier type for filtering
CREATE INDEX IF NOT EXISTS idx_carriers_type ON carriers(carrier_type);

-- Index on priority for ordered queries
CREATE INDEX IF NOT EXISTS idx_carriers_priority ON carriers(priority);

-- Index on team_id for team-based queries
CREATE INDEX IF NOT EXISTS idx_carriers_team ON carriers(team_id) WHERE team_id IS NOT NULL;

-- Comments for documentation
COMMENT ON TABLE carriers IS
    'Network carriers/peers with ASN and contact information for BGP alert handling';

COMMENT ON COLUMN carriers.asn IS
    'Autonomous System Number (32-bit, 1-4294967295). Used to identify BGP peers in alerts.';

COMMENT ON COLUMN carriers.carrier_type IS
    'Type of carrier relationship: transit (upstream), peering (IX), customer (downstream), provider (upstream), ixp (internet exchange)';

COMMENT ON COLUMN carriers.priority IS
    'Priority for routing decisions. Lower values are more critical (0=critical, 5=normal, 10=low).';

COMMENT ON COLUMN carriers.contacts IS
    'JSON array of contact objects with name, email, phone, role, and primary flag';

COMMENT ON COLUMN carriers.auto_ticket IS
    'If true, automatically create a ticket when BGP alerts are received for this carrier';

COMMENT ON COLUMN carriers.metadata IS
    'Additional carrier-specific metadata as JSON object';
