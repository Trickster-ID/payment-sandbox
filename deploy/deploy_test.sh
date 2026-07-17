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
printf 'DB_PASSWORD=pa$ss#word\nJWT_SECRET=a$b\n' > "$deploy_dir/.env"
printf 'token\n' > "$deploy_dir/.ghcr-token"
printf 'IMAGE=ghcr.io/owner/payment-sandbox:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n' > "$deploy_dir/.deploy.env"
case "$(cat "$deploy_dir/docker-compose.yml")" in
  *'env_file: {path: .env, format: raw}'*'export PGPASSWORD="$${DB_PASSWORD:?DB_PASSWORD must be set in .env}"'*) ;;
  *)
    echo 'expected raw runtime env-file and container-side PGPASSWORD export' >&2
    exit 1
    ;;
esac
test "$(sed -n 's/^DB_PASSWORD=//p' "$deploy_dir/.env")" = 'pa$ss#word'
test "$(sed -n 's/^JWT_SECRET=//p' "$deploy_dir/.env")" = 'a$b'
DB_PASSWORD='pa$ss#word' sh -ec 'export PGPASSWORD="${DB_PASSWORD:?DB_PASSWORD must be set in .env}"; test "$PGPASSWORD" = "pa\$ss#word"'
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
