# ISO Scope and Context

Last updated: 2026-04-28

## Objective

Define the exact system boundary and assumptions for ISO-aligned controls in this repository.

## In-Scope System Boundary

- Backend API service (`app/cmd`, module handlers/services/repositories).
- PostgreSQL primary transactional store.
- MongoDB journey log store.
- Local/development deployment via Docker Compose.
- CI-style verification commands (tests and verification scripts).
- Documentation and operational runbooks under `docs/` and `misc/`.

## Out-of-Scope Boundary

- Frontend applications and UI runtime behavior.
- External third-party payment rails.
- Formal external ISO certification activities.
- Corporate HR/legal controls outside this repository.

## Business Context

The system simulates payment operations for merchant/admin workflows:

- Merchant registration/login
- Invoice lifecycle
- Payment intent simulation
- Refund simulation
- Wallet top-up simulation
- Admin monitoring/statistics

## Security Context Assumptions

- Authentication relies on JWT with expiry.
- Authorization relies on role middleware (`MERCHANT`, `ADMIN`).
- Sensitive secrets are supplied by environment variables.
- State-machine and transaction guarantees are enforced by service + database logic.

## Interested Parties

- Engineering team (implementation and verification).
- Reviewer/assessor (evidence consumer).
- Sandbox operator/admin users.

## Constraints

- Must preserve current API behavior from project requirements.
- Must keep backend testability and clean layering conventions.
