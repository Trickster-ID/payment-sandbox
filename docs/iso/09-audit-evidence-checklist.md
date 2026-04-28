# Audit Evidence Checklist

Last updated: 2026-04-28

Use this checklist before internal/external review.

| Evidence Area | Required Artifact | Verify Command | Latest Status |
|---|---|---|---|
| ISO scope and ownership | `docs/iso/01-scope-and-context.md` | manual review | done |
| Asset inventory | `docs/iso/02-asset-inventory.md` | manual review | done |
| Risk management | `docs/iso/03-risk-register.md` | manual review | done |
| Control mapping | `docs/iso/04-control-matrix.md` | manual review | done |
| SoA | `docs/iso/05-statement-of-applicability.md` | manual review | done |
| Vulnerability process | `SECURITY.md`, `docs/iso/06-vulnerability-management.md` | manual review | done |
| Continuity and DR | `docs/iso/07-business-continuity-plan.md`, `docs/iso/08-disaster-recovery-runbook.md` | manual review | done |
| Continuity drill evidence | records under `docs/iso/drills/` | `./misc/verify/iso-drill-evidence.sh` | done |
| Config hardening | `app/config/config.go`, `app/config/config_test.go` | `go test ./app/config -v` | done |
| API and validation integrity | service and validator tests | `go test ./...` | done |
| Journey event taxonomy | required action names in handlers | `./misc/verify/iso-journey-events.sh` | done |
| ISO verification bundle | readiness automation | `make verify-iso` | done |
| CI-safe ISO verification | readiness automation without strict drill gate | `make verify-iso-ci` | done |
| CI workflow enforcement | automated ISO checks on push/PR | `.github/workflows/iso-verification.yml` | done |

## Reviewer Notes

- This checklist is evidence index only. Detailed policy semantics are defined in each referenced document.
