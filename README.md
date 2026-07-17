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
- `APP_ENV=local`
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

## VPS Deployment

The API container listens only on `127.0.0.1:8080`. Host-installed Nginx terminates TLS for `https://api-payment.pikri.my.id` and proxies to that listener. Do not run Nginx in Docker.

The existing Certbot certificate is valid through August 2026. Nginx expects its renewal-managed paths to remain:

```text
/etc/letsencrypt/live/api-payment.pikri.my.id/fullchain.pem
/etc/letsencrypt/live/api-payment.pikri.my.id/privkey.pem
```

First-server setup, run on the VPS except `ssh-keyscan`, which runs on the trusted GitHub Actions runner or administrator workstation:

```bash
sudo install -d -m 0750 -o pik -g pik /home/pik/container/payment-sandbox
sudo docker network inspect postgres_default mongodb_mongodb_default redis_redis_default
ssh-keyscan -H <VPS_HOST> > /tmp/payment-sandbox-known-hosts
sudo cp deploy/nginx/api-payment.pikri.my.id.conf /etc/nginx/sites-available/api-payment.pikri.my.id
sudo ln -s /etc/nginx/sites-available/api-payment.pikri.my.id /etc/nginx/sites-enabled/api-payment.pikri.my.id
sudo nginx -t
sudo systemctl reload nginx
curl -fsS http://127.0.0.1:8080/api/v1/ping
curl -fsS https://api-payment.pikri.my.id/api/v1/ping
```

The deployed Compose file joins these pre-existing external Docker networks exactly: `postgres_default`, `mongodb_mongodb_default`, and `redis_redis_default`.

`payment_sandbox_user` must own database `payment_sandbox` and schema `public`. If ownership differs, the PostgreSQL `root` superuser must execute:

```sql
ALTER DATABASE payment_sandbox OWNER TO payment_sandbox_user;
\c payment_sandbox
ALTER SCHEMA public OWNER TO payment_sandbox_user;
GRANT ALL ON SCHEMA public TO payment_sandbox_user;
```

Configure GitHub repository secrets without committing their values:

- `VPS_HOST`
- `VPS_SSH_PRIVATE_KEY`
- `VPS_SSH_KNOWN_HOSTS` (contents of `/tmp/payment-sandbox-known-hosts`)
- `JWT_SECRET`
- `DB_PASSWORD`
- `MONGO_PASSWORD`
- `GHCR_PULL_TOKEN`

Set optional repository variable `VPS_SSH_PORT`; it defaults to `22` when unset.

Deploy by pushing to `master`; `.github/workflows/deploy-vps.yml` tests, publishes the immutable Git SHA image, uploads the runtime files, and runs `deploy/deploy.sh`.

Runtime `.env` is loaded by Compose through scalar `env_file: .env`. The workflow single-quotes every secret-derived dotenv value, escapes embedded apostrophes as `\'`, and rejects CR/LF secrets. It is never passed with `--env-file`; `.deploy.env` contains only `IMAGE` for Compose interpolation. This preserves literal `$`, `$$`, `${...}`, `#`, and apostrophes in secrets.

Manual rollback: replace the SHA with an already-published image tag, then restart the API:

```bash
printf 'IMAGE=ghcr.io/<repository-owner>/payment-sandbox:<previous-sha>\n' > /home/pik/container/payment-sandbox/.deploy.env
cd /home/pik/container/payment-sandbox
sudo docker compose -f docker-compose.yml --env-file .deploy.env up -d api
curl -fsS http://127.0.0.1:8080/api/v1/ping
```

After copying or changing the Nginx configuration on the VPS, validate the public endpoint:

```bash
sudo nginx -t
sudo systemctl reload nginx
curl -fsS https://api-payment.pikri.my.id/api/v1/ping
```

Expected public response: HTTP `200` with `{"data":{"status":"ok"}}`.

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

Complete API end-to-end tests:

```bash
make test-e2e-api
```

Service-layer coverage snapshot:

```bash
make coverage-services
```

Full Batch 10 verification bundle:

```bash
make verify-batch10
```

Batch 11 reliability/performance verification bundle:

```bash
make verify-batch11
```

ISO readiness verification bundle:

```bash
make verify-iso
```

CI-safe ISO verification bundle (skips strict drill evidence gate unless explicitly enabled):

```bash
make verify-iso-ci
```

Run real backup-restore drill:

```bash
make drill-backup-restore
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
- Submission handoff: `docs/submission-handoff.md`
- ISO hardening package: `docs/iso/README.md`
- Security policy: `SECURITY.md`

## Security and ISO Operations

- Config hardening guide: `docs/security/config-hardening.md`
- Vulnerability handling workflow: `docs/iso/06-vulnerability-management.md`
- Backup helper: `misc/ops/backup.sh`
- Restore helper: `misc/ops/restore.sh`
- Backup-restore drill helper: `misc/ops/drill-backup-restore.sh`
- CI workflow for ISO checks: `.github/workflows/iso-verification.yml`
