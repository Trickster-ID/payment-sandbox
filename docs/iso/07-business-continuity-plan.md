# Business Continuity Plan (ISO/IEC 22301)

Last updated: 2026-04-28

## Objective

Maintain essential payment sandbox operations during service disruption and recover safely.

## Critical Services

- API service (`go run ./app/cmd`)
- PostgreSQL (transaction state source of truth)
- MongoDB journey log service (audit support)

## Disruption Scenarios

- PostgreSQL unavailable.
- MongoDB unavailable.
- Misconfigured environment causing failed startup.
- Application deployment regression.

## Continuity Strategy

- Keep core API dependent only on PostgreSQL for transaction correctness.
- Treat journey logging as best-effort and non-blocking for transaction completion.
- Use documented rollback and restore procedures from DR runbook.

## Target Recovery Metrics

- RTO (service restore target): 60 minutes
- RPO (acceptable data loss window): 15 minutes for database backup strategy

## Incident Communication Path

1. On-call engineer confirms incident and scope.
2. Notify backend lead and service operator.
3. Apply mitigation or failover steps.
4. Publish status update after stabilization.
5. Capture post-incident findings and corrective actions.

## Exercise Cadence

- Conduct continuity tabletop review monthly.
- Conduct backup/restore drill at least quarterly.
- Store drill records in `docs/iso/drills/`.
