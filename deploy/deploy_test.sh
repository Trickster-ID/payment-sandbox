#!/usr/bin/env sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

bin="$tmp/bin"
deploy_dir="$tmp/payment-sandbox"
mkdir -p "$bin" "$deploy_dir/misc/init-sql"

cat > "$bin/sudo" <<'EOF'
#!/usr/bin/env sh
exec "$@"
EOF

cat > "$bin/docker" <<'EOF'
#!/usr/bin/env sh
printf '%s\n' "$*" >> "${DOCKER_LOG:?}"
if [ "$1" = login ] && [ "${DOCKER_LOGIN_FAIL:-}" = 1 ]; then
  exit 1
fi
exit 0
EOF

cat > "$bin/curl" <<'EOF'
#!/usr/bin/env sh
if [ "${CURL_FAIL:-}" = 1 ]; then
  exit 1
fi
exit 0
EOF

cat > "$bin/sleep" <<'EOF'
#!/usr/bin/env sh
exit 0
EOF

chmod +x "$bin/sudo" "$bin/docker" "$bin/curl" "$bin/sleep"

cp "$root/deploy/docker-compose.yml" "$deploy_dir/docker-compose.yml"
workflow_dir="$tmp/workflow"
mkdir -p "$workflow_dir/deploy" "$workflow_dir/misc"
cp "$root/.github/workflows/deploy-vps.yml" "$workflow_dir/deploy-vps.yml"
cp "$root/deploy/docker-compose.yml" "$root/deploy/deploy.sh" "$workflow_dir/deploy/"
cp -R "$root/misc/init-sql" "$workflow_dir/misc/"
ruby -e 'require "yaml"; workflow = YAML.load_file(ARGV[0]); puts workflow.dig("jobs", "test-build-deploy", "steps").find { |step| step["name"] == "Prepare deployment files" }.fetch("run")' "$workflow_dir/deploy-vps.yml" > "$workflow_dir/prepare.sh"

JWT_SECRET='jwt$literal$$dollar${braced}#hashit'"'"'s' \
DB_PASSWORD='db$literal$$dollar${braced}#hashit'"'"'s' \
MONGO_PASSWORD='mongo$literal$$dollar${braced}#hashit'"'"'s' \
GHCR_PULL_TOKEN=token \
sh -c 'cd "$1" && sh prepare.sh' sh "$workflow_dir"
cp "$workflow_dir/.deploy-upload/.env" "$deploy_dir/.env"

rm -rf "$workflow_dir/.deploy-upload"
bad_jwt=$(printf 'bad\nsecretX')
bad_jwt=${bad_jwt%X}
set +e
JWT_SECRET="$bad_jwt" DB_PASSWORD=database MONGO_PASSWORD=mongo GHCR_PULL_TOKEN=token sh -c 'cd "$1" && sh prepare.sh' sh "$workflow_dir" >/dev/null 2>&1
status=$?
set -e
if [ "$status" -eq 0 ] || [ -e "$workflow_dir/.deploy-upload/.env" ]; then
  echo 'expected CR/LF secret rejection before dotenv generation' >&2
  exit 1
fi

JWT_SECRET='jwt$literal$$dollar${braced}#hashit'"'"'s' \
DB_PASSWORD='db$literal$$dollar${braced}#hashit'"'"'s' \
MONGO_PASSWORD='mongo$literal$$dollar${braced}#hashit'"'"'s' \
GHCR_PULL_TOKEN=token \
sh -c 'cd "$1" && sh prepare.sh' sh "$workflow_dir"
cp "$workflow_dir/.deploy-upload/.env" "$deploy_dir/.env"
printf 'token\n' > "$deploy_dir/.ghcr-token"
printf 'IMAGE=ghcr.io/owner/payment-sandbox:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n' > "$deploy_dir/.deploy.env"
case "$(cat "$deploy_dir/docker-compose.yml")" in
  *'env_file: .env'*'export PGPASSWORD="$${DB_PASSWORD:?DB_PASSWORD must be set in .env}"'*) ;;
  *)
    echo 'expected scalar runtime env-file and container-side PGPASSWORD export' >&2
    exit 1
    ;;
esac
if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
  docker compose -f "$deploy_dir/docker-compose.yml" --env-file "$deploy_dir/.deploy.env" config >/dev/null
  printf 'IMAGE=alpine:3.22\n' > "$deploy_dir/.deploy.env"
  cat > "$deploy_dir/compose-secret-probe.yml" <<'EOF'
services:
  probe:
    image: ${IMAGE:?IMAGE must be set in .deploy.env}
    command: env
    env_file: .env
