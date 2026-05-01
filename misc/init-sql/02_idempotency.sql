-- ==========================================
-- IDEMPOTENCY RECORDS
-- ==========================================
-- Stores idempotency keys for all state-changing endpoints.
-- Two-tier check: Redis (fast) + this table (durable).
-- Keys expire after TTL (default 24h).

CREATE TABLE IF NOT EXISTS idempotency_records (
    key             TEXT PRIMARY KEY,
    user_id         UUID,                       -- nullable for unauthenticated webhooks
    request_hash    TEXT NOT NULL,              -- SHA-256 of request body
    status          TEXT NOT NULL CHECK (status IN ('in_progress', 'completed', 'failed')),
    response_code   INT,
    response_body   JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_idempotency_expires ON idempotency_records(expires_at);
