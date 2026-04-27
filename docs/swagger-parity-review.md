# Swagger Parity Review

Last updated: 2026-04-27 (WIB)

## Scope

Quick parity check between:
- runtime routes (`app/cmd/router.go`, `app/cmd/router_test.go`)
- generated OpenAPI output (`docs/swagger.yaml`)
- swagger annotations in handlers (`@Router` tags)

## Findings

1. Health route mismatch (fixed)
- Runtime route: `GET /api/v1/ping`
- Previous Swagger path: `/healthz`
- Applied fix: updated Swagger docs path to `/ping` in:
  - `docs/swagger.yaml`
  - `docs/swagger.json`
  - `docs/docs.go`

2. Annotation coverage (fixed)
- `@Router` tags are now standardized across auth + non-auth handlers:
  - wallet
  - invoice
  - payment
  - refund
  - admin

## Recommended Fix

1. Update Swagger health path to match runtime:
- replace `/healthz` with `/ping` (base path `/api/v1` remains unchanged).

2. Add/standardize Swagger annotations across remaining handlers.
  - status: completed

3. Regenerate docs and re-verify:

```bash
make swag
go test ./app/cmd -run TestNewRouter_RegistersExpectedRoutes
```

Verification result:
- `make swag` completed successfully.
- `go test ./app/cmd -run TestNewRouter_RegistersExpectedRoutes -v` passed.

## Current Status

- Health-path mismatch is resolved.
- Swagger annotation coverage standardization is complete for current routes.
- Swagger parity is **done** for the reviewed scope.
