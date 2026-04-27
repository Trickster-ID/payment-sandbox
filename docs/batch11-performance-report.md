# Batch 11 Performance and Reliability Report

Last updated: 2026-04-27 (WIB)

## Environment

- API run command: `go run ./app/cmd`
- Database: PostgreSQL (local docker setup)
- Timing tool: `curl -w "%{time_total}"`

## Endpoint Response-Time Samples

### `GET /api/v1/ping`

Sample runs:
- 0.003263 s
- 0.000939 s
- 0.000683 s
- 0.000905 s
- 0.000969 s

Observed range: ~0.0007s to ~0.0033s

### `POST /api/v1/auth/login`

Sample runs:
- 0.083810 s
- 0.079587 s
- 0.088413 s
- 0.077012 s
- 0.081728 s

Observed range: ~0.077s to ~0.088s

### `GET /api/v1/admin/stats?start_date=2026-04-01&end_date=2026-04-30`

Sample runs:
- 0.047299 s
- 0.006262 s
- 0.010377 s
- 0.008524 s
- 0.011709 s

Observed range: ~0.006s to ~0.047s

## Result vs NFR Target

Target from requirements: normal API response time <= 300ms.

All sampled endpoints above are below 300ms in this local environment.

## Query Plan Evidence (`EXPLAIN (ANALYZE, BUFFERS)`)

Representative checks were run for key list/aggregate patterns.

### Invoice count/list by merchant

- Query shape: invoice count/list filtered by `merchant_id` + `deleted_at`.
- Execution time observed: ~0.17ms to ~0.30ms.
- Planner used index scans (in this low-cardinality dataset it selected the partial active-invoice index path).

### Admin payment nominal aggregate

- Query shape: `payment_intents` join `invoices`, filter by `status='SUCCESS'` and date range.
- Execution time observed: ~0.18ms.
- Planner used `idx_payment_intents_status_active` for payment-intent status filtering.

### Admin refund nominal aggregate

- Query shape: `refunds` join `payment_intents` join `invoices`, filter by `status='SUCCESS'` and date range.
- Execution time observed: ~1.43ms.
- Planner used indexed join paths across active rows (`payment_intents`/`refunds` partial indexes).

## Notes / Caveats

- The current dataset is small, so planner choices can differ from higher-volume production data.
- Query plan checks should be rerun with larger seeded volumes for stronger evidence under load.
