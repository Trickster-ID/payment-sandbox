#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

handler_files=(
  "app/modules/invoice/handlers/invoice_handler.go"
  "app/modules/payment/handlers/payment_handler.go"
  "app/modules/refund/handlers/refund_handler.go"
  "app/modules/wallet/handlers/wallet_handler.go"
)

required_actions=(
  "INVOICE_CREATE"
  "PAYMENT_INTENT_CREATE"
  "PAYMENT_INTENT_STATUS_UPDATE"
  "REFUND_REQUEST"
  "REFUND_REVIEW"
  "REFUND_PROCESS"
  "TOPUP_CREATE"
  "TOPUP_STATUS_UPDATE"
)

echo "[iso-journey-events] checking handler files"
for file in "${handler_files[@]}"; do
  [[ -f "$file" ]] || {
    echo "[iso-journey-events] missing handler file: $file" >&2
    exit 1
  }
done

echo "[iso-journey-events] checking required journey action constants"
for action in "${required_actions[@]}"; do
  if ! rg -q "Action:\\s*\"${action}\"" "${handler_files[@]}"; then
    echo "[iso-journey-events] missing journey action: ${action}" >&2
    exit 1
  fi
done

echo "[iso-journey-events] checks passed"
