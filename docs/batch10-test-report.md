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

