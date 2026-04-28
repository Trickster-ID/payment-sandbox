# Continuity Drill Record - Backup Restore

- Date: 2026-04-28 WIB
- Executor: AI agent session
- Drill type: backup-restore
- Environment: local Docker (`payment-sandbox-postgres`, `payment-sandbox-mongodb`)
- Start time: 08:27 WIB
- End time: 08:28 WIB
- Result: pass

## Scope

Validated real backup and restore workflow using temporary restore databases to avoid overwriting primary datasets.

## Commands Executed

```bash
./misc/ops/drill-backup-restore.sh
make verify-iso
```

## Output Summary

- Postgres backup: created and restored to temporary DB `payment_sandbox_drill_restore_20260428_082752`.
- Postgres validation: public table count in restored DB = `6`.
- Mongo backup: archive created and restored to temporary DB `payment_sandbox_drill_restore_20260428_082752`.
- Mongo validation: collection count in restored DB = `1`.
- Temporary restore databases cleaned up after verification.

## Artifacts

- `tmp/drills/20260428_082752/postgres_20260428_082752.dump`
- `tmp/drills/20260428_082752/mongo_20260428_082752.archive.gz`
- `tmp/drills/20260428_082752/drill-summary.txt`

## Findings

- Current runbook command patterns are executable in the local Docker environment.
- Restore validation is deterministic and non-destructive when using temporary restore databases.

## Follow-up Actions

- Continue quarterly backup-restore drill cadence and add new records in `docs/iso/drills/`.
- Extend drill script for optional API smoke checks against restored datasets if needed.
