# Control Matrix

Last updated: 2026-04-28

Status legend:

- `done`: implemented and evidenced
- `partial`: started, needs completion
- `missing`: not implemented yet

| Control ID | ISO Ref | Objective | Implementation | Owner | Linked Risks | Evidence | Status |
|---|---|---|---|---|---|---|---|
| C-001 | 27001 A.5 | Define security governance scope | ISO scope docs and ownership defined | Security owner | R-010 | `docs/iso/01-scope-and-context.md`, `docs/iso/README.md` | done |
| C-002 | 27001 A.5/A.8 | Maintain asset inventory and classification | Asset table with owner/classification | Security owner | R-006, R-010 | `docs/iso/02-asset-inventory.md` | done |
| C-003 | 27002 8.2 | Enforce secure configuration defaults | Non-local env config validation in startup | Backend lead | R-001 | `app/config/config.go`, `app/config/config_test.go` | done |
| C-004 | 27002 8.15 | Logging and monitoring for critical events | Journey log policy + implementation review + taxonomy verification script | Backend lead | R-004 | `docs/iso/logging-and-retention.md`, `misc/verify/iso-journey-events.sh`, `app/shared/journeylog/` | done |
| C-005 | 27005 | Risk assessment and treatment tracking | Risk register with score/owner/due date | Security owner | all | `docs/iso/03-risk-register.md` | done |
| C-006 | ISO 8601 | Standardize date/time usage | RFC3339 + UTC policy in docs and validators | Backend lead | R-008 | `docs/iso/logging-and-retention.md`, `docs/api-contract-v1.md`, `app/shared/validator/validator.go`, `app/shared/validator/validator_test.go` | done |
| C-007 | ISO 4217 | Standardize currency semantics | Explicit single-currency policy (`IDR`) with validator helper for ISO code format | Backend lead | R-007 | `docs/iso/logging-and-retention.md`, `docs/api-contract-v1.md`, `app/shared/validator/validator.go`, `app/shared/validator/validator_test.go` | done |
| C-008 | 29147 | Public vulnerability disclosure process | Security reporting policy document | Security owner | R-009 | `SECURITY.md` | done |
| C-009 | 30111 | Vulnerability triage/remediation workflow | Severity + SLA + closure process | Security owner | R-009 | `docs/iso/06-vulnerability-management.md` | done |
| C-010 | 22301 | Business continuity preparedness | BCP and DR runbooks with drill requirements | Service operator | R-005 | `docs/iso/07-business-continuity-plan.md`, `docs/iso/08-disaster-recovery-runbook.md` | done |
| C-011 | 27001 A.9 | Periodic verification and audit evidence | ISO readiness script and checklist | Release assignee | R-010 | `misc/verify/iso-readiness.sh`, `docs/iso/09-audit-evidence-checklist.md`, `Makefile` | done |
