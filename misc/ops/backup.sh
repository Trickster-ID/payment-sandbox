#!/usr/bin/env bash
set -euo pipefail

BACKUP_DIR="${1:-./tmp/backups}"
mkdir -p "$BACKUP_DIR"

: "${DB_HOST:?DB_HOST is required}"
: "${DB_PORT:?DB_PORT is required}"
: "${DB_USER:?DB_USER is required}"
: "${DB_PASSWORD:?DB_PASSWORD is required}"
: "${DB_NAME:?DB_NAME is required}"
: "${MONGO_URI:?MONGO_URI is required}"
: "${MONGO_DB_NAME:?MONGO_DB_NAME is required}"

ts="$(date +%Y%m%d_%H%M%S)"
pg_file="$BACKUP_DIR/postgres_${ts}.dump"
mongo_file="$BACKUP_DIR/mongo_${ts}.archive.gz"

echo "[backup] creating postgres backup at $pg_file"
PGPASSWORD="$DB_PASSWORD" pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -Fc -f "$pg_file"

echo "[backup] creating mongo backup at $mongo_file"
mongodump --uri "$MONGO_URI" --db "$MONGO_DB_NAME" --archive="$mongo_file" --gzip

echo "[backup] completed"
