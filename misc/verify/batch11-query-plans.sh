#!/usr/bin/env bash

set -euo pipefail

if ! command -v psql >/dev/null 2>&1; then
  echo "psql is required for Batch 11 query-plan verification."
  exit 1
fi

DB_HOST="${DB_HOST:-127.0.0.1}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-root}"
DB_NAME="${DB_NAME:-payment_sandbox}"
DB_PASSWORD="${DB_PASSWORD:-secretpassword}"

export PGPASSWORD="$DB_PASSWORD"

echo "Running Batch 11 query-plan checks on ${DB_HOST}:${DB_PORT}/${DB_NAME} as ${DB_USER}..."

psql \
  -h "$DB_HOST" \
  -p "$DB_PORT" \
  -U "$DB_USER" \
  -d "$DB_NAME" \
  -v ON_ERROR_STOP=1 <<'SQL'
\timing on

EXPLAIN (ANALYZE, BUFFERS)
SELECT count(*)
FROM invoices i
WHERE i.merchant_id IN (
  SELECT m.id FROM merchants m WHERE m.deleted_at IS NULL LIMIT 1
)
AND i.deleted_at IS NULL;

EXPLAIN (ANALYZE, BUFFERS)
SELECT COALESCE(SUM(pi.amount), 0) AS total_payment_nominal
FROM payment_intents p
JOIN invoices i ON i.id = p.invoice_id AND i.deleted_at IS NULL
JOIN LATERAL (SELECT i.amount) pi ON true
WHERE p.status = 'SUCCESS'
AND p.deleted_at IS NULL
AND i.created_at BETWEEN CURRENT_DATE - INTERVAL '30 days' AND CURRENT_DATE + INTERVAL '1 day';

EXPLAIN (ANALYZE, BUFFERS)
SELECT COALESCE(SUM(i.amount), 0) AS total_refund_nominal
FROM refunds r
JOIN payment_intents p ON p.id = r.payment_intent_id AND p.deleted_at IS NULL
JOIN invoices i ON i.id = p.invoice_id AND i.deleted_at IS NULL
WHERE r.status = 'SUCCESS'
AND r.deleted_at IS NULL
AND i.created_at BETWEEN CURRENT_DATE - INTERVAL '30 days' AND CURRENT_DATE + INTERVAL '1 day';
SQL

echo "Batch 11 query-plan checks completed."
