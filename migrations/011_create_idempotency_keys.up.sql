-- Migration: Create idempotency_keys table for webhook deduplication
-- This table stores idempotency keys to prevent duplicate alert creation
-- from repeated webhook deliveries.

CREATE TABLE IF NOT EXISTS idempotency_keys (
    -- The idempotency key, typically a hash of the request or client-provided value
    key VARCHAR(255) PRIMARY KEY,

    -- When this key was first created
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- When this key expires and can be removed/reused
    expires_at TIMESTAMPTZ NOT NULL
);

-- Index for efficient cleanup of expired keys
CREATE INDEX IF NOT EXISTS idx_idempotency_keys_expires_at
    ON idempotency_keys(expires_at);

-- Comment on table for documentation
COMMENT ON TABLE idempotency_keys IS
    'Stores idempotency keys to prevent duplicate webhook processing';

COMMENT ON COLUMN idempotency_keys.key IS
    'The idempotency key (hash of request body + integration key, or X-Idempotency-Key header value)';

COMMENT ON COLUMN idempotency_keys.created_at IS
    'Timestamp when this key was first stored';

COMMENT ON COLUMN idempotency_keys.expires_at IS
    'Timestamp after which this key can be removed and the same request can be processed again';
