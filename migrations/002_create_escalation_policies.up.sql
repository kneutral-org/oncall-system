-- Create escalation_policies table (dependency for teams)
CREATE TABLE escalation_policies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    repeat_count INT NOT NULL DEFAULT 0,
    repeat_interval_seconds INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_escalation_policies_name ON escalation_policies(name);
