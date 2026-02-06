-- Create equipment_types table for managing equipment type definitions and routing rules
CREATE TABLE IF NOT EXISTS equipment_types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    category VARCHAR(50) NOT NULL CHECK (category IN ('network', 'compute', 'storage', 'security')),
    vendor VARCHAR(100),
    criticality INT NOT NULL DEFAULT 3 CHECK (criticality >= 1 AND criticality <= 5),
    default_team_id UUID REFERENCES teams(id) ON DELETE SET NULL,
    escalation_policy VARCHAR(255),
    routing_rules JSONB DEFAULT '[]'::jsonb,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create indexes for common queries
CREATE INDEX idx_equipment_types_category ON equipment_types(category);
CREATE INDEX idx_equipment_types_vendor ON equipment_types(vendor);
CREATE INDEX idx_equipment_types_criticality ON equipment_types(criticality);
CREATE INDEX idx_equipment_types_name_lower ON equipment_types(LOWER(name));

-- Create trigger for updating updated_at timestamp
CREATE OR REPLACE FUNCTION update_equipment_types_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_equipment_types_updated_at
    BEFORE UPDATE ON equipment_types
    FOR EACH ROW
    EXECUTE FUNCTION update_equipment_types_updated_at();

-- Seed data for common equipment types
INSERT INTO equipment_types (name, category, criticality, metadata) VALUES
    ('router', 'network', 5, '{"description": "Network router - core infrastructure"}'),
    ('core_router', 'network', 5, '{"description": "Core network router"}'),
    ('edge_router', 'network', 4, '{"description": "Edge network router"}'),
    ('switch', 'network', 4, '{"description": "Network switch"}'),
    ('core_switch', 'network', 5, '{"description": "Core network switch"}'),
    ('access_switch', 'network', 3, '{"description": "Access layer network switch"}'),
    ('firewall', 'security', 5, '{"description": "Network firewall"}'),
    ('load_balancer', 'network', 5, '{"description": "Load balancer"}'),
    ('server', 'compute', 3, '{"description": "General purpose server"}'),
    ('database_server', 'compute', 5, '{"description": "Database server"}'),
    ('web_server', 'compute', 3, '{"description": "Web server"}'),
    ('storage', 'storage', 4, '{"description": "Storage system"}'),
    ('nas', 'storage', 4, '{"description": "Network Attached Storage"}'),
    ('san', 'storage', 5, '{"description": "Storage Area Network"}'),
    ('access_point', 'network', 2, '{"description": "Wireless access point"}'),
    ('pdu', 'compute', 4, '{"description": "Power Distribution Unit"}'),
    ('ups', 'compute', 5, '{"description": "Uninterruptible Power Supply"}'),
    ('vpn_gateway', 'security', 4, '{"description": "VPN Gateway"}'),
    ('dns_server', 'network', 5, '{"description": "DNS Server"}'),
    ('ntp_server', 'network', 4, '{"description": "NTP Server"}')
ON CONFLICT (name) DO NOTHING;

-- Add comment for documentation
COMMENT ON TABLE equipment_types IS 'Equipment type definitions for alert routing based on device type';
COMMENT ON COLUMN equipment_types.criticality IS 'Criticality level from 1 (lowest) to 5 (highest)';
COMMENT ON COLUMN equipment_types.routing_rules IS 'Array of routing rule IDs specific to this equipment type';
