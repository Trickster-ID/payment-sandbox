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
if [ "$1" = login ]; then
  exit 1
fi
exit 0
EOF

cat > "$bin/curl" <<'EOF'
#!/usr/bin/env sh
exit 0
EOF

chmod +x "$bin/sudo" "$bin/docker" "$bin/curl"

printf 'runtime=true\n' > "$deploy_dir/.env"
printf 'token\n' > "$deploy_dir/.ghcr-token"
printf 'IMAGE=ghcr.io/owner/payment-sandbox:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n' > "$deploy_dir/.deploy.env"

set +e
PATH="$bin:$PATH" DEPLOY_DIR="$deploy_dir" sh "$root/deploy/deploy.sh" ghcr.io/owner/payment-sandbox:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb >/dev/null 2>&1
status=$?
set -e
if [ "$status" -eq 0 ]; then
  echo 'expected GHCR login failure' >&2
  exit 1
fi

test "$(cat "$deploy_dir/.deploy.env")" = 'IMAGE=ghcr.io/owner/payment-sandbox:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'

set +e
PATH="$bin:$PATH" DEPLOY_DIR="$deploy_dir" sh "$root/deploy/deploy.sh" ghcr.io/owner/payment-sandbox:latest >/dev/null 2>&1
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
PATH="$bin:$PATH" DEPLOY_DIR="$deploy_dir" sh "$root/deploy/deploy.sh" ghcr.io/owner/payment-sandbox:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb >/dev/null 2>&1
status=$?
set -e
if [ "$status" -eq 0 ]; then
  echo 'expected GHCR login failure without prior deployment' >&2
  exit 1
fi

test ! -e "$deploy_dir/.deploy.env"
