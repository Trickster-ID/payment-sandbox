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
