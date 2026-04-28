#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

DRILL_DIR="docs/iso/drills"

if [[ ! -d "$DRILL_DIR" ]]; then
  echo "[iso-drill-evidence] missing drill directory: $DRILL_DIR" >&2
  exit 1
fi

count=$(find "$DRILL_DIR" -maxdepth 1 -type f -name '*.md' ! -name 'README.md' | wc -l | tr -d ' ')
if [[ "$count" -lt 1 ]]; then
  echo "[iso-drill-evidence] no drill record found in $DRILL_DIR" >&2
  exit 1
fi

backup_restore_count=$(rg -l "^-\s*Drill type:\s*backup-restore$" "$DRILL_DIR"/*.md 2>/dev/null | wc -l | tr -d ' ')
if [[ "$backup_restore_count" -lt 1 ]]; then
  echo "[iso-drill-evidence] at least one backup-restore drill record is required" >&2
  exit 1
fi

echo "[iso-drill-evidence] drill record count: $count"
echo "[iso-drill-evidence] backup-restore record count: $backup_restore_count"
