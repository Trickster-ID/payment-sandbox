Status: DONE
Commit SHA(s): 8ebff5d
Test result: `docker compose -f /var/folders/41/xc2gytfj04d1t5xk3dzq1wfh0000gp/T/opencode/payment-sandbox-deploy-check/docker-compose.yml --env-file /var/folders/41/xc2gytfj04d1t5xk3dzq1wfh0000gp/T/opencode/payment-sandbox-deploy-check/.env config >/dev/null` and `sh -n deploy/deploy.sh` passed.
Concerns: Remote deployment requires existing external Docker networks, runtime `.env`, GHCR pull token, database ownership, and schema privileges.

Review-fix result: `sh -n deploy/deploy.sh`, `sh -n deploy/deploy_test.sh`, and `sh deploy/deploy_test.sh` passed. The no-Docker test uses temporary fake `sudo`, `docker`, and `curl` binaries; it verifies failed GHCR login restores an existing `.deploy.env`, removes `.deploy.env` when none existed, and rejects `:latest`.
Review-fix concern: `DEPLOY_DIR` is a test-only override; production default remains `/home/pik/container/payment-sandbox`.

Compatibility fix result: `deploy/docker-compose.yml` loads runtime `.env` with scalar `env_file: .env` for API and schema. The workflow writes every secret-derived value as single-quoted dotenv, escapes embedded apostrophes as `\'`, rejects CR/LF, and percent-encodes the Mongo password before embedding `MONGO_URI`. Schema derives `PGPASSWORD` inside its container shell. `deploy.sh` supplies only `.deploy.env`; it contains only `IMAGE`. The regression executes workflow dotenv generation, rejects a CR/LF secret, verifies `docker compose config`, and verifies container runtime round-trip for `$`, `$$`, `${...}`, `#`, and apostrophes while preserving both rollback branches.

Compatibility concern: exact VPS Compose 5.1.4 remains untested locally. Scalar `env_file` is longstanding syntax and passes local Compose 5.3.1 config and runtime verification; execute the same config probe on the VPS before deployment.
