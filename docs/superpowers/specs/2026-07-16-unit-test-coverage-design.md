# Design: Maximize Unit Test Coverage (Table-Driven)

## Goal

Raise unit test coverage as high as reasonably possible across the codebase,
excluding generated files, with all new/changed tests written in table-driven
style using `testify/require` + `testify/assert`, per `AGENTS.md`.

## Current State (baseline, 2026-07-16)

- `go test ./... -coverprofile` total: **41.6%** (raw, includes generated files).
- Business logic (services/handlers/repositories) already 87-100% in most
  modules. The raw total is dragged down by:
  - `docs/docs.go` — swagger-generated, thousands of lines, 0%.
  - `app/cmd/wire_gen.go`, `app/cmd/main.go`, `app/cmd/providers.go` — DI wiring
    and process entrypoint.
  - `*/api/routes.go` registrars across every module — 0%, mechanical route
    registration.
  - Entity enum parsers (`ParsePaymentStatus`, `ParseRefundProcessStatus`) — 0%.
  - `app/modules/ledger/repositories` — 0%, no tests at all yet.
  - `app/shared/locking` (43.8%), `app/shared/idempotency` (69%),
    `app/shared/audit` (73.3%) — partial coverage on infra helpers.
  - `app/modules/oauth2/*` (handlers/services/repositories) — 73-93%, several
    uncovered error branches.

## Decisions (from brainstorming Q&A)

1. **Scope**: target coverage excludes only generated files —
   `docs/docs.go` and `app/cmd/wire_gen.go`. Everything else (including
   `main.go`, `providers.go`, `*/api/routes.go`, entity parsers,
   `ledger/repositories`) is in scope and must be tested.
2. **Test type boundary**: unit tests only, using the existing mocking
   patterns already in the repo (`sqlmock` for repositories, `miniredis` for
   Redis-backed code, generated mocks via `mockery` for
   service/repository interfaces). Small refactors for testability are
   allowed (e.g., extracting a pure function out of a hard-to-test one). No
   new integration tests against real Postgres/Redis/Mongo containers.
3. **Target metric**: add a `make coverage` target that computes total
   coverage from `go test ./... -coverprofile=coverage.out`, with
   `docs/docs.go` and `app/cmd/wire_gen.go` lines stripped from the profile
   before running `go tool cover -func`. Target: **>= 90%** on this
   filtered metric.

## Non-goals

- No 100%-per-package crusade; documented skips are acceptable for
  practically untestable branches (e.g., `main()` process bootstrap).
- No new test frameworks or CI changes beyond the `make coverage` target.
- No behavior changes to production code except minimal testability
  refactors (pure-function extraction), never changing external behavior.

## Approach

### Part 1 — Tooling

Add to `Makefile`:

```makefile
.PHONY: coverage
coverage:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	grep -v -E "docs/docs\.go|app/cmd/wire_gen\.go" coverage.out > coverage.filtered.out
	go tool cover -func=coverage.filtered.out | tail -1
```

This becomes the single source of truth for "current %" throughout
implementation. Re-run after every tier below.

### Part 2 — Work tiers (ordered by ROI, each independently verifiable)

**Tier 1 — Trivial enum/value parsers**
- `app/modules/payment/models/entity.ParsePaymentStatus`
- `app/modules/refund/models/entity.ParseRefundProcessStatus`
- `app/modules/payment/sagas.Name()` (x3, saga step names)
- Table-driven test: valid values, invalid/unknown string, empty string.

**Tier 2 — Route registrars (mechanical, one pattern reused)**
- `admin/api`, `invoice/api`, `ledger/api`, `merchants/api`, `oauth2/api`,
  `payment/api`, `refund/api`, `users/api`, `wallet/api` — each
  `RegisterXRoutes` function.
- Pattern: build a `gin.Engine`, call the register function with nil/mock
  handler struct (only needs the method set, not real behavior), assert
  expected routes exist via `engine.Routes()` (method + path), following the
  existing style in `app/cmd/router_test.go`.
