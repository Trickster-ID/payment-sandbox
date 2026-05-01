-- ==========================================
-- LEDGER TABLES (Double-Entry + Immutable)
-- ==========================================
-- Strategy 1: Double-Entry Bookkeeping
-- Strategy 2: Immutable Ledger (Append-Only)
--
-- Money truth lives here. merchants.balance is a denormalized cache.
-- All amounts stored as BIGINT (smallest unit = 1 IDR).

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'account_type') THEN
        CREATE TYPE account_type AS ENUM ('asset', 'liability', 'revenue', 'expense', 'equity');
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS accounts (
    id          UUID PRIMARY KEY DEFAULT app_uuid(),
    user_id     UUID,
    merchant_id UUID REFERENCES merchants(id),
    name        TEXT NOT NULL,
    type        account_type NOT NULL,
    currency    CHAR(3) NOT NULL DEFAULT 'IDR',
    balance     BIGINT NOT NULL DEFAULT 0,
    version     BIGINT NOT NULL DEFAULT 0,        -- for optimistic locking
    is_active   BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_accounts_merchant ON accounts(merchant_id) WHERE merchant_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_accounts_user ON accounts(user_id) WHERE user_id IS NOT NULL;

-- Ledger transaction groups all entries for one business event.
CREATE TABLE IF NOT EXISTS ledger_transactions (
    id          UUID PRIMARY KEY DEFAULT app_uuid(),
    reference   TEXT UNIQUE NOT NULL,   -- ties back to payment_id, topup_id, refund_id, etc.
    description TEXT NOT NULL,
    metadata    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID                    -- user/admin/system actor
);

-- Each row = one side of a double-entry posting (debit OR credit).
-- NEVER updated or deleted. Use Reverse() to cancel.
CREATE TABLE IF NOT EXISTS ledger_entries (
    id             BIGSERIAL PRIMARY KEY,
    transaction_id UUID NOT NULL REFERENCES ledger_transactions(id),
    account_id     UUID NOT NULL REFERENCES accounts(id),
    direction      CHAR(1) NOT NULL CHECK (direction IN ('D', 'C')),   -- D=Debit, C=Credit
    amount         BIGINT NOT NULL CHECK (amount > 0),
    currency       CHAR(3) NOT NULL,
    balance_after  BIGINT NOT NULL,    -- snapshot of account balance after this entry
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ledger_entries_tx      ON ledger_entries(transaction_id);
CREATE INDEX IF NOT EXISTS idx_ledger_entries_account ON ledger_entries(account_id, created_at DESC);

-- Strategy 2: Enforce immutability at the DB level.
-- Any UPDATE or DELETE on ledger_entries raises an exception.
CREATE OR REPLACE FUNCTION reject_ledger_modifications() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'ledger_entries is append-only';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS prevent_ledger_update ON ledger_entries;
CREATE TRIGGER prevent_ledger_update
BEFORE UPDATE OR DELETE ON ledger_entries
FOR EACH ROW EXECUTE FUNCTION reject_ledger_modifications();

-- Updated_at trigger for accounts
DROP TRIGGER IF EXISTS trg_accounts_updated_at ON accounts;
CREATE TRIGGER trg_accounts_updated_at
BEFORE UPDATE ON accounts
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- Seed system accounts (deterministic UUIDs so they can be referenced in Go constants)
INSERT INTO accounts (id, name, type, currency)
VALUES
    ('00000000-0000-4000-8000-000000000010'::uuid, 'system:topup_clearing',   'liability', 'IDR'),
    ('00000000-0000-4000-8000-000000000011'::uuid, 'system:pending_payments', 'liability', 'IDR'),
    ('00000000-0000-4000-8000-000000000012'::uuid, 'system:fees_revenue',     'revenue',   'IDR'),
    ('00000000-0000-4000-8000-000000000013'::uuid, 'system:refunds_expense',  'expense',   'IDR')
ON CONFLICT (id) DO NOTHING;

-- Backfill: create a wallet account for each existing merchant.
-- balance is copied from merchants.balance (the old denormalized column).
INSERT INTO accounts (merchant_id, name, type, currency, balance)
SELECT m.id,
       'wallet:' || m.id::text,
       'asset',
       'IDR',
       m.balance
FROM merchants m
WHERE m.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM accounts a WHERE a.merchant_id = m.id
  );

-- Auto-provision a wallet ledger account for every newly inserted merchant
-- so the post-Phase-2 invariant ("every merchant has a wallet account") holds.
CREATE OR REPLACE FUNCTION provision_merchant_account() RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO accounts (merchant_id, name, type, currency, balance)
    VALUES (NEW.id, 'wallet:' || NEW.id::text, 'asset', 'IDR', COALESCE(NEW.balance, 0))
    ON CONFLICT DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_provision_merchant_account ON merchants;
CREATE TRIGGER trg_provision_merchant_account
AFTER INSERT ON merchants
FOR EACH ROW EXECUTE FUNCTION provision_merchant_account();
