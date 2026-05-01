# K6 Performance Tests

Performance test suite for the Payment Sandbox REST API using [K6](https://k6.io).
Every run produces a self-contained HTML report + JSON summary.

---

## Prerequisites

- [K6 ≥ 0.49](https://k6.io/docs/get-started/installation/) installed
- Backend running and reachable
- A first-party OAuth2 client created in the database (for `password` grant)

### Install K6 (macOS)

```bash
brew install k6
```

### Install K6 (Linux)

```bash
sudo gpg --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
echo "deb https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list
sudo apt-get update && sudo apt-get install k6
```

---

## Required Environment Variables

| Variable | Description |
|---|---|
| `K6_BASE_URL` | Backend base URL, e.g. `http://127.0.0.1:8080` |
| `K6_ADMIN_EMAIL` | Admin user email |
| `K6_ADMIN_PASSWORD` | Admin user password |
| `K6_OAUTH2_CLIENT_ID` | OAuth2 first-party client ID |
| `K6_OAUTH2_CLIENT_SECRET` | OAuth2 first-party client secret |

### Optional Variables

| Variable | Default | Description |
|---|---|---|
| `K6_MERCHANT_EMAIL` | _(auto-register)_ | Merchant email; if blank, a new merchant is registered per run |
| `K6_MERCHANT_PASSWORD` | _(auto-register)_ | Merchant password |
| `K6_VUS` | `5` | Number of virtual users |
| `K6_DURATION` | `30s` | Test duration |
| `K6_RAMP_UP` | `10s` | Ramp-up duration |
| `K6_REPORT_DIR` | `docs/k6/reports` | Output directory for HTML/JSON reports |

---

## Quick Start

```bash
# 1. Export required vars
export K6_BASE_URL=http://127.0.0.1:8080
export K6_ADMIN_EMAIL=admin@sandbox.local
export K6_ADMIN_PASSWORD=adminpassword
export K6_OAUTH2_CLIENT_ID=<client-id>
export K6_OAUTH2_CLIENT_SECRET=<client-secret>

# 2. Smoke test (always run this first)
make perf-smoke

# 3. Open the HTML report
make perf-open-last-report
```

---

## Makefile Commands

| Command | Description |
|---|---|
| `make perf-smoke` | 1 VU, 20 s — CI health check |
| `make perf-baseline` | Moderate load — benchmark reference |
| `make perf-stress` | Ramp to 3× VUs — find degradation point |
| `make perf-soak` | Sustained load — detect leaks |
| `make perf-full-coverage` | All endpoints, deterministic sweep |
| `make perf-open-last-report` | Open latest `summary.html` in browser |
| `make perf-clean-reports` | Delete all report files |

Override defaults on the command line:

```bash
make perf-baseline K6_VUS=20 K6_DURATION=2m K6_RAMP_UP=30s
```

---

## Running Scripts Directly

```bash
# Smoke
K6_BASE_URL=... K6_ADMIN_EMAIL=... K6_ADMIN_PASSWORD=... \
  K6_OAUTH2_CLIENT_ID=... K6_OAUTH2_CLIENT_SECRET=... \
  k6 run docs/k6/scripts/smoke.js

# Full coverage
k6 run docs/k6/scripts/full-coverage.js
```

---

## Reports

Each run writes to:

```
docs/k6/reports/<YYYYMMDDHHMMSS>/<profile>/
  summary.json   — raw K6 summary data
  summary.html   — visual report (open in any browser)
```

---

## Endpoint Coverage

### Public
| Endpoint | Scenario |
|---|---|
| `GET /api/v1/ping` | smoke, public |
| `POST /api/v1/users/register` | auth |
| `POST /api/v1/oauth2/token` | auth |
| `POST /api/v1/oauth2/introspect` | auth |
| `POST /api/v1/oauth2/revoke` | auth |
| `GET /api/v1/oauth2/authorize` | auth |
| `GET /api/v1/pay/:token` | public, lifecycle_payment |
| `POST /api/v1/pay/:token/intents` | public, lifecycle_payment, lifecycle_refund |

### Secured (auth, no role)
| Endpoint | Scenario |
|---|---|
| `GET /api/v1/oauth2/userinfo` | auth |

### Merchant
| Endpoint | Scenario |
|---|---|
| `GET /api/v1/merchant/wallet` | merchant, lifecycle_topup |
| `GET /api/v1/merchant/topups` | merchant, lifecycle_topup |
| `POST /api/v1/merchant/topups` | merchant, lifecycle_topup, lifecycle_refund |
| `POST /api/v1/merchant/invoices` | merchant, lifecycle_payment, lifecycle_refund |
| `GET /api/v1/merchant/invoices` | merchant |
| `GET /api/v1/merchant/invoices/:id` | merchant, lifecycle_payment |
| `POST /api/v1/merchant/refunds` | lifecycle_refund |
| `GET /api/v1/merchant/refunds` | merchant, lifecycle_refund |
| `POST /api/v1/merchant/clients` | merchant, lifecycle_oauth2_client |
| `GET /api/v1/merchant/clients` | merchant, lifecycle_oauth2_client |
| `DELETE /api/v1/merchant/clients/:id` | merchant, lifecycle_oauth2_client |

### Admin
| Endpoint | Scenario |
|---|---|
| `GET /api/v1/admin/topups` | admin, lifecycle_topup |
| `PATCH /api/v1/admin/topups/:id/status` | lifecycle_topup, lifecycle_refund |
| `GET /api/v1/admin/payment-intents` | admin, lifecycle_payment |
| `PATCH /api/v1/admin/payment-intents/:id/status` | lifecycle_payment, lifecycle_refund |
| `GET /api/v1/admin/refunds` | admin |
| `PATCH /api/v1/admin/refunds/:id/review` | lifecycle_refund |
| `PATCH /api/v1/admin/refunds/:id/process` | lifecycle_refund |
| `GET /api/v1/admin/stats` | admin |
| `GET /api/v1/admin/ledger/accounts/:merchant_id` | admin |

---

## Thresholds

Defined in `fixtures/thresholds.json`:

| Metric | Threshold |
|---|---|
| `http_req_failed` | `rate < 1%` |
| `http_req_duration` | `p(95) < 300 ms` |
| `checks` | `rate > 99%` |

Stress test uses relaxed thresholds (5% failure, p95 < 1 000 ms).

---

## Project Structure

```
docs/k6/
├── scripts/          Profile entry points (smoke, baseline, stress, soak, full-coverage)
├── scenarios/        Endpoint-grouped and lifecycle scenario functions
├── helpers/          Shared utilities (env, auth, client, checks, idempotency, report)
├── fixtures/         Threshold and profile configuration JSON
└── reports/          Generated reports (gitignored except .gitkeep)
```

---

## CI

The workflow `.github/workflows/perf-smoke.yml` runs `make perf-smoke` automatically.
Requires secrets: `K6_ADMIN_EMAIL`, `K6_ADMIN_PASSWORD`, `K6_OAUTH2_CLIENT_ID`, `K6_OAUTH2_CLIENT_SECRET`.

For heavier tests (baseline/stress/soak/full-coverage), trigger `.github/workflows/perf-smoke.yml`
with `workflow_dispatch` or add separate scheduled workflows.

---

## Recommended Workflow

1. Start backend + DB.
2. Export required env vars.
3. `make perf-smoke` — confirm environment is healthy.
4. `make perf-open-last-report` — review HTML report.
5. If smoke passes: `make perf-full-coverage`.
6. If full-coverage passes: `make perf-baseline`.
7. For regression testing: compare baseline `summary.json` across runs.
8. Log findings in `.agents/performance-generation-progress.md`.
