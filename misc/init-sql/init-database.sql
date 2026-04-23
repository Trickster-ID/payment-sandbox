-- ==========================================
-- PREPARATION
-- ==========================================
-- Kalau pakai PostgreSQL 17, `uuid_v7()` udah bawaan, jadi gak butuh ekstensi uuid-ossp lagi.
-- Tapi kalo di bawah versi tersebut, bisa pakau query di bawah buat install extensionnya.
-- CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ==========================================
-- 1. ENUM TYPES (Sesuai State Machine Rules)
-- ==========================================
CREATE TYPE user_role AS ENUM ('MERCHANT', 'ADMIN');
CREATE TYPE invoice_status AS ENUM ('PENDING', 'PAID', 'EXPIRED');
CREATE TYPE payment_status AS ENUM ('PENDING', 'SUCCESS', 'FAILED');
CREATE TYPE refund_status AS ENUM ('REQUESTED', 'APPROVED', 'REJECTED', 'SUCCESS', 'FAILED');
CREATE TYPE payment_method AS ENUM ('WALLET', 'VA_DUMMY', 'EWALLET_DUMMY');

-- ==========================================
-- 2. TABLES & RELATIONS
-- ==========================================

-- Tabel Users
CREATE TABLE users (
    -- Kita pakai uuid_v7() biar urut berdasarkan waktu, insert lebih cepet, dan index gak gampang bengkak!
                       id UUID PRIMARY KEY DEFAULT uuid_v7(),
                       name VARCHAR(255) NOT NULL,
                       email VARCHAR(255) UNIQUE NOT NULL,
                       password_hash VARCHAR(255) NOT NULL,
                       role user_role NOT NULL,
                       created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
                       updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Tabel Merchants (1:1 dengan Users)
CREATE TABLE merchants (
                           id UUID PRIMARY KEY DEFAULT uuid_v7(),
                           user_id UUID UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                           balance DECIMAL(15, 2) NOT NULL DEFAULT 0.00,
                           created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
                           updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Tabel Invoices
CREATE TABLE invoices (
                          id UUID PRIMARY KEY DEFAULT uuid_v7(),
                          merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
                          invoice_number VARCHAR(100) UNIQUE NOT NULL,
                          customer_name VARCHAR(255) NOT NULL,
                          customer_email VARCHAR(255) NOT NULL,
                          amount DECIMAL(15, 2) NOT NULL CHECK (amount > 0),
                          description TEXT,
                          due_date TIMESTAMP WITH TIME ZONE NOT NULL,
                          status invoice_status NOT NULL DEFAULT 'PENDING',
                          payment_link_token VARCHAR(255) UNIQUE NOT NULL,
                          created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
                          updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Tabel Payment Intents
CREATE TABLE payment_intents (
                                 id UUID PRIMARY KEY DEFAULT uuid_v7(),
                                 invoice_id UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
                                 method payment_method NOT NULL,
                                 status payment_status NOT NULL DEFAULT 'PENDING',
                                 created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
                                 updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Tabel Topups (Balance Requests)
CREATE TABLE topups (
                        id UUID PRIMARY KEY DEFAULT uuid_v7(),
                        merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
                        amount DECIMAL(15, 2) NOT NULL CHECK (amount > 0),
                        status payment_status NOT NULL DEFAULT 'PENDING',
                        created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
                        updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Tabel Refunds
CREATE TABLE refunds (
                         id UUID PRIMARY KEY DEFAULT uuid_v7(),
                         transaction_id UUID NOT NULL REFERENCES payment_intents(id) ON DELETE CASCADE,
                         reason TEXT NOT NULL,
                         status refund_status NOT NULL DEFAULT 'REQUESTED',
                         created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
                         updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- ==========================================
-- 3. INDEXING (Biar performa ngebut <= 300ms)
-- ==========================================

-- Index buat Invoices
-- Alasan: Merchant kan butuh fitur nampilin daftar invoice pake filter dan pagination.
-- Index ini bikin query nyari invoice per merchant jadi jauh lebih kenceng.
CREATE INDEX idx_invoices_merchant_id ON invoices(merchant_id);

-- Alasan: Admin butuh liat statistik total PAID/FAILED/EXPIRED di dashboard.
-- Selain itu, sistem bakal ngecek invoice PENDING yang udah lewat due_date.
-- Kalo gak ada index ini, database harus ngecek jutaan row satu-satu (Full Table Scan).
CREATE INDEX idx_invoices_status ON invoices(status);

-- Alasan: Ini yang paling krusial! Pas pembeli buka public link payment (/pay/:token),
-- backend butuh nyari data invoice berdasarkan token ini. Index ini bikin pencariannya instan.
CREATE INDEX idx_invoices_payment_token ON invoices(payment_link_token);

-- Index buat Payment Intents
-- Alasan: Postgres emang ngecek relasi Foreign Key, tapi dia nggak otomatis bikinin index-nya.
-- Index ini wajib ditambahin biar pas nge-JOIN data dari invoice ke payment intent nggak bikin database ngos-ngosan.
CREATE INDEX idx_payment_intents_invoice_id ON payment_intents(invoice_id);

-- Alasan: Biar admin gampang pas nyari atau nge-filter payment intent di Admin Panel.
CREATE INDEX idx_payment_intents_status ON payment_intents(status);

-- Index buat Topups
-- Alasan: Biar kenceng pas merchant mau liat riwayat simulasi saldo dan top-up mereka.
CREATE INDEX idx_topups_merchant_id ON topups(merchant_id);

-- Alasan: Biar gampang pas admin mau filter top-up mana aja yang statusnya masih PENDING dan nunggu diproses.
CREATE INDEX idx_topups_status ON topups(status);

-- Index buat Refunds
-- Alasan: Sama kayak kasus Foreign Key sebelumnya. Biar operasi nge-JOIN dari transaksi ke data refund tetep mulus.
CREATE INDEX idx_refunds_transaction_id ON refunds(transaction_id);

-- Alasan: Admin butuh panel buat nge-approve atau nge-reject refund.
-- Index ini ngebantu query list refund yang statusnya masih REQUESTED biar responnya cepet.
CREATE INDEX idx_refunds_status ON refunds(status);