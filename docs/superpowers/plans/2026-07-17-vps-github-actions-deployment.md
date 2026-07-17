# VPS GitHub Actions Deployment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Automatically publish and deploy the API to the VPS after every push to `master`, using an immutable GHCR image.

**Architecture:** GitHub Actions tests and publishes `ghcr.io/<repository-owner>/payment-sandbox:<commit-sha>`. It uploads the deployment inputs to `/home/pik/container/payment-sandbox/`, then invokes a remote script which pulls the immutable image, applies idempotent SQL through a temporary PostgreSQL client container, recreates the API, and rolls back the previous image if loopback health fails. Host Nginx proxies the public API domain to the loopback-only API port.

**Tech Stack:** Go 1.26, Docker multi-stage build, Docker Compose v5, GitHub Actions, GHCR, OpenSSH/SCP, PostgreSQL 18 client, host Nginx.

## Global Constraints

- Trigger only on push to `master`; pull requests run no deployment.
- Run `go test ./...` before any image push.
- Push and deploy immutable commit-SHA image tags; `latest` is informational only.
- Docker commands on VPS run through `sudo` as user `pik`.
- API compose project is `/home/pik/container/payment-sandbox/`.
- Use external Docker network keys and names `postgres_default`, `mongodb_mongodb_default`, `redis_redis_default`.
- API hostnames are `postgres`, `mongodb`, and `redis`; no database port is published by this project.
- Bind API `8080` to `127.0.0.1` only; Nginx serves `https://api-payment.pikri.my.id`.
- Never commit secrets or bake `.env` files into images. Runtime `.env` mode must be `0600`.
- Schema input is `misc/init-sql/01_core.sql` through `misc/init-sql/04_saga.sql`; execute in lexical order with `payment_sandbox_user` after verifying it owns the database and can create objects in `public`.
- Existing unit tests remain table-driven and use `testify/assert` and `testify/require`.
- Do not add a new application dependency.

---

## File Structure

- Modify: `Dockerfile` - production image without environment files, non-root runtime.
- Modify: `app/middleware/cors_middleware.go` - permit the deployed frontend origin only.
- Modify: `app/middleware/cors_middleware_test.go` - assert the deployed CORS policy.
- Create: `deploy/docker-compose.yml` - VPS API and one-shot schema client services.
- Create: `deploy/deploy.sh` - remote pull, schema, health, rollback workflow.
- Create: `deploy/nginx/api-payment.pikri.my.id.conf` - host Nginx virtual host.
- Create: `.github/workflows/deploy-vps.yml` - test, GHCR publish, SCP, SSH deploy workflow.
- Modify: `README.md` - secret inventory, first-server setup, Nginx activation, deployment verification, rollback command.

### Task 1: Harden Runtime Image And Browser Origin

**Files:**
- Modify: `Dockerfile:13-25`
- Modify: `app/middleware/cors_middleware.go:5-20`
- Modify: `app/middleware/cors_middleware_test.go:15-100`

**Interfaces:**
- Consumes: environment variables injected by Docker Compose at container runtime.
- Produces: image that starts `/app/myapp` without an image-baked `.env`; CORS responses limited to `https://payment.pikri.my.id`.

- [ ] **Step 1: Write the failing CORS policy case**

Add this table row to `TestCORSMiddleware` in `app/middleware/cors_middleware_test.go` and retain existing table-driven test structure:

```go
{
    name: "4. trusted frontend origin is returned instead of wildcard",
    method: http.MethodGet,
    wantStatus: http.StatusOK,
    wantHeaders: map[string]string{
        "Access-Control-Allow-Origin":      "https://payment.pikri.my.id",
        "Access-Control-Allow-Credentials": "true",
    },
},
```

Update the existing expected `Access-Control-Allow-Origin` value from `"*"` to `"https://payment.pikri.my.id"` for all applicable cases.

- [ ] **Step 2: Run the focused test and verify failure**

Run: `go test ./app/middleware -run TestCORSMiddleware -v`

Expected: FAIL because the middleware currently emits `Access-Control-Allow-Origin: *`.

- [ ] **Step 3: Implement the minimum CORS policy**

Replace the wildcard origin assignment in `app/middleware/cors_middleware.go` with:

```go
c.Writer.Header().Set("Access-Control-Allow-Origin", "https://payment.pikri.my.id")
```

Keep authorization, idempotency, methods, and preflight handling unchanged. Do not add origin configuration, reflection, or a CORS package: the project has one known frontend origin.

