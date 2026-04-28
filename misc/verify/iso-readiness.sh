#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

ISO_SKIP_DRILL_CHECK="${ISO_SKIP_DRILL_CHECK:-0}"

required_files=(
  "SECURITY.md"
  "docs/iso/README.md"
  "docs/iso/01-scope-and-context.md"
  "docs/iso/02-asset-inventory.md"
  "docs/iso/03-risk-register.md"
  "docs/iso/04-control-matrix.md"
  "docs/iso/05-statement-of-applicability.md"
  "docs/iso/06-vulnerability-management.md"
  "docs/iso/07-business-continuity-plan.md"
  "docs/iso/08-disaster-recovery-runbook.md"
  "docs/iso/09-audit-evidence-checklist.md"
  "docs/iso/logging-and-retention.md"
  "docs/security/config-hardening.md"
)

echo "[iso-readiness] checking required files"
for file in "${required_files[@]}"; do
  if [[ ! -f "$file" ]]; then
    echo "[iso-readiness] missing required file: $file" >&2
    exit 1
  fi
done

echo "[iso-readiness] running config tests"
go test ./app/config -v

echo "[iso-readiness] running full test suite"
go test ./...

echo "[iso-readiness] running route parity regression"
go test ./app/cmd -run TestNewRouter_RegistersExpectedRoutes -v

echo "[iso-readiness] running journey event taxonomy check"
./misc/verify/iso-journey-events.sh

if [[ "$ISO_SKIP_DRILL_CHECK" == "1" ]]; then
  echo "[iso-readiness] skipping continuity drill evidence check (ISO_SKIP_DRILL_CHECK=1)"
else
  echo "[iso-readiness] checking continuity drill evidence"
  ./misc/verify/iso-drill-evidence.sh
fi

echo "[iso-readiness] all checks passed"
