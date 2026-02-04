-- Create maintenance_windows table
CREATE TABLE maintenance_windows (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    affected_sites TEXT[] DEFAULT '{}',
    affected_services TEXT[] DEFAULT '{}',
    affected_labels JSONB NOT NULL DEFAULT '{}',
    action VARCHAR(50) NOT NULL DEFAULT 'suppress',
    status VARCHAR(50) NOT NULL DEFAULT 'scheduled',
    change_ticket_id VARCHAR(255),
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create indexes for common queries
CREATE INDEX idx_maintenance_status ON maintenance_windows(status);
CREATE INDEX idx_maintenance_time ON maintenance_windows(start_time, end_time);
CREATE INDEX idx_maintenance_created_by ON maintenance_windows(created_by);
CREATE INDEX idx_maintenance_change_ticket ON maintenance_windows(change_ticket_id);

-- Index for GIN on arrays and JSONB
CREATE INDEX idx_maintenance_affected_sites ON maintenance_windows USING GIN(affected_sites);
CREATE INDEX idx_maintenance_affected_services ON maintenance_windows USING GIN(affected_services);
CREATE INDEX idx_maintenance_affected_labels ON maintenance_windows USING GIN(affected_labels);
