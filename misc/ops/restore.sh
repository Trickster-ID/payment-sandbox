#!/usr/bin/env bash
set -euo pipefail

PG_BACKUP_FILE="${1:-}"
MONGO_BACKUP_FILE="${2:-}"

if [[ -z "$PG_BACKUP_FILE" || -z "$MONGO_BACKUP_FILE" ]]; then
  echo "usage: $0 <postgres_backup.dump> <mongo_backup.archive.gz>" >&2
  exit 1
fi

: "${DB_HOST:?DB_HOST is required}"
: "${DB_PORT:?DB_PORT is required}"
: "${DB_USER:?DB_USER is required}"
: "${DB_PASSWORD:?DB_PASSWORD is required}"
: "${DB_NAME:?DB_NAME is required}"
: "${MONGO_URI:?MONGO_URI is required}"
: "${MONGO_DB_NAME:?MONGO_DB_NAME is required}"

echo "[restore] restoring postgres from $PG_BACKUP_FILE"
PGPASSWORD="$DB_PASSWORD" pg_restore -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" --clean --if-exists "$PG_BACKUP_FILE"

echo "[restore] restoring mongo from $MONGO_BACKUP_FILE"
mongorestore --uri "$MONGO_URI" --nsInclude "$MONGO_DB_NAME.*" --archive="$MONGO_BACKUP_FILE" --gzip --drop

echo "[restore] completed"
