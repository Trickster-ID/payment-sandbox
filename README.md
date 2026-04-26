# payment-sandbox
## Payment Sandbox Backend (Example)

Backend example based on `Gin` + PostgreSQL with simple clean layering:
- `handler -> service -> repository/store`
- JWT auth + role-based middleware (`MERCHANT`, `ADMIN`)
- Main flows: wallet topup, invoice, payment intent, refund, admin stats
- Transaction journey logging: MongoDB (best-effort, non-blocking for core transactions)

## Run

```bash
go mod tidy
go run ./app/cmd
```

Default env:
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

## Docker Compose

```bash
docker compose up -d
```

Notes:
- Mongo init script at `misc/init-mongo/mongo.js` runs only on first initialization (empty Mongo data directory).
- If you need to re-run initialization, recreate volume:

```bash
docker compose down -v
docker compose up -d
```

## Seeded Admin

- Email: `admin@sandbox.local`
- Password: `admin1234`

## Main Endpoints

Public:
- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `GET /api/v1/pay/:token`
- `POST /api/v1/pay/:token/intents`

Merchant:
- `GET /api/v1/merchant/wallet`
- `POST /api/v1/merchant/topups`
- `POST /api/v1/merchant/invoices`
- `GET /api/v1/merchant/invoices`
- `GET /api/v1/merchant/invoices/:id`
- `POST /api/v1/merchant/refunds`

Admin:
- `GET /api/v1/admin/topups`
- `PATCH /api/v1/admin/topups/:id/status`
- `GET /api/v1/admin/payment-intents`
- `PATCH /api/v1/admin/payment-intents/:id/status`
- `GET /api/v1/admin/refunds`
- `PATCH /api/v1/admin/refunds/:id/review`
- `PATCH /api/v1/admin/refunds/:id/process`
- `GET /api/v1/admin/stats`

## Swagger / OpenAPI

Generate docs:

```bash
go run github.com/swaggo/swag/cmd/swag@v1.8.12 init -g app/cmd/main.go -o docs --parseDependency --parseInternal
```

Or using Makefile:

```bash
make swag
```

Generate mocks:

```bash
make mock
```

Run API:

```bash
go run ./app/cmd
```

Open Swagger UI:

- `http://localhost:8080/swagger/index.html`
