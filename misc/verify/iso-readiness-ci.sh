#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

echo "[iso-readiness-ci] running CI-safe ISO checks (drill evidence optional)"
ISO_SKIP_DRILL_CHECK=1 ./misc/verify/iso-readiness.sh
