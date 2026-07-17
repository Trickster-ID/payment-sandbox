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
