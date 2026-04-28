# Logging and Retention Policy

Last updated: 2026-04-28

## Logging Objectives

- Provide traceability for all critical transaction lifecycle events.
- Support incident investigation and operational debugging.
- Avoid exposing sensitive data in logs.

## Event Taxonomy (Minimum Required)

Runtime action names in journey events:

- `TOPUP_CREATE`
- `TOPUP_STATUS_UPDATE`
- `INVOICE_CREATE`
- `PAYMENT_INTENT_CREATE`
- `PAYMENT_INTENT_STATUS_UPDATE`
- `REFUND_REQUEST`
- `REFUND_REVIEW`
- `REFUND_PROCESS`

Verification command:

```bash
./misc/verify/iso-journey-events.sh
```

## Required Event Fields

- `timestamp` in RFC3339 UTC
- `request_id`
- `actor_id` (user/admin if available)
- `actor_role` (`MERCHANT`, `ADMIN`, `PUBLIC`, or `SYSTEM`)
- `action` (event name)
- `resource_id` (invoice/payment/refund/topup id if available)
- `result` (`success` or `error`)
- `error_code` (optional)

## Sensitive Data Rules

- Do not log passwords or password hashes.
- Do not log JWT tokens or full secrets.
- Avoid logging full connection strings.
- Mask personally identifiable data where full value is not required.

## Date/Time Standard (ISO 8601)

- All log timestamps and datetime payload examples must use RFC3339.
- UTC (`Z`) is the default timezone for API examples and operational logs.

## Currency Standard (ISO 4217)

- Current backend policy is single-currency mode: `IDR`.
- Any amount field in docs/reports is interpreted as `IDR` unless explicitly extended in future API versions.
- If multi-currency support is introduced, API must add explicit `currency_code` using ISO 4217 uppercase 3-letter format.

## Retention and Purge

- Application structured logs: minimum 30 days.
- MongoDB journey logs: minimum 90 days.
- Audit evidence artifacts under `docs/iso/`: retained per release history.
- Purge operations must be documented and reversible through backup strategy.
