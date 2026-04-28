# ISO Hardening Program - Payment Sandbox Backend

Last updated: 2026-04-28

This directory contains the ISO-aligned implementation package for backend security, risk, continuity, and audit readiness.

## Standards in Scope

- ISO/IEC 27001 (ISMS baseline)
- ISO/IEC 27002 (controls implementation guidance)
- ISO/IEC 27005 (risk management)
- ISO/IEC 22301 (business continuity)
- ISO 8601 (date/time standardization)
- ISO 4217 (currency standardization)
- ISO/IEC 29147 (vulnerability disclosure)
- ISO/IEC 30111 (vulnerability handling lifecycle)

## Document Map

- [01-scope-and-context.md](./01-scope-and-context.md)
- [02-asset-inventory.md](./02-asset-inventory.md)
- [03-risk-register.md](./03-risk-register.md)
- [04-control-matrix.md](./04-control-matrix.md)
- [05-statement-of-applicability.md](./05-statement-of-applicability.md)
- [06-vulnerability-management.md](./06-vulnerability-management.md)
- [07-business-continuity-plan.md](./07-business-continuity-plan.md)
- [08-disaster-recovery-runbook.md](./08-disaster-recovery-runbook.md)
- [09-audit-evidence-checklist.md](./09-audit-evidence-checklist.md)
- [logging-and-retention.md](./logging-and-retention.md)
- [final-handover.md](./final-handover.md)

## Ownership

- Security owner: Backend lead
- Risk owner: Module owner per risk item
- Continuity owner: DevOps or service operator
- Evidence owner: Current release assignee

## Execution Rule

- Implementation sequence follows `.agents/iso-generation-plan.md`.
- If conflict exists with `.agents/project-requirement.md`, the requirement file wins.
