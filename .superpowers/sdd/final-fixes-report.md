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
