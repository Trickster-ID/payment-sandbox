# Final Review Fixes Report

## Scope

- Fixed only final-review findings: MongoDB URI password encoding and Gin trusted proxies.
- Did not add MongoDB provisioning. Deployment context confirms the MongoDB user exists.

## Changes

- `.github/workflows/deploy-vps.yml`: encodes `MONGO_PASSWORD` with Python stdlib `urllib.parse.quote(..., safe='')` before composing `MONGO_URI`. The encoded password is never printed.
- `app/cmd/router.go`: trusts only `127.0.0.1` and `::1`; router construction fails fast if Gin rejects the static configuration.
- `app/cmd/router_test.go`: table-driven coverage proves untrusted peers cannot supply the client IP through `X-Forwarded-For`; IPv4 and IPv6 loopback proxies can.

## Verification

- `go test ./app/cmd -run 'TestNewRouter_(TrustedProxies|RegistersExpectedRoutes)' -v`: pass.
- `ruby -e 'require "yaml"; YAML.load_file(".github/workflows/deploy-vps.yml")'`: pass.
- Static secret scan rejects shell tracing, environment dumps, direct password interpolation in `MONGO_URI`, and explicit Mongo password output: pass.

## Concern

- The deployment workflow depends on the runner-provided `python3`, present on `ubuntu-latest`.

## Follow-up Final Review Fixes (2026-07-17 WIB)

### Changes

- `.github/workflows/deploy-vps.yml`: pinned `actions/checkout` v4, `actions/setup-go` v5, `docker/login-action` v3, and `docker/build-push-action` v6 to their resolved full commit SHAs. Major-version comments remain for maintenance visibility.
- `app/cmd/router.go`: trusts Docker bridge peers only within `172.16.0.0/12`, plus `127.0.0.1` and `::1`.
- `app/cmd/router_test.go`: added table-driven Docker bridge coverage proving a `172.17.0.1` peer can forward the client IP while a public untrusted peer cannot.
- `deploy/nginx/api-payment.pikri.my.id.conf`: overwrites `X-Forwarded-For` with `$remote_addr`; it does not append a client-supplied chain across the Docker trust boundary.
- Updated stale Nginx deployment plan, brief, and design wording/configuration to match sole-value XFF behavior.

### Verification

- `go test ./app/cmd -run 'TestNewRouter_(TrustedProxies|RegistersExpectedRoutes)' -v`: pass.
- `ruby -e 'require "yaml"; YAML.load_file(".github/workflows/deploy-vps.yml")'`: pass.
- `rg -n '\$proxy_add_x_forwarded_for|X-Forwarded-For.*append|append.*X-Forwarded-For' deploy .superpowers docs .agents --glob '*.{conf,md,yml,yaml}' --glob '!final-fixes-report.md'`: no stale append wording/configuration.
- `git diff --check`: pass.
- Host `nginx -t` is unavailable. `docker run ... nginx:1.27-alpine ... nginx -t` with temporary self-signed files at the configured certificate paths: pass. It emits only existing `listen ... http2` deprecation warnings.

### Concern

- Run `sudo nginx -t` on the VPS before reload. Local validation uses temporary certificates, not the host-managed Let's Encrypt chain.

## Final Deployment Review Fixes (2026-07-17 WIB)

### Changes

- `deploy/docker-compose.yml`: API and schema consume runtime `.env` with `env_file: {path: .env, format: raw}`. Schema exports `PGPASSWORD` from `DB_PASSWORD` inside its container command; Compose no longer interpolates `DB_PASSWORD`.
- `deploy/deploy.sh`: all Compose calls receive only `.deploy.env`, which contains only `IMAGE`. First-deploy rollback removes the unhealthy API before deleting `.deploy.env`; rollback with a prior image restores/restarts it without removing the API.
- `deploy/deploy_test.sh`: validates literal `DB_PASSWORD=pa$ss#word` and `JWT_SECRET=a$b`, raw env-file declarations, schema shell export, no runtime `.env` Compose argument, first-deploy cleanup, and prior-image preservation.
- `README.md`, deployment spec, and deployment plan: document raw runtime environment semantics and correct rollback command.

### Verification

- `sh -n deploy/deploy.sh && sh -n deploy/deploy_test.sh && sh deploy/deploy_test.sh`: pass.
- `rg -n -- '--env-file \\.env|DB_PASSWORD:\\s*\\$\\{' --glob '*.{md,sh,yml,yaml}'`: no stale runtime Compose interpolation or CLI `.env` references.
- `git diff --check`: pass.

### Concern

- Local Docker Compose 5.3.1 rejects the server-supported raw mapping (`services.api.env_file must be a string`), so `docker compose config` must be run on the VPS Docker Compose 5.1.4 before deployment. The focused regression statically enforces the exact production declaration and executes the container-side shell expansion with literal `$` and `#` secrets.
