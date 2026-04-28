#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

timestamp="$(date +%Y%m%d_%H%M%S)"
artifact_dir="${1:-./tmp/drills/$timestamp}"
mkdir -p "$artifact_dir"

POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-payment-sandbox-postgres}"
POSTGRES_USER="${POSTGRES_USER:-root}"
POSTGRES_DB="${POSTGRES_DB:-payment_sandbox}"

MONGO_CONTAINER="${MONGO_CONTAINER:-payment-sandbox-mongodb}"
MONGO_URI="${MONGO_URI:-mongodb://mongo_user:mongo_password@127.0.0.1:27017/?authSource=admin}"
MONGO_DB_NAME="${MONGO_DB_NAME:-payment_sandbox}"

pg_dump_file="postgres_${timestamp}.dump"
mongo_dump_file="mongo_${timestamp}.archive.gz"
pg_restore_db="${POSTGRES_DB}_drill_restore_${timestamp}"
mongo_restore_db="${MONGO_DB_NAME}_drill_restore_${timestamp}"

echo "[drill] artifacts dir: $artifact_dir"
echo "[drill] postgres container: $POSTGRES_CONTAINER"
echo "[drill] mongo container: $MONGO_CONTAINER"

echo "[drill] step 1/8: backup postgres"
docker exec "$POSTGRES_CONTAINER" sh -lc \
  "pg_dump -U '$POSTGRES_USER' -d '$POSTGRES_DB' -Fc -f '/tmp/$pg_dump_file'"
docker cp "$POSTGRES_CONTAINER:/tmp/$pg_dump_file" "$artifact_dir/$pg_dump_file"
docker exec "$POSTGRES_CONTAINER" sh -lc "rm -f '/tmp/$pg_dump_file'"

echo "[drill] step 2/8: restore postgres into temporary database"
docker cp "$artifact_dir/$pg_dump_file" "$POSTGRES_CONTAINER:/tmp/$pg_dump_file"
docker exec "$POSTGRES_CONTAINER" sh -lc \
  "psql -U '$POSTGRES_USER' -d postgres -v ON_ERROR_STOP=1 -c 'DROP DATABASE IF EXISTS \"$pg_restore_db\";' -c 'CREATE DATABASE \"$pg_restore_db\";'"
docker exec "$POSTGRES_CONTAINER" sh -lc \
  "pg_restore -U '$POSTGRES_USER' -d '$pg_restore_db' --clean --if-exists '/tmp/$pg_dump_file'"
pg_table_count="$(docker exec "$POSTGRES_CONTAINER" sh -lc \
  "psql -U '$POSTGRES_USER' -d '$pg_restore_db' -tAc \"SELECT count(*) FROM information_schema.tables WHERE table_schema='public';\"")"
echo "[drill] postgres restored table count: ${pg_table_count}"

echo "[drill] step 3/8: backup mongo"
docker exec "$MONGO_CONTAINER" sh -lc \
  "mongodump --uri '$MONGO_URI' --db '$MONGO_DB_NAME' --archive='/tmp/$mongo_dump_file' --gzip"
docker cp "$MONGO_CONTAINER:/tmp/$mongo_dump_file" "$artifact_dir/$mongo_dump_file"
docker exec "$MONGO_CONTAINER" sh -lc "rm -f '/tmp/$mongo_dump_file'"

echo "[drill] step 4/8: restore mongo into temporary database"
docker cp "$artifact_dir/$mongo_dump_file" "$MONGO_CONTAINER:/tmp/$mongo_dump_file"
docker exec "$MONGO_CONTAINER" sh -lc \
  "mongorestore --uri '$MONGO_URI' --archive='/tmp/$mongo_dump_file' --gzip --nsFrom='${MONGO_DB_NAME}.*' --nsTo='${mongo_restore_db}.*' --drop"
mongo_collection_count="$(docker exec "$MONGO_CONTAINER" sh -lc \
  "mongosh '$MONGO_URI' --quiet --eval \"db.getSiblingDB('$mongo_restore_db').getCollectionNames().length\"")"
echo "[drill] mongo restored collection count: ${mongo_collection_count}"

echo "[drill] step 5/8: cleanup temporary restore databases"
docker exec "$POSTGRES_CONTAINER" sh -lc \
  "psql -U '$POSTGRES_USER' -d postgres -v ON_ERROR_STOP=1 -c 'DROP DATABASE IF EXISTS \"$pg_restore_db\";'"
docker exec "$MONGO_CONTAINER" sh -lc \
  "mongosh '$MONGO_URI' --quiet --eval \"db.getSiblingDB('$mongo_restore_db').dropDatabase()\" >/dev/null"
docker exec "$MONGO_CONTAINER" sh -lc "rm -f '/tmp/$mongo_dump_file'"
docker exec "$POSTGRES_CONTAINER" sh -lc "rm -f '/tmp/$pg_dump_file'"

if [[ "${pg_table_count:-0}" -lt 1 ]]; then
  echo "[drill] postgres restore validation failed: table count is zero" >&2
  exit 1
fi
if [[ "${mongo_collection_count:-0}" -lt 1 ]]; then
  echo "[drill] mongo restore validation failed: collection count is zero" >&2
  exit 1
fi

echo "[drill] step 6/8: write summary"
summary_file="$artifact_dir/drill-summary.txt"
{
  echo "timestamp=$timestamp"
  echo "postgres_table_count=$pg_table_count"
  echo "mongo_collection_count=$mongo_collection_count"
  echo "postgres_restore_db=$pg_restore_db"
  echo "mongo_restore_db=$mongo_restore_db"
  echo "status=pass"
} >"$summary_file"

echo "[drill] step 7/8: completed"
echo "[drill] summary file: $summary_file"
echo "[drill] step 8/8: pass"
