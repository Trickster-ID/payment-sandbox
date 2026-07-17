#!/usr/bin/env sh
set -eu

deploy_dir=${DEPLOY_DIR:-/home/pik/container/payment-sandbox}
image=${1:?usage: deploy.sh <immutable-image>}

if ! printf '%s\n' "$image" | grep -Eq '^ghcr\.io/[^/:]+/payment-sandbox:[0-9a-f]{40}$'; then
  echo "image must be ghcr.io/<owner>/payment-sandbox:<40 lowercase hexadecimal SHA>" >&2
  exit 64
fi

cd "$deploy_dir"
test -f .env
test -f .ghcr-token
test -d misc/init-sql

had_deploy_env=false
deploy_env_backup=.deploy.env.rollback.$$
if [ -f .deploy.env ]; then
  cp .deploy.env "$deploy_env_backup"
  had_deploy_env=true
fi

rollback_api=false
rollback() {
  status=$?
  trap - EXIT
  if [ "$status" -ne 0 ]; then
    if [ "$had_deploy_env" = true ]; then
      mv "$deploy_env_backup" .deploy.env
      if [ "$rollback_api" = true ]; then
        sudo docker compose -f docker-compose.yml --env-file .deploy.env up -d api || true
      fi
    elif [ "$rollback_api" = true ]; then
      sudo docker compose -f docker-compose.yml --env-file .deploy.env rm -sf api || true
      rm -f .deploy.env
    else
      rm -f .deploy.env
    fi
  else
    rm -f "$deploy_env_backup"
  fi
  exit "$status"
}
trap rollback EXIT

umask 077
printf 'IMAGE=%s\n' "$image" > .deploy.env.next
mv .deploy.env.next .deploy.env

registry=$(printf '%s' "$image" | cut -d/ -f1)
username=$(printf '%s' "$image" | cut -d/ -f2)
sudo docker login "$registry" --username "$username" --password-stdin < .ghcr-token
rm -f .ghcr-token

rollback_api=true

sudo docker compose -f docker-compose.yml --env-file .deploy.env pull api
sudo docker compose -f docker-compose.yml --env-file .deploy.env --profile schema run --rm schema
sudo docker compose -f docker-compose.yml --env-file .deploy.env up -d --remove-orphans api

attempt=0
until curl -fsS http://127.0.0.1:8080/api/v1/ping >/dev/null; do
  attempt=$((attempt + 1))
  if [ "$attempt" -eq 30 ]; then
    echo "API health check failed" >&2
    exit 1
  fi
  sleep 2
done

rollback
