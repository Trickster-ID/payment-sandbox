-- Migration: bring an existing payment-sandbox DB up to the post-refactor schema
-- Safe to re-run (idempotent). Apply with:
--   PGPASSWORD=secretpassword psql -h 127.0.0.1 -U root -d payment_sandbox \
--     -f misc/migrations/2026-05-01_money-int64-and-phase1-3.sql

BEGIN;

-- Phase 0.6: money columns DECIMAL(15,2) -> BIGINT (smallest unit = 1 IDR)
ALTER TABLE merchants ALTER COLUMN balance TYPE BIGINT
    USING (round(balance)::bigint);
ALTER TABLE invoices  ALTER COLUMN amount  TYPE BIGINT
    USING (round(amount)::bigint);
ALTER TABLE topups    ALTER COLUMN amount  TYPE BIGINT
    USING (round(amount)::bigint);

-- Phase 0.6: refunds.amount must exist as BIGINT
ALTER TABLE refunds ADD COLUMN IF NOT EXISTS amount BIGINT NOT NULL DEFAULT 0;
ALTER TABLE refunds DROP CONSTRAINT IF EXISTS refunds_amount_check;
ALTER TABLE refunds ADD CONSTRAINT refunds_amount_check CHECK (amount >= 0);

COMMIT;
