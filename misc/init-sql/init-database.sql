-- ==========================================
-- PREPARATION
-- ==========================================
-- Wajib PostgreSQL 17+ (menggunakan uuidv7() untuk sorting berbasis waktu).

CREATE OR REPLACE FUNCTION app_uuid() RETURNS uuid AS $$
BEGIN
    RETURN uuidv7();
END;
$$ LANGUAGE plpgsql VOLATILE;

-- ==========================================
-- 1. ENUM TYPES (Sesuai State Machine Rules)
-- ==========================================
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_role') THEN
        CREATE TYPE user_role AS ENUM ('MERCHANT', 'ADMIN');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'invoice_status') THEN
        CREATE TYPE invoice_status AS ENUM ('PENDING', 'PAID', 'EXPIRED');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'payment_status') THEN
        CREATE TYPE payment_status AS ENUM ('PENDING', 'SUCCESS', 'FAILED');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'refund_status') THEN
        CREATE TYPE refund_status AS ENUM ('REQUESTED', 'APPROVED', 'REJECTED', 'SUCCESS', 'FAILED');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'payment_method') THEN
        CREATE TYPE payment_method AS ENUM ('WALLET', 'VA_DUMMY', 'EWALLET_DUMMY');
    END IF;
END $$;

-- ==========================================
-- 2. TABLES & RELATIONS
-- ==========================================
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT app_uuid(),
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role user_role NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS merchants (
    id UUID PRIMARY KEY DEFAULT app_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    balance DECIMAL(15, 2) NOT NULL DEFAULT 0.00,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS invoices (
    id UUID PRIMARY KEY DEFAULT app_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants(id),
    invoice_number VARCHAR(100) NOT NULL,
    customer_name VARCHAR(255) NOT NULL,
    customer_email VARCHAR(255) NOT NULL,
    amount DECIMAL(15, 2) NOT NULL CHECK (amount > 0),
    description TEXT,
    due_date TIMESTAMPTZ NOT NULL CHECK (due_date::date >= CURRENT_DATE),
    status invoice_status NOT NULL DEFAULT 'PENDING',
    payment_link_token VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS payment_intents (
    id UUID PRIMARY KEY DEFAULT app_uuid(),
    invoice_id UUID NOT NULL REFERENCES invoices(id),
    method payment_method NOT NULL,
    status payment_status NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS topups (
    id UUID PRIMARY KEY DEFAULT app_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants(id),
    amount DECIMAL(15, 2) NOT NULL CHECK (amount > 0),
    status payment_status NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS refunds (
    id UUID PRIMARY KEY DEFAULT app_uuid(),
    payment_intent_id UUID NOT NULL REFERENCES payment_intents(id),
    reason TEXT NOT NULL,
    status refund_status NOT NULL DEFAULT 'REQUESTED',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

-- ==========================================
-- 3. CONSTRAINTS FOR SOFT DELETE + UNIQUENESS
-- ==========================================
CREATE UNIQUE INDEX IF NOT EXISTS uq_users_email_active
    ON users (LOWER(email))
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_merchants_user_id_active
    ON merchants (user_id)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_invoices_invoice_number_active
    ON invoices (invoice_number)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_invoices_payment_token_active
    ON invoices (payment_link_token)
    WHERE deleted_at IS NULL;

-- ==========================================
-- 4. UPDATED_AT AUTOMATION
-- ==========================================
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_users_updated_at ON users;
CREATE TRIGGER trg_users_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_merchants_updated_at ON merchants;
CREATE TRIGGER trg_merchants_updated_at
BEFORE UPDATE ON merchants
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_invoices_updated_at ON invoices;
CREATE TRIGGER trg_invoices_updated_at
BEFORE UPDATE ON invoices
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_payment_intents_updated_at ON payment_intents;
CREATE TRIGGER trg_payment_intents_updated_at
BEFORE UPDATE ON payment_intents
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_topups_updated_at ON topups;
CREATE TRIGGER trg_topups_updated_at
BEFORE UPDATE ON topups
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS trg_refunds_updated_at ON refunds;
CREATE TRIGGER trg_refunds_updated_at
BEFORE UPDATE ON refunds
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- ==========================================
-- 5. STATE MACHINE GUARDS
-- ==========================================
CREATE OR REPLACE FUNCTION enforce_invoice_transition() RETURNS trigger AS $$
BEGIN
    IF NEW.deleted_at IS NOT NULL THEN
        RETURN NEW;
    END IF;

    IF OLD.status = NEW.status THEN
        RETURN NEW;
    END IF;

    IF NOT (
        (OLD.status = 'PENDING' AND NEW.status IN ('PAID', 'EXPIRED'))
    ) THEN
        RAISE EXCEPTION 'invalid invoice transition: % -> %', OLD.status, NEW.status;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_invoice_transition ON invoices;
CREATE TRIGGER trg_invoice_transition
BEFORE UPDATE ON invoices
FOR EACH ROW
EXECUTE FUNCTION enforce_invoice_transition();

CREATE OR REPLACE FUNCTION enforce_payment_transition() RETURNS trigger AS $$
BEGIN
    IF NEW.deleted_at IS NOT NULL THEN
        RETURN NEW;
    END IF;

    IF OLD.status = NEW.status THEN
        RETURN NEW;
    END IF;

    IF NOT (
        (OLD.status = 'PENDING' AND NEW.status IN ('SUCCESS', 'FAILED'))
    ) THEN
        RAISE EXCEPTION 'invalid payment transition: % -> %', OLD.status, NEW.status;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_payment_transition ON payment_intents;
CREATE TRIGGER trg_payment_transition
BEFORE UPDATE ON payment_intents
FOR EACH ROW
EXECUTE FUNCTION enforce_payment_transition();

DROP TRIGGER IF EXISTS trg_topup_transition ON topups;
CREATE TRIGGER trg_topup_transition
BEFORE UPDATE ON topups
FOR EACH ROW
EXECUTE FUNCTION enforce_payment_transition();

CREATE OR REPLACE FUNCTION enforce_refund_transition() RETURNS trigger AS $$
BEGIN
    IF NEW.deleted_at IS NOT NULL THEN
        RETURN NEW;
    END IF;

    IF OLD.status = NEW.status THEN
        RETURN NEW;
    END IF;

    IF NOT (
        (OLD.status = 'REQUESTED' AND NEW.status IN ('APPROVED', 'REJECTED')) OR
        (OLD.status = 'APPROVED'  AND NEW.status IN ('SUCCESS', 'FAILED'))
    ) THEN
        RAISE EXCEPTION 'invalid refund transition: % -> %', OLD.status, NEW.status;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_refund_transition ON refunds;
CREATE TRIGGER trg_refund_transition
BEFORE UPDATE ON refunds
FOR EACH ROW
EXECUTE FUNCTION enforce_refund_transition();

-- ==========================================
-- 6. INDEXING (Performance <= 300ms target)
-- ==========================================
CREATE INDEX IF NOT EXISTS idx_invoices_merchant_id_active
    ON invoices(merchant_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_invoices_status_active
    ON invoices(status)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_invoices_due_date_active
    ON invoices(due_date)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_invoices_payment_token_active
    ON invoices(payment_link_token)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_payment_intents_invoice_id_active
    ON payment_intents(invoice_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_payment_intents_status_active
    ON payment_intents(status)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_topups_merchant_id_active
    ON topups(merchant_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_topups_status_active
    ON topups(status)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_refunds_payment_intent_id_active
    ON refunds(payment_intent_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_refunds_status_active
    ON refunds(status)
    WHERE deleted_at IS NULL;
