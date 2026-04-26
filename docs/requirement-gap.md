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
| Atomic transaction handling (payment/refund/top-up success) | partial | Implemented in repository flows; needs integration tests to prove atomicity end-to-end |
| State transition rules | partial | Implemented in service/repo logic; needs additional negative integration coverage |
| Standard JSON response + errors | done | Shared envelope utilities used broadly |
| Pagination standardization | done | Shared pagination utility added and used in invoice list |
| Shared validation utilities | done | Shared validator package added with tests |
| Structured logging key events | done | Journey log events emitted in handlers |
| MongoDB transaction journey persistence | done | `shared/journeylog` + Mongo logger available |
| Swagger/OpenAPI accuracy | partial | Swagger exists; contract parity review still needed |
| Unit tests for business logic | done | Table-driven tests across service/handler layers with mockery |
| Integration tests for core flows | partial | Route coverage exists; full DB-backed flow integration tests still pending |
| README operational documentation | partial | Core run docs exist; verify final completeness before release |

## Next Focus

1. Add DB-backed integration tests for core transactional flows (payment success, refund success, top-up success).
2. Review Swagger contract against `docs/api-contract-v1.md` and align examples/error codes.
3. Finalize acceptance checklist and README completeness for handoff.