- [ ] **Step 4: Remove image-baked environment configuration**

Replace `Dockerfile` from `FROM scratch` onward with:

```dockerfile
FROM scratch

WORKDIR /app

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/myapp /app/myapp

USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/app/myapp"]
```

Delete `ARG APP_ENV` and `COPY --from=builder /app/.env.${APP_ENV} /app/.env`. Docker Compose will provide `.env` at runtime.

- [ ] **Step 5: Verify test and image behavior**

Run: `go test ./app/middleware -run TestCORSMiddleware -v`

Expected: PASS.

Run: `docker build -t payment-sandbox:deployment-test .`

Expected: successful image build; no `COPY` failure for `.env.prod`.

- [ ] **Step 6: Commit**

```bash
git add Dockerfile app/middleware/cors_middleware.go app/middleware/cors_middleware_test.go
git commit -m "fix(deploy): use runtime env and trusted frontend origin"
```

### Task 2: Add VPS Compose And Transactional Deploy Script

**Files:**
- Create: `deploy/docker-compose.yml`
- Create: `deploy/deploy.sh`
- Uses at deploy time: `misc/init-sql/01_core.sql`, `misc/init-sql/02_idempotency.sql`, `misc/init-sql/03_ledger.sql`, `misc/init-sql/04_saga.sql`

**Interfaces:**
- Consumes: `/home/pik/container/payment-sandbox/.env`, `.deploy.env`, `.ghcr-token`, `misc/init-sql/*.sql`.
- Consumes: `IMAGE` in `.deploy.env`, format `ghcr.io/<owner>/payment-sandbox:<commit-sha>`.
- Produces: API container `payment-sandbox-api`; schema client profile `schema`; `deploy.sh <immutable-image>` exit status.

- [ ] **Step 1: Create compose definition**

Create `deploy/docker-compose.yml`:

```yaml
services:
  api:
    image: ${IMAGE:?IMAGE must be set in .deploy.env}
    container_name: payment-sandbox-api
    restart: unless-stopped
    env_file:
      - .env
    ports:
      - "127.0.0.1:8080:8080"
    networks:
      - postgres_default
      - mongodb_mongodb_default
      - redis_redis_default
    read_only: true
    tmpfs:
      - /tmp
    security_opt:
      - no-new-privileges:true

  schema:
    image: postgres:18.3-alpine3.23
    profiles:
      - schema
    environment:
      PGHOST: postgres
      PGPORT: "5432"
      PGDATABASE: payment_sandbox
      PGUSER: payment_sandbox_user
      PGPASSWORD: ${DB_PASSWORD:?DB_PASSWORD must be set in .env}
    volumes:
      - ./misc/init-sql:/sql:ro
    networks:
      - postgres_default
    command:
      - /bin/sh
      - -ec
      - |
        for file in /sql/*.sql; do
          psql -v ON_ERROR_STOP=1 -f "$file"
        done

networks:
  postgres_default:
    external: true
  mongodb_mongodb_default:
    external: true
  redis_redis_default:
    external: true
```

- [ ] **Step 2: Create remote deployment script**

Create executable `deploy/deploy.sh`:

```sh
#!/usr/bin/env sh
set -eu

deploy_dir=/home/pik/container/payment-sandbox
image=${1:?usage: deploy.sh <immutable-image>}

case "$image" in
  ghcr.io/*:*) ;;
  *) echo "image must be an immutable GHCR tag" >&2; exit 64 ;;
esac

cd "$deploy_dir"
test -f .env
test -f .ghcr-token
test -d misc/init-sql

previous_image=""
if [ -f .deploy.env ]; then
  previous_image=$(sed -n 's/^IMAGE=//p' .deploy.env)
fi

umask 077
printf 'IMAGE=%s\n' "$image" > .deploy.env.next
mv .deploy.env.next .deploy.env

registry=$(printf '%s' "$image" | cut -d/ -f1)
username=$(printf '%s' "$image" | cut -d/ -f2)
sudo docker login "$registry" --username "$username" --password-stdin < .ghcr-token
rm -f .ghcr-token

rollback() {
  status=$?
  if [ "$status" -ne 0 ] && [ -n "$previous_image" ]; then
    printf 'IMAGE=%s\n' "$previous_image" > .deploy.env
    sudo docker compose -f docker-compose.yml --env-file .env --env-file .deploy.env up -d api || true
  fi
  exit "$status"
}
trap rollback EXIT

sudo docker compose -f docker-compose.yml --env-file .env --env-file .deploy.env pull api
sudo docker compose -f docker-compose.yml --env-file .env --env-file .deploy.env --profile schema run --rm schema
sudo docker compose -f docker-compose.yml --env-file .env --env-file .deploy.env up -d --remove-orphans api

attempt=0
until curl -fsS http://127.0.0.1:8080/api/v1/ping >/dev/null; do
  attempt=$((attempt + 1))
  if [ "$attempt" -eq 30 ]; then
    echo "API health check failed" >&2
    exit 1
  fi
  sleep 2
done

trap - EXIT
```

