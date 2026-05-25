# Batch 10 Test Report

Last updated: 2026-04-27 09:54 WIB

## Scope Covered

### Integration tests (`app/cmd/integration_batch10_test.go`)
- Auth endpoints:
  - register success
  - login success
  - duplicate register input
  - invalid email payload
  - invalid credentials
- Access control:
  - missing bearer token (`auth_missing_bearer_token`)
  - invalid bearer token (`auth_invalid_token`)
  - merchant blocked from admin endpoints (`auth_forbidden`)
  - admin blocked from merchant endpoints (`auth_forbidden`)
- Merchant invoice flow:
  - create invoice success
  - invalid due-date format
- Admin payment update flow:
  - payment intent status update success
  - invalid method on payment intent creation
  - invalid/missing status payload
  - reprocessing finalized payment intent blocked
- Refund approval/process flow:
  - request refund success
  - review approve success
  - process refund success
  - invalid review decision
  - process before approval blocked
  - reprocessing finalized refund blocked
- DB-backed assertions:
  - payment intent and invoice final statuses
  - refund final status and merchant balance adjustment

### Complete API E2E tests (`app/cmd/e2e_api_test.go`)
- Covers every registered API route except Swagger UI through in-process HTTP requests against the real router and PostgreSQL-backed repositories.
- Authenticates through the real OAuth2 token endpoint instead of minting JWTs directly.
- Exercises public, secured, merchant, and admin route groups:
  - `/ping`, `/users/register`, OAuth2 token/introspect/revoke/authorize/userinfo.
  - public invoice/payment link endpoints.
  - merchant wallet, top-up, invoice, refund, and OAuth2 client endpoints.
  - admin top-up, payment intent, refund, stats, wallet transaction, ledger account, and merchant listing endpoints.
- Verifies idempotency behavior for protected money-mutating POST routes:
  - missing key returns `idempotency_key_required`.
  - replaying the same key and payload returns the original response.
  - reusing the same key with a different payload returns `idempotency_key_conflict`.
- Includes DB-backed lifecycle assertions:
  - top-up success increases balance once.
  - payment success sets payment intent to `SUCCESS` and invoice to `PAID` atomically.
  - refund success sets refund to `SUCCESS` and restores merchant balance.
  - admin stats and ledger/wallet transaction endpoints reflect lifecycle data.
- Includes negative E2E coverage for auth, role guards, invalid payloads, invalid state transitions, ownership isolation, and invalid filters.

### Service tests
- Added/expanded tests:
  - `app/modules/invoice/services/invoice_service_test.go`
    - create invoice
    - list invoices
    - invoice by id
  - `app/modules/admin/services/admin_service_test.go`
    - date parsing validation
    - filter mapping for repository call

## Verification Commands

```bash
go test ./app/cmd -run TestIntegration -v
go test ./app/cmd -run TestE2EAPI -v
go test ./app/modules/admin/services ./app/modules/invoice/services
make coverage-services
go test ./...
```

## Service Coverage Snapshot

From `make coverage-services`:
- `admin/services`: 100.0%
- `auth/services`: 91.3%
- `invoice/services`: 100.0%
- `payment/services`: 100.0%
- `refund/services`: 94.1%
- `wallet/services`: 90.9%