EOF
  probe_config=$(docker compose -f "$deploy_dir/compose-secret-probe.yml" --env-file "$deploy_dir/.deploy.env" config --format json)
  config_jwt='"JWT_SECRET": "jwt$$literal$$$$dollar$${braced}#hashit'"'"'s"'
  config_db='"DB_PASSWORD": "db$$literal$$$$dollar$${braced}#hashit'"'"'s"'
  config_mongo='"MONGO_URI": "mongodb://payment_sandbox_user:mongo%24literal%24%24dollar%24%7Bbraced%7D%23hashit%27s@mongodb:27017/payment_sandbox?authSource=payment_sandbox"'
  case "$probe_config" in *"$config_jwt"*) ;; *) echo 'expected quoted JWT secret in Compose config' >&2; exit 1 ;; esac
  case "$probe_config" in *"$config_db"*) ;; *) echo 'expected quoted DB password in Compose config' >&2; exit 1 ;; esac
  case "$probe_config" in *"$config_mongo"*) ;; *) echo 'expected quoted Mongo URI in Compose config' >&2; exit 1 ;; esac
  probe_runtime=$(docker compose -f "$deploy_dir/compose-secret-probe.yml" --env-file "$deploy_dir/.deploy.env" run --rm probe)
  case "$probe_runtime" in
    *'JWT_SECRET=jwt$literal$$dollar${braced}#hashit'"'"'s'*) ;;
    *) echo 'expected JWT secret runtime round-trip' >&2; exit 1 ;;
  esac
  case "$probe_runtime" in
    *'DB_PASSWORD=db$literal$$dollar${braced}#hashit'"'"'s'*) ;;
    *) echo 'expected DB password runtime round-trip' >&2; exit 1 ;;
  esac
  case "$probe_runtime" in
    *'MONGO_URI=mongodb://payment_sandbox_user:mongo%24literal%24%24dollar%24%7Bbraced%7D%23hashit%27s@mongodb:27017/payment_sandbox?authSource=payment_sandbox'*) ;;
    *) echo 'expected Mongo URI runtime round-trip' >&2; exit 1 ;;
  esac
  printf 'IMAGE=ghcr.io/owner/payment-sandbox:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n' > "$deploy_dir/.deploy.env"
fi
case "$(cat "$root/deploy/deploy.sh")" in
  *'--env-file .env'*)
    echo 'must not pass runtime .env to Compose' >&2
    exit 1
    ;;
esac

set +e
DOCKER_LOG="$tmp/docker.log" DOCKER_LOGIN_FAIL=1 PATH="$bin:$PATH" DEPLOY_DIR="$deploy_dir" sh "$root/deploy/deploy.sh" ghcr.io/owner/payment-sandbox:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb >/dev/null 2>&1
status=$?
set -e
if [ "$status" -eq 0 ]; then
  echo 'expected GHCR login failure' >&2
  exit 1
fi

test "$(cat "$deploy_dir/.deploy.env")" = 'IMAGE=ghcr.io/owner/payment-sandbox:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'

set +e
DOCKER_LOG="$tmp/docker.log" PATH="$bin:$PATH" DEPLOY_DIR="$deploy_dir" sh "$root/deploy/deploy.sh" ghcr.io/owner/payment-sandbox:latest >/dev/null 2>&1
status=$?
set -e
if [ "$status" -eq 0 ]; then
  echo 'expected mutable tag rejection' >&2
  exit 1
fi

test "$(cat "$deploy_dir/.deploy.env")" = 'IMAGE=ghcr.io/owner/payment-sandbox:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'

rm "$deploy_dir/.deploy.env"
printf 'token\n' > "$deploy_dir/.ghcr-token"

set +e
DOCKER_LOG="$tmp/docker.log" DOCKER_LOGIN_FAIL=1 PATH="$bin:$PATH" DEPLOY_DIR="$deploy_dir" sh "$root/deploy/deploy.sh" ghcr.io/owner/payment-sandbox:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb >/dev/null 2>&1
status=$?
set -e
if [ "$status" -eq 0 ]; then
  echo 'expected GHCR login failure without prior deployment' >&2
  exit 1
fi

test ! -e "$deploy_dir/.deploy.env"

printf 'token\n' > "$deploy_dir/.ghcr-token"
: > "$tmp/docker.log"

set +e
DOCKER_LOG="$tmp/docker.log" CURL_FAIL=1 PATH="$bin:$PATH" DEPLOY_DIR="$deploy_dir" sh "$root/deploy/deploy.sh" ghcr.io/owner/payment-sandbox:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb >/dev/null 2>&1
status=$?
set -e
if [ "$status" -eq 0 ]; then
  echo 'expected first-deploy health failure' >&2
  exit 1
fi

test ! -e "$deploy_dir/.deploy.env"
case "$(tr '\n' ' ' < "$tmp/docker.log")" in
  *'compose -f docker-compose.yml --env-file .deploy.env rm -sf api'*) ;;
  *)
    echo 'expected first-deploy API removal before deploy state cleanup' >&2
    exit 1
    ;;
esac

printf 'IMAGE=ghcr.io/owner/payment-sandbox:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n' > "$deploy_dir/.deploy.env"
printf 'token\n' > "$deploy_dir/.ghcr-token"
: > "$tmp/docker.log"

set +e
DOCKER_LOG="$tmp/docker.log" CURL_FAIL=1 PATH="$bin:$PATH" DEPLOY_DIR="$deploy_dir" sh "$root/deploy/deploy.sh" ghcr.io/owner/payment-sandbox:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb >/dev/null 2>&1
status=$?
set -e
if [ "$status" -eq 0 ]; then
  echo 'expected rollback health failure' >&2
  exit 1
fi

test "$(tr -d '\n' < "$deploy_dir/.deploy.env")" = 'IMAGE=ghcr.io/owner/payment-sandbox:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
case "$(tr '\n' ' ' < "$tmp/docker.log")" in
  *'compose -f docker-compose.yml --env-file .deploy.env rm -sf api'*)
    echo 'must not remove API when a prior image exists' >&2
    exit 1
    ;;
esac