- **`oauth2/api` grouping is load-bearing, do not "correct" it**: as of
  2026-07-16, `POST /oauth2/authorize` lives in `RegisterSecuredRoutes` (behind
  `middleware.AuthMiddleware`), while `GET /oauth2/authorize` stays in
  `RegisterPublicRoutes`. This was a deliberate bug fix (the handler calls
  `middleware.MustUserID`, which requires `AuthMiddleware` to have run first —
  see production bug note below). Tests for `oauth2/api` must assert this exact
  split (`GET` public, `POST` secured), not assume both verbs are public.

  > **Production bug fix context (do not revert)**: `POST /oauth2/authorize`
  > was previously registered under `RegisterPublicRoutes`, which made it
  > 100% broken — `ApproveAuthorize` always failed `MustUserID` and returned
  > 401 regardless of a valid Bearer token, because `AuthMiddleware` never ran.
  > Fixed by moving it to `RegisterSecuredRoutes`. No DB/contract change; only
  > effect is that valid-Bearer-token requests are now correctly accepted.
  > If any new consent/authorize endpoint is added, check whether its handler
  > calls `middleware.MustUserID`/`RequireRoles` — if so it belongs under
  > `RegisterSecuredRoutes`, not `RegisterPublicRoutes`.

**Tier 3 — Repository gaps via `sqlmock`**
- `app/modules/ledger/repositories` (currently 0%, no test file at all): add
  `ledger_repository_test.go` covering `NewRepository`, `Post`, `Reverse`,
  `ListEntriesByAccount`, `GetAccountByMerchantID` — success + DB-error cases,
  following the exact `sqlmock` pattern in
  `app/modules/payment/repositories/payment_repository_test.go`.
- `oauth2/repositories`: add missing `CreateClient` case, and an error-path
  case for `ListClientsByOwner`.
- `payment/repositories`: add error-path sqlmock cases for
  `UpdatePaymentStatus`, `getPaymentIntentByID`, `CreatePaymentIntent`.
- `refund/repositories`: add error-path cases for `RequestRefund`,
  `ReviewRefund`, `ProcessRefund`.

**Tier 4 — Shared infra (mock/fake-backed)**
- `shared/locking.Acquire`/`Release`: table-driven tests using `miniredis`
  (already a dependency, used in `idempotency/store_test.go`) — success,
  already-locked/conflict, and connection-error cases.
- `shared/idempotency`: `cache.Get`/`Set` (0%) and `middleware.WriteString`/
  `Handle` (70.6%) — extend existing `miniredis`-based test file with missing
  branches.
- `shared/audit.Log`/`NewLogger`: mock `io.Writer`, assert log line format;
  cover the currently-uncovered `NewLogger` branch (66.7%) and `Log` (0%).
- `shared/database` (`postgres.New` 71.4%, `redis.NewRedis` 88.9%,
  `mongodb.NewMongoDB` 90.9%): minimal refactor — extract DSN/config-building
  logic into a small pure function (e.g. `buildDSN(cfg) string`) that can be
  unit-tested without a live connection. The actual `sql.Open`/`redis.NewClient`
  call itself stays untested against a real server (see Non-goals).
  - `ponytail:` no real-connection test for `database.New*`; ceiling is the
    pure config/DSN logic. Upgrade path: testcontainers if real integration
    coverage is ever required.

**Tier 5 — Residual branches in already-high-coverage packages**
- `oauth2` handlers/services: `handleRefreshTokenGrant` (73.3%),
  `isValidURL`/`generateRandomHex` (75%), `IssueAuthCode`/`IssueRefreshToken`
  (85.7%), `ValidateToken` (87.5%), `ApproveAuthorize` (88.9%),
  `handleClientCredentialsGrant` (88.9%), `RegisterClient` (91.3%) — add
  missing error/edge-case table rows to existing test files.
- `middleware.AuthMiddleware` (87%), `wallet/repositories.UpdateTopupStatus`
  (97.1%), `wallet/handlers.ListWalletTransactions` (95.2%),
  `router.newRouter` (97.3%), `users/services.RegisterMerchant` (91.7%) — add
  the 1-2 remaining uncovered branches per function.

**Tier 6 — Explicitly deprioritized (documented, not chased)**
- `app/cmd/main.go`, `app/cmd/providers.go` (`main`, `runReconcile`,
  `provideUserRepository`, `initApp` in `wire_gen.go` is excluded already).
  These require a real running process/DI graph. Not unit-testable without
  disproportionate scaffolding (subprocess harness) for the value gained.
  - `ponytail:` skip process-level entrypoints; upgrade path: subprocess/e2e
    smoke test if startup-regression coverage becomes a real need.

### Verification

After each tier: run `make coverage`, confirm the number moves up and no
existing test breaks (`go test ./...` full pass). Final gate: filtered total
>= 90%.

## Testing Standard (all new tests)

- Table-driven structs (`name`, `input`, `want`, `wantErr` fields).
- `require` for setup/precondition assertions that should stop the test on
  failure (e.g., mock setup, non-nil checks before proceeding).
- `assert` for the actual behavior/output checks.
- Reuse existing mock generation (`make mock`) and `sqlmock`/`miniredis`
  patterns already established in the repo — no new test tooling introduced.
