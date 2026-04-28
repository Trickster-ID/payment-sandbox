# Asset Inventory

Last updated: 2026-04-28

## Classification Legend

- `C1`: Public/non-sensitive
- `C2`: Internal operational
- `C3`: Sensitive security/transaction data

## Asset Table

| Asset ID | Asset | Location | Type | Classification | Owner | Notes |
|---|---|---|---|---|---|---|
| A-001 | Backend source code | `app/` | code | C2 | Backend lead | Core business logic |
| A-002 | API docs | `docs/swagger.yaml` and `docs/swagger.json` | docs | C1 | Backend lead | Public API contract |
| A-003 | ISO evidence docs | `docs/iso/` | docs | C2 | Security owner | Audit evidence package |
| A-004 | PostgreSQL data | Docker service `postgres` | database | C3 | Service operator | Core transaction data |
| A-005 | MongoDB journey logs | Docker service `mongo` | database | C2 | Service operator | Audit trail, best-effort |
| A-006 | App environment config | `.env`, runtime env vars | configuration | C3 | Service operator | Secrets and connection strings |
| A-007 | Verification scripts | `misc/verify/` | script | C2 | Backend lead | Readiness automation |
| A-008 | Operational scripts | `misc/ops/` | script | C2 | Service operator | Backup/restore utilities |

## Critical Dependencies

- PostgreSQL availability directly affects all business operations.
- JWT secret integrity directly affects auth security.
- MongoDB availability affects audit detail completeness but should not block core transactions.

## Protection Requirements

- C3 assets must not be logged in plaintext.
- C3 secrets must not use insecure defaults in non-local environments.
- C3 data must be covered by backup/restore procedures.
