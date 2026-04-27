# Requirement Gap Tracking

Reference: `.agents/project-requirement.md`

Legend:
- `done`: implemented and verified by tests/manual checks.
- `partial`: implemented but requires additional hardening/tests/docs.
- `missing`: not implemented yet.

## Backend Requirement Status

| Area | Status | Notes |
|---|---|---|
| Registration + login (hashed password + JWT) | done | Implemented in auth module |
| Role-based authorization middleware | done | Merchant/Admin guards in router |
| Wallet top-up simulation | done | Create + admin status update |
| Invoice create/list/detail + payment token | done | Merchant flow covered |
| Payment intent simulation + admin update | done | Public create + admin update |
| Refund request/review/process | done | Merchant/admin flows complete |
| Admin dashboard stats | done | `/admin/stats` endpoint available |
| Atomic transaction handling (payment/refund/top-up success) | done | DB-backed integration flow tests now verify top-up, payment, and refund success effects |
| State transition rules | done | Negative integration tests cover finalized reprocessing and refund process-before-approval failures |
| Standard JSON response + errors | done | Shared envelope utilities used broadly |
| Pagination standardization | done | Shared pagination utility added and used in invoice list |
| Shared validation utilities | done | Shared validator package added with tests |
| Structured logging key events | done | Journey log events emitted in handlers |
| MongoDB transaction journey persistence | done | `shared/journeylog` + Mongo logger available |
| Swagger/OpenAPI accuracy | partial | Parity review documented in `docs/swagger-parity-review.md`; `/healthz` -> `/ping` mismatch fixed, annotation coverage follow-up remains |
| Unit tests for business logic | done | Table-driven tests across service/handler layers with mockery |
| Integration tests for core flows | done | DB-backed integration tests in `app/cmd/integration_batch10_test.go` cover auth, invoice, payment, refund, and access-control negatives |
| README operational documentation | partial | Core run docs exist; verify final completeness before release |

## Next Focus

1. Expand/standardize Swagger annotations across non-auth handlers to reduce future drift.
2. Finalize backend acceptance checklist and handoff notes for assessment delivery.
3. Continue Batch 11 reliability/performance verification (query plan checks and response-time evidence).
