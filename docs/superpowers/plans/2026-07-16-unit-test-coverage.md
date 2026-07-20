# Unit Test Coverage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Raise filtered unit-test coverage to at least 90% without production behavior changes.

**Architecture:** Add focused black-box unit tests at existing package boundaries. Use `sqlmock` for repositories, `miniredis` for Redis code, Gin route introspection for registrars, and existing mocks for handlers/services. Filter only generated Swagger and Wire code from the aggregate metric.

**Tech Stack:** Go 1.26, `testify/assert`, `testify/require`, `go-sqlmock`, Gin, `alicebob/miniredis/v2`.

## Global Constraints

- New and changed tests are table-driven and use `require` for setup/preconditions and `assert` for expected behavior.
- Exclude only `docs/docs.go` and `app/cmd/wire_gen.go` from aggregate coverage.
- Do not modify `app/shared/database/*_test.go`; it is complete and optimal.
- `POST /oauth2/authorize` remains secured; `GET /oauth2/authorize` remains public.
- No real Postgres, Redis, or Mongo containers in unit tests.
- No production behavior changes except a minimal pure testability extraction when unavoidable.
- `ponytail:` process bootstraps in `app/cmd/main.go` and `app/cmd/providers.go` remain out of scope; use a subprocess smoke test only if startup coverage becomes required.

---

## File Structure

- `Makefile`: filtered aggregate coverage command.
- `go.mod`, `go.sum`: direct `miniredis` test dependency.
- `app/modules/{payment,refund}/models/entity/*_test.go`: enum parser coverage.
- `app/modules/payment/sagas/payment_saga_test.go`: saga step-name coverage.
- `app/modules/*/api/routes_test.go`: route registrar coverage, including OAuth2 public/secured split.
- `app/modules/ledger/repositories/ledger_repository_test.go`: missing sqlmock repository suite.
- Existing OAuth2/payment/refund repository tests: missing SQL error rows.
- `app/shared/locking/redis_lock_test.go`, `app/shared/idempotency/cache_test.go`: real miniredis behavior tests replacing literal-only assertions.
- `app/shared/idempotency/*_test.go`, `app/shared/audit/*_test.go`: remaining helper branches.
- Existing OAuth2, middleware, wallet, router, and users tests: residual branch rows only.
- `.agents/feature/generation-progress.md`: session handoff.

### Task 1: Coverage Command And Test Dependency

**Files:**
- Modify: `Makefile:71-137`
- Modify: `go.mod`, `go.sum`

**Produces:** `make coverage`, reporting coverage after filtering generated files.

- [ ] **Step 1: Add the dependency**

Run: `go get github.com/alicebob/miniredis/v2`

Expected: `go.mod` lists `github.com/alicebob/miniredis/v2` as a direct dependency.

- [ ] **Step 2: Add the filtered coverage target**

```makefile
.PHONY: coverage

coverage:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	grep -v -E "docs/docs\.go|app/cmd/wire_gen\.go" coverage.out > coverage.filtered.out
	go tool cover -func=coverage.filtered.out | tail -1
```

- [ ] **Step 3: Verify the target**

Run: `make coverage`

Expected: all tests pass; final line reports `total:`.

### Task 2: Pure Values And Route Registrars

**Files:**
- Create: `app/modules/payment/models/entity/payment_entity_test.go`
- Create: `app/modules/refund/models/entity/refund_entity_test.go`
- Modify: `app/modules/payment/sagas/payment_saga_test.go`
- Create: `app/modules/admin/api/routes_test.go`, `app/modules/invoice/api/routes_test.go`, `app/modules/ledger/api/routes_test.go`, `app/modules/merchants/api/routes_test.go`, `app/modules/oauth2/api/routes_test.go`, `app/modules/payment/api/routes_test.go`, `app/modules/refund/api/routes_test.go`, `app/modules/users/api/routes_test.go`, `app/modules/wallet/api/routes_test.go`

**Produces:** parser and route-registration coverage without HTTP/DB execution.

- [ ] **Step 1: Write table-driven parser and saga tests**

```go
tests := []struct {
	name    string
	input   string
	wantErr bool
}{
	{name: "valid", input: "SUCCESS"},
	{name: "unknown", input: "UNKNOWN", wantErr: true},
	{name: "empty", input: "", wantErr: true},
}
for _, tc := range tests {
	t.Run(tc.name, func(t *testing.T) {
		got, err := ParsePaymentStatus(tc.input)
		assert.Equal(t, tc.wantErr, err != nil)
		if !tc.wantErr { assert.Equal(t, tc.input, string(got)) }
	})
}
```

Use each package's real accepted enum strings. Assert every saga `Name()` value exactly.

- [ ] **Step 2: Write registrar tests from `engine.Routes()`**

```go
engine := gin.New()
RegisterRoutes(engine.Group(""), handler)
routes := engine.Routes()
assert.Contains(t, routeKeys(routes), "GET /expected-path")
```

Build a route-key helper in each test file only when needed. Pass handler instances with nil dependencies because registration only captures methods. For OAuth2 call `RegisterPublicRoutes` and `RegisterSecuredRoutes` separately, assert `GET /oauth2/authorize` exists only in public routes and `POST /oauth2/authorize` exists only in secured routes.

