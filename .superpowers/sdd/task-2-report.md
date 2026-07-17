Status: DONE
Commit SHA(s): 8ebff5d
Test result: `docker compose -f /var/folders/41/xc2gytfj04d1t5xk3dzq1wfh0000gp/T/opencode/payment-sandbox-deploy-check/docker-compose.yml --env-file /var/folders/41/xc2gytfj04d1t5xk3dzq1wfh0000gp/T/opencode/payment-sandbox-deploy-check/.env config >/dev/null` and `sh -n deploy/deploy.sh` passed.
Concerns: Remote deployment requires existing external Docker networks, runtime `.env`, GHCR pull token, database ownership, and schema privileges.

Review-fix result: `sh -n deploy/deploy.sh`, `sh -n deploy/deploy_test.sh`, and `sh deploy/deploy_test.sh` passed. The no-Docker test uses temporary fake `sudo`, `docker`, and `curl` binaries; it verifies failed GHCR login restores an existing `.deploy.env`, removes `.deploy.env` when none existed, and rejects `:latest`.
Review-fix concern: `DEPLOY_DIR` is a test-only override; production default remains `/home/pik/container/payment-sandbox`.