Run: `chmod +x deploy/deploy.sh`.

`postgres:18.3-alpine3.23` must match the PostgreSQL server major version because `01_core.sql` uses `uuidv7()`.

- [ ] **Step 3: Validate compose syntax without secrets**

Run:

```bash
mkdir -p /tmp/payment-sandbox-deploy-check
cp deploy/docker-compose.yml /tmp/payment-sandbox-deploy-check/docker-compose.yml
printf 'IMAGE=ghcr.io/example/payment-sandbox:0123456789abcdef\nDB_PASSWORD=test\n' > /tmp/payment-sandbox-deploy-check/.env
docker compose -f /tmp/payment-sandbox-deploy-check/docker-compose.yml --env-file /tmp/payment-sandbox-deploy-check/.env config >/dev/null
```

Expected: exit `0`. The command validates interpolation only; it must not start containers.

- [ ] **Step 4: Commit**

```bash
git add deploy/docker-compose.yml deploy/deploy.sh
git commit -m "feat(deploy): add VPS compose deployment"
```

### Task 3: Add GitHub Actions Publish And Remote Deploy

**Files:**
- Create: `.github/workflows/deploy-vps.yml`

**Interfaces:**
- Consumes secrets: `VPS_HOST`, `VPS_SSH_PRIVATE_KEY`, `VPS_SSH_KNOWN_HOSTS`, `JWT_SECRET`, `DB_PASSWORD`, `MONGO_PASSWORD`, `GHCR_PULL_TOKEN`.
- Consumes variables: `VPS_SSH_PORT` (default `22`).
- Produces: immutable image `${{ github.sha }}`, uploaded remote `.env`, `.ghcr-token`, compose, script, and SQL files.

- [ ] **Step 1: Create workflow**

Create `.github/workflows/deploy-vps.yml`:

