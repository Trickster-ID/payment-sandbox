# Statement of Applicability (SoA)

Last updated: 2026-04-28

This SoA is scoped to the repository-level backend implementation and engineering controls.

## Applicable Controls

| Control Theme | Applicable | Reason | Reference |
|---|---|---|---|
| Access control and role enforcement | yes | Admin and merchant flows require strict separation | middleware and integration tests |
| Secure configuration management | yes | Env-based runtime configuration includes secrets | `app/config/config.go` |
| Logging and traceability | yes | Payment and refund lifecycle must be auditable | journey logs + logging policy |
| Risk management workflow | yes | Need tracked risk-to-treatment mapping | risk register |
| Vulnerability disclosure and handling | yes | Security issues require governed intake and SLA | `SECURITY.md`, vuln management doc |
| Continuity and disaster recovery | yes | Service outage and DB restore readiness are required | BCP/DR docs |
| Date/time standardization (ISO 8601) | yes | API contracts depend on consistent datetime parsing | validator + API docs |
| Currency standardization (ISO 4217) | yes | Financial amounts need explicit currency semantics | policy docs and API contract |

## Not Applicable (Current Repository Scope)

| Control Theme | Applicable | Reason |
|---|---|---|
| Physical security controls | no | Outside repository and software implementation scope |
| Supplier/third-party payment processor controls | no | Third-party payment integrations are out of scope |
| Endpoint device hardening for employee laptops | no | Managed by organizational IT, not this codebase |

## Review Cadence

- Revisit SoA each release milestone or major architecture change.
- Any change in scope must update this document and control matrix together.
