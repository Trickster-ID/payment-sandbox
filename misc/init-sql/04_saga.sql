-- ==========================================
-- SAGA STATE TABLES
-- ==========================================
-- Strategy 5: Saga Pattern for Distributed Transactions
--
-- Each saga row represents one multi-step business operation.
-- saga_step_log is an immutable audit trail of step outcomes.

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'saga_status') THEN
        CREATE TYPE saga_status AS ENUM (
            'pending', 'running', 'compensating', 'completed', 'failed', 'compensated'
        );
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS sagas (
    id           UUID PRIMARY KEY DEFAULT app_uuid(),
    type         TEXT NOT NULL,            -- 'payment_saga', 'topup_saga', 'refund_saga', etc.
    payload      JSONB NOT NULL,           -- input data for the saga steps
    status       saga_status NOT NULL DEFAULT 'pending',
    current_step INT NOT NULL DEFAULT 0,
    last_error   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS saga_step_log (
    id         BIGSERIAL PRIMARY KEY,
    saga_id    UUID NOT NULL REFERENCES sagas(id),
    step_index INT NOT NULL,
    step_name  TEXT NOT NULL,
    direction  TEXT NOT NULL CHECK (direction IN ('forward', 'compensate')),
    status     TEXT NOT NULL CHECK (status IN ('success', 'failed')),
    error      TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_saga_step_log_saga ON saga_step_log(saga_id);
-- Index for recovery worker: find stuck sagas quickly
CREATE INDEX IF NOT EXISTS idx_sagas_status_updated
    ON sagas(status, updated_at)
    WHERE status IN ('running', 'compensating');

-- Updated_at trigger for sagas
DROP TRIGGER IF EXISTS trg_sagas_updated_at ON sagas;
CREATE TRIGGER trg_sagas_updated_at
BEFORE UPDATE ON sagas
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
