# VPS GitHub Actions Deployment Design

## Scope

Deploy the Payment Sandbox API to the existing Ubuntu 22.04 VPS whenever a commit reaches `master`.

- API domain: `https://api-payment.pikri.my.id`
- Frontend domain: `https://payment.pikri.my.id`
- Container runtime: Docker Compose, executed through `sudo`
- Reverse proxy: host-installed Nginx
- Image registry: GitHub Container Registry (GHCR)

This scope excludes frontend deployment, database container management, TLS certificate provisioning, and database backup automation.

## Architecture

GitHub Actions runs `go test ./...`, builds the API image once, then pushes it to GHCR under both the immutable commit SHA and `latest` tags. The VPS deployment always references the immutable SHA tag supplied by the workflow.

The VPS compose project resides at `/home/pik/container/payment-sandbox/`. It runs only the API container and joins these pre-existing Docker networks as external networks:

- `postgres_default`
- `mongodb_mongodb_default`
- `redis_redis_default`

The compose network keys and API Docker DNS hostnames are:

- PostgreSQL: `postgres_default`, `postgres:5432`
- MongoDB: `mongodb_mongodb_default`, `mongodb:27017`
- Redis: `redis_redis_default`, `redis:6379`

Nginx on the host terminates HTTPS for `api-payment.pikri.my.id` and proxies to the API's loopback-bound port. The container port must not be publicly exposed.

## Runtime Configuration

The image must contain only the binary and CA certificates. It must not copy `.env` files or credentials during its build.

The VPS owns `/home/pik/container/payment-sandbox/.env`, mode `0600`, read by Docker Compose at runtime through `env_file: {path: .env, format: raw}`. GitHub Actions writes or updates it from GitHub Actions secrets during deployment without logging values. `.deploy.env` contains only `IMAGE` and is the sole Compose CLI `--env-file`, preventing Compose interpolation of runtime secrets.

Required runtime settings:

- Application: `APP_ENV`, `APP_PORT`, `JWT_SECRET`, JWT/OAuth durations, shutdown timeout.
- PostgreSQL: `DB_HOST=postgres`, `DB_PORT=5432`, `DB_USER=payment_sandbox_user`, `DB_PASSWORD`, `DB_NAME=payment_sandbox`, `DB_SSLMODE=disable`.
- MongoDB: credentialed `MONGO_URI` with `authSource=payment_sandbox`, `MONGO_DB_NAME=payment_sandbox`, journey logging enabled.
- Redis: `REDIS_URL=redis://redis:6379`.

Credentials remain GitHub secrets and VPS runtime configuration. No credential is committed in repository files, images, workflow output, or logs.

## PostgreSQL Initialization

The PostgreSQL database and application user already exist but have no schema. Before the first automated deployment, an operator verifies that `payment_sandbox_user` owns `payment_sandbox` and can create objects in `public`; a root-owned database must be granted that access once.

Before the first API start, deployment applies `misc/init-sql/01_core.sql` through `misc/init-sql/04_saga.sql` in filename order. A one-shot PostgreSQL client container joins `postgres_default`, receives the raw runtime `.env`, and exports `PGPASSWORD` from `DB_PASSWORD` inside its shell before connecting as `payment_sandbox_user`; that user must own `payment_sandbox` and its `public` schema. Normal API runtime uses the same application user.

Subsequent releases run the same idempotent initialization command before updating the API container. A schema failure stops deployment before the new application container starts.

## GitHub Actions Deployment Flow

Trigger: push to `master`.

1. Checkout source and run `go test ./...`.
2. Authenticate to GHCR with GitHub's workflow token.
3. Build the Docker image once; push `<image>:<commit-sha>` and `<image>:latest`.
4. SSH to the VPS as `pik` using the deployment key stored in GitHub secrets.
5. Create/update runtime `.env` without printing secrets.
6. Pull the immutable SHA image with `sudo docker compose pull`.
7. Apply idempotent PostgreSQL schema initialization.
8. Start/recreate the API with `sudo docker compose up -d`.
9. Poll the loopback health endpoint `/api/v1/ping`.
10. Fail the workflow if schema initialization, container startup, or health verification fails.

The VPS authenticates to GHCR with a read-only package token stored locally or supplied through deployment secrets. The deployment user requires narrowly scoped passwordless `sudo` permission for the required Docker commands.

## Rollback

The running image reference is recorded before deployment. If API health verification fails with a prior image, the deploy script restores that immutable image reference and runs `sudo docker compose up -d` again. On a first deployment, it stops and removes the unhealthy API before deleting `.deploy.env`.

Database initialization is additive/idempotent only. This deployment does not attempt automatic database rollback. A destructive or non-backward-compatible schema migration requires a separate, reviewed migration plan.

## Nginx

An Nginx server block for `api-payment.pikri.my.id` proxies HTTPS traffic to the loopback API listener. It forwards `Host`, `X-Real-IP`, `X-Forwarded-Proto`, and `X-Request-ID`, and replaces `X-Forwarded-For` with the direct client address so it is the sole trusted value across the Docker boundary.

The API listener binds to `127.0.0.1` only. PostgreSQL, MongoDB, Redis, and the application container do not require public port publishing.

## Failure Handling

- Test/build/push failure: no VPS action.
- SSH failure: current production container remains unchanged.
- Schema initialization failure: new container is not started.
- API health failure: workflow reports failure and restores previous image.
- MongoDB journey logging outage: existing application behavior remains best-effort; core API transactions continue.

## Verification

- CI: `go test ./...` passes before image publication.
- Deploy: API container is running and `curl -fsS http://127.0.0.1:<app-port>/api/v1/ping` passes.
- Public: `curl -fsS https://api-payment.pikri.my.id/api/v1/ping` passes after Nginx proxying.
- First deployment: verify PostgreSQL schema objects exist and MongoDB journey events are written after one API flow.

## Required Secrets

- `VPS_HOST`
- `VPS_SSH_PRIVATE_KEY`
- `VPS_SSH_PORT` if non-default
- `JWT_SECRET`
- `DB_PASSWORD`
- `MONGO_PASSWORD`
- `GHCR_PULL_TOKEN`

Database usernames, hostnames, database names, Redis URL, image path, and deployment directory are non-secret configuration committed as deploy templates or workflow variables.

The PostgreSQL and MongoDB passwords were shared in chat while designing this deployment. Treat them as exposed and rotate both before production deployment; store replacements only in GitHub Actions secrets and VPS runtime configuration.
