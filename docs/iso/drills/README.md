# Continuity Drill Records

Store continuity and disaster-recovery drill evidence in this directory.

## Minimum Record Format

Each drill record must include:

- Date and timezone
- Executor
- Drill type (`tabletop`, `backup-only`, `backup-restore`)
- Environment
- Commands executed
- Result (`pass`/`fail`)
- Findings and follow-up actions

## Naming Convention

- `YYYY-MM-DD-<drill-type>.md`
- example: `2026-04-28-tabletop.md`

## Mandatory Coverage Rule

- At least one `backup-restore` drill record must exist for ISO readiness checks.