```yaml
name: Deploy VPS

on:
  push:
    branches:
      - master

permissions:
  contents: read
  packages: write

concurrency:
  group: payment-sandbox-production
  cancel-in-progress: false

jobs:
  test-build-deploy:
    runs-on: ubuntu-latest
    env:
      IMAGE_NAME: ghcr.io/${{ github.repository_owner }}/payment-sandbox
      VPS_PORT: ${{ vars.VPS_SSH_PORT || '22' }}
      VPS_TARGET: /home/pik/container/payment-sandbox
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Test
        run: go test ./...

      - name: Login to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and publish immutable image
        uses: docker/build-push-action@v6
        with:
          context: .
          push: true
          tags: |
            ${{ env.IMAGE_NAME }}:${{ github.sha }}
            ${{ env.IMAGE_NAME }}:latest

      - name: Prepare deployment files
        env:
          JWT_SECRET: ${{ secrets.JWT_SECRET }}
          DB_PASSWORD: ${{ secrets.DB_PASSWORD }}
          MONGO_PASSWORD: ${{ secrets.MONGO_PASSWORD }}
          GHCR_PULL_TOKEN: ${{ secrets.GHCR_PULL_TOKEN }}
        run: |
          umask 077
          mkdir -p .deploy-upload/misc
          cp deploy/docker-compose.yml deploy/deploy.sh .deploy-upload/
          cp -R misc/init-sql .deploy-upload/misc/init-sql
          cat > .deploy-upload/.env <<EOF
          APP_ENV=prod
          APP_PORT=8080
          JWT_SECRET=${JWT_SECRET}
          JWT_DURATION_MINUTES=60
          SHUTDOWN_TIMEOUT_SECONDS=10
          DB_HOST=postgres
          DB_PORT=5432
          DB_USER=payment_sandbox_user
          DB_PASSWORD=${DB_PASSWORD}
          DB_NAME=payment_sandbox
          DB_SSLMODE=disable
          MONGO_URI=mongodb://payment_sandbox_user:${MONGO_PASSWORD}@mongodb:27017/payment_sandbox?authSource=payment_sandbox
          MONGO_DB_NAME=payment_sandbox
          MONGO_JOURNEY_ENABLE=true
          REDIS_URL=redis://redis:6379
          OAUTH2_ACCESS_TOKEN_DURATION_MINUTES=15
          OAUTH2_REFRESH_TOKEN_DURATION_DAYS=30
          OAUTH2_AUTH_CODE_DURATION_MINUTES=10
          EOF
          printf '%s' "$GHCR_PULL_TOKEN" > .deploy-upload/.ghcr-token

      - name: Configure SSH
        env:
          VPS_SSH_PRIVATE_KEY: ${{ secrets.VPS_SSH_PRIVATE_KEY }}
          VPS_SSH_KNOWN_HOSTS: ${{ secrets.VPS_SSH_KNOWN_HOSTS }}
        run: |
          umask 077
          mkdir -p ~/.ssh
          printf '%s\n' "$VPS_SSH_PRIVATE_KEY" > ~/.ssh/id_ed25519
          printf '%s\n' "$VPS_SSH_KNOWN_HOSTS" > ~/.ssh/known_hosts

      - name: Upload and deploy
        env:
          VPS_HOST: ${{ secrets.VPS_HOST }}
        run: |
          ssh -i ~/.ssh/id_ed25519 -p "$VPS_PORT" pik@"$VPS_HOST" "mkdir -p $VPS_TARGET/misc"
          scp -i ~/.ssh/id_ed25519 -P "$VPS_PORT" .deploy-upload/docker-compose.yml .deploy-upload/deploy.sh .deploy-upload/.env .deploy-upload/.ghcr-token pik@"$VPS_HOST":"$VPS_TARGET"/
          scp -i ~/.ssh/id_ed25519 -P "$VPS_PORT" -r .deploy-upload/misc/init-sql pik@"$VPS_HOST":"$VPS_TARGET"/misc/
          ssh -i ~/.ssh/id_ed25519 -p "$VPS_PORT" pik@"$VPS_HOST" "chmod 600 $VPS_TARGET/.env $VPS_TARGET/.ghcr-token && chmod 700 $VPS_TARGET/deploy.sh && $VPS_TARGET/deploy.sh $IMAGE_NAME:${{ github.sha }}"
```

The SSH known-host secret contains the exact `ssh-keyscan` output captured from the trusted VPS during setup. Do not replace it with `StrictHostKeyChecking=no`.

- [ ] **Step 2: Review secret exposure paths**

Run: `git diff --check .github/workflows/deploy-vps.yml`

Expected: exit `0`.

Confirm the workflow has no `set -x`, `printenv`, `cat .deploy-upload/.env`, or command that prints `.ghcr-token`.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/deploy-vps.yml
git commit -m "ci: deploy master to VPS through GHCR"
```

### Task 4: Add Nginx Configuration And Operations Runbook

**Files:**
- Create: `deploy/nginx/api-payment.pikri.my.id.conf`
- Modify: `README.md:16-75,156-175`

**Interfaces:**
- Consumes: API listener `127.0.0.1:8080`, certificate paths under `/etc/letsencrypt/live/api-payment.pikri.my.id/`.
- Produces: Nginx HTTPS proxy for `api-payment.pikri.my.id` and repeatable first-deployment instructions.

- [ ] **Step 1: Create Nginx server configuration**

Create `deploy/nginx/api-payment.pikri.my.id.conf`:

```nginx
server {
    listen 80;
    listen [::]:80;
    server_name api-payment.pikri.my.id;

    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name api-payment.pikri.my.id;

    ssl_certificate /etc/letsencrypt/live/api-payment.pikri.my.id/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api-payment.pikri.my.id/privkey.pem;

    client_max_body_size 1m;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $remote_addr;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Request-ID $http_x_request_id;
        proxy_connect_timeout 5s;
        proxy_read_timeout 60s;
    }
}
```

- [ ] **Step 2: Document first-server setup and operations**

Append a `## VPS Deployment` section to `README.md` containing these commands and checks:

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

Document that `payment_sandbox_user` must own database `payment_sandbox` and schema `public`; otherwise the PostgreSQL `root` superuser must run:

```sql
ALTER DATABASE payment_sandbox OWNER TO payment_sandbox_user;
\c payment_sandbox
ALTER SCHEMA public OWNER TO payment_sandbox_user;
GRANT ALL ON SCHEMA public TO payment_sandbox_user;
```

