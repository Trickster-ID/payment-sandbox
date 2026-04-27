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
| Swagger/OpenAPI accuracy | done | Parity review completed in `docs/swagger-parity-review.md`; `/healthz` -> `/ping` aligned and non-auth handler annotations standardized + regenerated |
| Unit tests for business logic | done | Table-driven tests across service/handler layers with mockery |
| Integration tests for core flows | done | DB-backed integration tests in `app/cmd/integration_batch10_test.go` cover auth, invoice, payment, refund, and access-control negatives |
| README operational documentation | done | README includes prerequisites, env vars, DB init, run/test/mock/swagger commands, and delivery artifact links |

## Next Focus

1. Keep docs and generated swagger artifacts in sync when endpoint contracts change.
2. Re-run performance/query-plan checks on larger seeded datasets for stronger non-functional evidence.
3. Proceed with assessment handoff/review packaging.
