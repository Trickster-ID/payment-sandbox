# ISO Program Final Handover

Last updated: 2026-04-28

## Completed

- ISO documentation baseline under `docs/iso/`.
- Risk register and control matrix with mapped ownership.
- Non-local configuration hardening for JWT secret safety.
- Vulnerability disclosure and handling process documentation.
- Continuity and DR runbooks.
- Continuity drill evidence bootstrap:
  - `docs/iso/drills/2026-04-28-tabletop.md`
  - `docs/iso/drills/2026-04-28-backup-restore.md`
  - verification check `./misc/verify/iso-drill-evidence.sh`
- ISO readiness verification command (`make verify-iso`).
- ISO CI verification automation:
  - `make verify-iso-ci`
  - `.github/workflows/iso-verification.yml`

## Deferred / Partial

- Currency handling is currently documented as single-currency mode (`IDR`) and not yet modeled as explicit API field.
- Recovery drill evidence has started; recurring execution records should be maintained each quarter.

## Residual Risks

- R-004: journey log completeness depends on Mongo availability and operational monitoring.
- R-005: mitigated with executed backup-restore drill; periodic cadence must be maintained.
- R-007: currency interpretation remains policy-based until explicit API field is introduced.

## Next Iteration Priorities

1. Add explicit `currency_code` fields in next API version if multi-currency is required.
2. Capture quarterly backup-restore drill records and attach to audit evidence checklist.
3. Add optional smoke test against restored dataset in drill workflow.
