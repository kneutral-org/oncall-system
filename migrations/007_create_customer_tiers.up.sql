-- Create customer_tiers table
CREATE TABLE customer_tiers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL UNIQUE,
    level INT NOT NULL UNIQUE,
    critical_response_minutes INT NOT NULL DEFAULT 15,
    high_response_minutes INT NOT NULL DEFAULT 30,
    medium_response_minutes INT NOT NULL DEFAULT 60,
    escalation_multiplier FLOAT NOT NULL DEFAULT 1.0,
    dedicated_team_id UUID REFERENCES teams(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create index for level lookups
CREATE INDEX idx_customer_tiers_level ON customer_tiers(level);
CREATE INDEX idx_customer_tiers_name ON customer_tiers(name);

-- Insert default tiers
INSERT INTO customer_tiers (name, level, critical_response_minutes, high_response_minutes, medium_response_minutes, escalation_multiplier, metadata)
VALUES
    ('Platinum', 1, 5, 15, 30, 0.5, '{"description": "Highest priority customers with 24/7 dedicated support"}'),
    ('Gold', 2, 15, 30, 60, 0.75, '{"description": "Premium customers with priority support"}'),
    ('Silver', 3, 30, 60, 120, 1.0, '{"description": "Standard customers with business hours support"}'),
    ('Bronze', 4, 60, 120, 240, 1.5, '{"description": "Basic tier customers"}');