- [ ] **Step 3: Verify package tests**

Run: `go test ./app/modules/.../models/entity ./app/modules/payment/sagas ./app/modules/.../api`

Expected: PASS.

### Task 3: Repository SQL Error Coverage

**Files:**
- Create: `app/modules/ledger/repositories/ledger_repository_test.go`
- Modify: `app/modules/oauth2/repositories/oauth2_repository_test.go`
- Modify: `app/modules/payment/repositories/payment_repository_test.go`
- Modify: `app/modules/refund/repositories/refund_repository_test.go`

**Consumes:** existing repository constructors and SQL query shapes.

**Produces:** success and database-error coverage for all specified methods.

- [ ] **Step 1: Add ledger sqlmock cases**

Use the established constructor pattern:

```go
db, mock, err := sqlmock.New()
require.NoError(t, err)
t.Cleanup(func() { assert.NoError(t, db.Close()) })
repository := NewRepository(db)
```

For `Post`, `Reverse`, `ListEntriesByAccount`, and `GetAccountByMerchantID`, add table rows for valid rows/result and `WillReturnError(errors.New("db unavailable"))`. Assert returned domain values or error; call `require.NoError(t, mock.ExpectationsWereMet())` after every case.

- [ ] **Step 2: Add targeted existing repository error rows**

Add missing cases only:

```go
{name: "query error", mock: func(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnError(errors.New("db unavailable"))
}, wantErr: true}
```

Cover OAuth2 `CreateClient`, `ListClientsByOwner`; payment `UpdatePaymentStatus`, `getPaymentIntentByID`, `CreatePaymentIntent`; refund `RequestRefund`, `ReviewRefund`, `ProcessRefund`.

- [ ] **Step 3: Verify repositories**

Run: `go test ./app/modules/ledger/repositories ./app/modules/oauth2/repositories ./app/modules/payment/repositories ./app/modules/refund/repositories`

Expected: PASS.

### Task 4: Redis And Audit Infrastructure

**Files:**
- Modify: `app/shared/locking/redis_lock_test.go`
- Modify: `app/shared/idempotency/cache_test.go`
- Modify: `app/shared/idempotency/store_test.go`
- Modify: `app/shared/idempotency/middleware_test.go`
- Modify: `app/shared/audit/audit_test.go`

**Produces:** real Redis command coverage and audit writer coverage.

- [ ] **Step 1: Replace literal-only Redis tests with miniredis tests**

```go
server := miniredis.RunT(t)
client := redis.NewClient(&redis.Options{Addr: server.Addr()})
t.Cleanup(func() { assert.NoError(t, client.Close()) })
```

Call actual `Acquire` and `Release`; assert first acquire succeeds, repeated acquire reports conflict, release permits a new acquire, and closed-client calls return an error. Call actual cache `Set`/`Get`; assert cache miss, round-trip hit, and closed-client errors.

- [ ] **Step 2: Complete idempotency and audit branches**

Use existing miniredis patterns to cover `middleware.WriteString` and `Handle` outcomes: cache miss proceeds, cache hit replays status/body, incompatible request hash conflicts, and store errors pass through correctly. Construct audit logger with `bytes.Buffer`, call `Log`, assert the serialized line includes the expected action fields. Cover `NewLogger`'s currently untested writer/default branch.

- [ ] **Step 3: Verify shared tests**

Run: `go test ./app/shared/locking ./app/shared/idempotency ./app/shared/audit`

Expected: PASS.

### Task 5: Remaining Branches And Final Gate

**Files:**
- Modify: `app/modules/oauth2/handlers/oauth2_handler_test.go`
- Modify: `app/modules/oauth2/services/oauth2_service_test.go`
- Modify: `app/middleware/middleware_test.go`
- Modify: `app/modules/wallet/repositories/wallet_repository_test.go`
- Modify: `app/modules/wallet/handlers/wallet_handler_endpoints_test.go`
- Modify: `app/cmd/router_test.go`
- Modify: `app/modules/users/services/users_service_test.go`
- Modify: `.agents/feature/generation-progress.md`

**Produces:** coverage for known residual branches; final filtered metric evidence.

- [ ] **Step 1: Add the OAuth2 refresh validation error row**

In `TestOAuth2Handler_HandleToken`, add the `refresh_token` grant case whose mocked `ValidateClient` returns an error. Assert the handler's existing error status/envelope. This specifically covers `handleRefreshTokenGrant` lines 354-357.

- [ ] **Step 2: Add remaining table rows by actual branch**

Add one focused row per uncovered branch in `isValidURL`, `generateRandomHex`, `IssueAuthCode`, `IssueRefreshToken`, `ValidateToken`, `ApproveAuthorize`, `handleClientCredentialsGrant`, `RegisterClient`, auth middleware, wallet update/list, router, and merchant registration. Reuse current fixtures and mocks. Do not alter production behavior.

- [ ] **Step 3: Run final coverage gate**

Run: `make coverage && go test ./...`

Expected: both commands pass; filtered `total:` is at least `90.0%`.

- [ ] **Step 4: Update handoff**

Record date/time, Batch 10 coverage workstream, measured filtered percentage, commands run, deferred process-entrypoint coverage, and all changed files in `.agents/feature/generation-progress.md`.