Document GitHub secrets exactly: `VPS_HOST`, `VPS_SSH_PRIVATE_KEY`, `VPS_SSH_KNOWN_HOSTS`, `JWT_SECRET`, `DB_PASSWORD`, `MONGO_PASSWORD`, `GHCR_PULL_TOKEN`; document optional repository variable `VPS_SSH_PORT` defaulting to `22`.

Document manual rollback, replacing the SHA with an already-published tag:

```bash
printf 'IMAGE=ghcr.io/<repository-owner>/payment-sandbox/<previous-sha>\n' > /home/pik/container/payment-sandbox/.deploy.env
cd /home/pik/container/payment-sandbox
sudo docker compose -f docker-compose.yml --env-file .env --env-file .deploy.env up -d api
curl -fsS http://127.0.0.1:8080/api/v1/ping
```

- [ ] **Step 3: Validate host configuration on VPS**

Run on VPS after copying the Nginx file:

```bash
sudo nginx -t
sudo systemctl reload nginx
curl -fsS https://api-payment.pikri.my.id/api/v1/ping
```

Expected: Nginx test passes; public ping returns HTTP `200` with `{"data":{"status":"ok"}}`.

- [ ] **Step 4: Commit**

```bash
git add deploy/nginx/api-payment.pikri.my.id.conf README.md
git commit -m "docs(deploy): document VPS and Nginx setup"
```

### Task 5: End-To-End Deployment Validation

**Files:**
- Modify only if a defect is observed in files created by Tasks 1-4.

**Interfaces:**
- Consumes: repository secrets, GitHub Actions workflow, VPS Docker networks, Nginx config, GHCR package.
- Produces: successful `Deploy VPS` workflow and public API health response.

- [ ] **Step 1: Set GitHub repository settings**

Create the seven named GitHub Actions secrets from Task 4. Set `VPS_SSH_PORT` only if SSH does not use port `22`.

Ensure the GHCR package grants the repository read access for the VPS pull token and repository write access for the GitHub Actions workflow token.

- [ ] **Step 2: Apply host Nginx configuration**

Run Task 4's Nginx commands on the VPS. Ensure the TLS certificate paths exist before reloading Nginx.

- [ ] **Step 3: Trigger deployment**

Merge/push the committed work to `master`.

Expected GitHub Actions sequence: `Test` passes, image push succeeds, `Upload and deploy` exits `0`.

- [ ] **Step 4: Verify production dependencies and public API**

Run on VPS:

```bash
sudo docker ps --filter name=payment-sandbox-api
sudo docker logs --tail 100 payment-sandbox-api
curl -fsS http://127.0.0.1:8080/api/v1/ping
curl -fsS -H 'Origin: https://payment.pikri.my.id' -D - https://api-payment.pikri.my.id/api/v1/ping -o /dev/null
```

Expected: API container is running; both health checks return `200`; public response includes `Access-Control-Allow-Origin: https://payment.pikri.my.id`.

- [ ] **Step 5: Verify first schema initialization**

Run on VPS:

```bash
sudo docker run --rm --network postgres_default -e PGPASSWORD='<application-password>' postgres:18.3-alpine3.23 psql -h postgres -U payment_sandbox_user -d payment_sandbox -c '\dt'
```

Expected: tables including `users`, `invoices`, `idempotency_records`, `accounts`, and `sagas` exist.

- [ ] **Step 6: Commit any defect correction**

If validation required a source correction:

```bash
git add Dockerfile app/middleware deploy .github/workflows/deploy-vps.yml README.md
git commit -m "fix(deploy): correct production deployment validation"
```

If no source correction was needed, make no empty commit.

## Plan Self-Review

- Spec coverage: Tasks 1-5 cover runtime secret isolation, GHCR SHA images, external Docker networks, schema initialization, SSH deployment, health checks, rollback, Nginx proxying, CORS, and verification.
- Explicit non-goals: frontend publication, database infrastructure management, TLS issuance, backups, and destructive schema rollback remain out of scope.
- Consistency: compose `IMAGE` is persisted in `.deploy.env`; `DB_PASSWORD` is loaded from runtime `.env`; workflow invokes `deploy.sh` with matching SHA; API binds `127.0.0.1:8080`; Nginx proxies that endpoint.
- No placeholders: intentionally uses GitHub context expressions for image owner/SHA and documented command substitution for the previously published SHA.
