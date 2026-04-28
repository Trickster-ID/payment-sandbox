# Configuration Hardening Guide

Last updated: 2026-04-28

## Objective

Prevent unsafe runtime configuration in non-local environments.

## Environment Policy

- `APP_ENV` supports: `local`, `dev`, `staging`, `prod`.
- Unknown values are normalized to `local`.

## Mandatory Rules

- In `dev`, `staging`, and `prod`:
  - `JWT_SECRET` must be explicitly set.
  - `JWT_SECRET` must not use insecure defaults (`change-me-in-env`, `supersecretkey`).
  - `JWT_DURATION_MINUTES` must be greater than `0`.
- In `local`:
  - defaults are allowed for ease of development.

## Secret Rotation Minimum

- Rotate JWT secret on schedule (recommended every 90 days) or immediately after compromise suspicion.
- Rotation must include:
  - secret update in deployment environment
  - validation in smoke test
  - deployment note and timestamp

## Verification

```bash
go test ./app/config -v
```

The test suite validates env normalization and non-local secret constraints.
