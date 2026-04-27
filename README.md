# payment-sandbox
## Payment Sandbox Backend (Example)

Backend example based on `Gin` + PostgreSQL with simple clean layering:
- `handler -> service -> repository/store`
- JWT auth + role-based middleware (`MERCHANT`, `ADMIN`)
- Main flows: wallet topup, invoice, payment intent, refund, admin stats
- Transaction journey logging: MongoDB (best-effort, non-blocking for core transactions)

## Prerequisites

- Go `1.24+`
- Docker + Docker Compose
- PostgreSQL client (`psql`) for manual schema initialization (optional if DB already initialized)

## Environment

Copy example env:

```bash
cp .env.example .env
```

Default env (local):

- `APP_PORT=8080`
- `DB_HOST=127.0.0.1`
- `DB_PORT=5432`
- `DB_USER=root`
- `DB_PASSWORD=secretpassword`
- `DB_NAME=payment_sandbox`
- `DB_SSLMODE=disable`
- `JWT_SECRET=supersecretkey`
- `JWT_DURATION_MINUTES=60`
- `SHUTDOWN_TIMEOUT_SECONDS=10`
- `MONGO_URI=mongodb://mongo_user:mongo_password@127.0.0.1:27017/?authSource=admin`
- `MONGO_DB_NAME=payment_sandbox`
- `MONGO_COLLECTION=journey_logs`
- `MONGO_JOURNEY_ENABLE=true`

## Database and Services Setup

Start dependencies:

```bash
docker compose up -d
```

Initialize PostgreSQL schema (idempotent):

```bash
PGPASSWORD=secretpassword psql -h 127.0.0.1 -p 5432 -U root -d payment_sandbox -f misc/init-sql/init-database.sql
```

Notes:
- Mongo init script at `misc/init-mongo/mongo.js` runs only on first initialization (empty Mongo data directory).
- If you need to re-run initialization, recreate volume:

```bash
docker compose down -v
docker compose up -d
```

## Run API

```bash
go mod tidy
go run ./app/cmd
```

Open Swagger UI:
- `http://localhost:8080/swagger/index.html`

## Seeded Admin Account

- Email: `admin@sandbox.local`
- Password: `admin1234`

## Verification and Test Commands

Unit + integration bundle:

```bash
go test ./...
```

DB-backed integration tests only:

```bash
make test-integration
```

Service-layer coverage snapshot:

```bash
make coverage-services
```

Full Batch 10 verification bundle:

```bash
make verify-batch10
```

Generate mocks:

```bash
make mock
```

## Swagger / OpenAPI

Generate docs (direct command):

```bash
go run github.com/swaggo/swag/cmd/swag@v1.8.12 init -g app/cmd/main.go -o docs --parseDependency --parseInternal
```

Or via Makefile:

```bash
make swag
```

## Delivery Artifacts

- API contract: `docs/api-contract-v1.md`
- Requirement gap tracker: `docs/requirement-gap.md`
- Batch 10 test report: `docs/batch10-test-report.md`
- Batch 11 performance report: `docs/batch11-performance-report.md`
- Swagger parity review: `docs/swagger-parity-review.md`
- Backend acceptance checklist: `docs/backend-acceptance-checklist.md`
