# Backend Acceptance Checklist

Last updated: 2026-04-27 (WIB)

## Functional Requirements

- [x] Authentication endpoints work (`/auth/register`, `/auth/login`)
- [x] JWT auth middleware and role-based access (`MERCHANT`, `ADMIN`)
- [x] Merchant wallet flow (view wallet, create top-up)
- [x] Admin top-up status update flow
- [x] Invoice flow (create/list/detail, payment token generation)
- [x] Public payment flow (`/pay/:token`, create payment intent)
- [x] Admin payment status update flow
- [x] Refund flow (request/review/process)
- [x] Admin dashboard stats endpoint

## Data Integrity and State Machine

- [x] Top-up success path updates merchant balance
- [x] Payment success path updates invoice status to `PAID`
- [x] Refund success path deducts merchant balance
- [x] Reprocessing finalized payment intent is rejected
- [x] Reprocessing finalized refund is rejected
- [x] Refund process before approval is rejected
- [x] Invalid payloads and validation errors return consistent error envelope

## Security and Access Control

- [x] Missing bearer token rejected (`auth_missing_bearer_token`)
- [x] Invalid bearer token rejected (`auth_invalid_token`)
- [x] Merchant cannot call admin endpoints (`auth_forbidden`)
- [x] Admin cannot call merchant endpoints (`auth_forbidden`)

## Testing Evidence

- [x] Unit tests are table-driven and use `testify/require` + `testify/assert`
- [x] Core DB-backed integration tests exist at:
  - `app/cmd/integration_batch10_test.go`
- [x] Full test suite command passes:
  - `go test ./...`
- [x] Service-layer coverage snapshot command available:
  - `make coverage-services`

## Documentation and Tooling

- [x] API contract document exists: `docs/api-contract-v1.md`
- [x] Requirement gap tracker exists: `docs/requirement-gap.md`
- [x] Batch 10 test report exists: `docs/batch10-test-report.md`
- [x] Swagger generation command documented and available (`make swag`)
- [x] Mock generation command documented and available (`make mock`)

## Remaining for Final Handoff

- [x] Swagger/OpenAPI parity review against live behavior and error examples
- [x] Performance evidence capture for key endpoints (target <= 300ms in normal local load)
- [x] Query plan validation notes for key aggregate/list endpoints
