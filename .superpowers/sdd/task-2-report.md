Status: DONE
Commit SHA(s): 8ebff5d
Test result: `docker compose -f /var/folders/41/xc2gytfj04d1t5xk3dzq1wfh0000gp/T/opencode/payment-sandbox-deploy-check/docker-compose.yml --env-file /var/folders/41/xc2gytfj04d1t5xk3dzq1wfh0000gp/T/opencode/payment-sandbox-deploy-check/.env config >/dev/null` and `sh -n deploy/deploy.sh` passed.
Concerns: Remote deployment requires existing external Docker networks, runtime `.env`, GHCR pull token, database ownership, and schema privileges.

Review-fix result: `sh -n deploy/deploy.sh`, `sh -n deploy/deploy_test.sh`, and `sh deploy/deploy_test.sh` passed. The no-Docker test uses temporary fake `sudo`, `docker`, and `curl` binaries; it verifies failed GHCR login restores an existing `.deploy.env`, removes `.deploy.env` when none existed, and rejects `:latest`.
Review-fix concern: `DEPLOY_DIR` is a test-only override; production default remains `/home/pik/container/payment-sandbox`.

Final-review fix result: `deploy/docker-compose.yml` loads runtime `.env` with `env_file: {path: .env, format: raw}` for API and schema. Schema derives `PGPASSWORD` inside its container shell. `deploy.sh` supplies only `.deploy.env` to Compose; `.deploy.env` contains only `IMAGE`. `sh -n deploy/deploy.sh`, `sh -n deploy/deploy_test.sh`, and `sh deploy/deploy_test.sh` pass. The regression asserts literal `DB_PASSWORD=pa$ss#word` and `JWT_SECRET=a$b`, raw Compose declarations, container-side `PGPASSWORD` export, no runtime `--env-file .env`, first-deploy API removal, and prior-image rollback preservation.

Final-review fix concern: local Docker Compose 5.3.1 rejects the server-supported mapping form (`services.api.env_file must be a string`), so local config rendering cannot cover the production Compose 5.1.4 raw syntax. The regression is a static Compose-contract and shell-expansion check; execute `docker compose ... config` on the VPS before deployment.
