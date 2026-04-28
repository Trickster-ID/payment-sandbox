# Disaster Recovery Runbook

Last updated: 2026-04-28

## Preconditions

- Docker services available or equivalent database hosts reachable.
- Credentials set in environment.
- Backup directory write permission.

## PostgreSQL Backup

Command pattern:

```bash
PGPASSWORD="$DB_PASSWORD" pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -Fc -f "$BACKUP_DIR/postgres.dump"
```

## PostgreSQL Restore

Command pattern:

```bash
PGPASSWORD="$DB_PASSWORD" pg_restore -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" --clean --if-exists "$BACKUP_FILE"
```

## MongoDB Backup

Command pattern:

```bash
mongodump --uri "$MONGO_URI" --db "$MONGO_DB_NAME" --archive="$BACKUP_DIR/mongo.archive" --gzip
```

## MongoDB Restore

Command pattern:

```bash
mongorestore --uri "$MONGO_URI" --nsInclude "$MONGO_DB_NAME.*" --archive="$BACKUP_FILE" --gzip --drop
```

## Post-Restore Validation Checklist

- API health endpoint reachable (`/api/v1/ping`).
- Auth login flow works.
- Merchant invoice list endpoint returns expected records.
- Payment/refund state transitions still enforce valid state machine.
- Journey logs are written (if Mongo enabled).

## Dry-Run Record Template

- Date:
- Executor:
- Environment:
- Data source backup timestamp:
- Start time:
- End time:
- Result (`pass/fail`):
- Notes and corrective actions:

Store drill records in:

- `docs/iso/drills/`
