# Risk Register (ISO/IEC 27005)

Last updated: 2026-04-28

Scoring formula: `risk_score = likelihood x impact` (range 1-25).

- Likelihood: 1 (rare) to 5 (frequent)
- Impact: 1 (low) to 5 (critical)
- High risk threshold: `>= 15`

## Risk Table

| Risk ID | Scenario | Asset | Threat | Vulnerability | Likelihood | Impact | Score | Owner | Treatment | Due Date | Status |
|---|---|---|---|---|---:|---:|---:|---|---|---|---|
| R-001 | Token forgery or weak JWT secret | A-006 | Unauthorized access | Insecure or default JWT secret | 4 | 5 | 20 | Backend lead | Enforce non-local secret validation in startup config | 2026-05-05 | mitigated |
| R-002 | Access escalation to admin endpoints | A-001 | Privilege abuse | Misconfigured middleware route guard | 3 | 5 | 15 | Backend lead | Route parity tests and auth/role integration tests | 2026-05-07 | open |
| R-003 | Invalid payment/refund transition accepted | A-004 | Data integrity loss | Missing state guard in service/repo/DB | 2 | 5 | 10 | Module owner | Keep state-machine triggers and negative tests | 2026-05-10 | mitigated |
| R-004 | Journey logs missing during incidents | A-005 | Audit evidence gap | Mongo unavailable or logger misconfig | 3 | 3 | 9 | Service operator | Health monitoring + retention policy + fallback documentation | 2026-05-12 | open |
| R-005 | DB outage blocks transaction processing | A-004 | Availability disruption | No practiced restore runbook | 3 | 4 | 12 | Service operator | BCP/DR runbook + recovery drill cadence (backup-restore drill executed, continue quarterly cadence) | 2026-05-15 | mitigated |
| R-006 | Secrets leaked in repository logs/docs | A-006 | Data exposure | Weak review discipline | 2 | 4 | 8 | All maintainers | Add security policy and review checklist | 2026-05-03 | open |
| R-007 | Currency interpretation mismatch in totals | A-002 | Reporting error | No explicit ISO 4217 policy in docs/contracts | 3 | 3 | 9 | Backend lead | Document single-currency policy (`IDR`) and enforce in docs | 2026-05-08 | mitigated |
| R-008 | Date parsing ambiguity in API usage | A-002 | Contract misuse | Inconsistent datetime examples/format | 2 | 3 | 6 | Backend lead | Standardize RFC3339/UTC in docs and validators | 2026-05-08 | mitigated |
| R-009 | Vulnerability reports ignored or delayed | A-003 | Unhandled security weakness | No formal disclosure/triage process | 3 | 4 | 12 | Security owner | Add `SECURITY.md` and triage workflow with SLA | 2026-05-06 | mitigated |
| R-010 | Incomplete audit evidence during review | A-003 | Compliance failure | No evidence checklist and single verification command | 3 | 4 | 12 | Release assignee | Add ISO evidence checklist + `make verify-iso` | 2026-05-10 | mitigated |

## High-Risk Focus

- R-001: complete config hardening first.
- R-002: keep role-guard tests as release gate.
- R-005: maintain quarterly backup-restore cadence and evidence.

## Residual Risk Notes

- This is a sandbox environment. Some controls are documented for operational readiness but not all enterprise controls are automated yet.
- Continuity risk posture depends on sustaining quarterly drill discipline.
