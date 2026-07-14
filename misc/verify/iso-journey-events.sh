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

# parallel arrays (portable across bash 3.2 / no associative arrays)
required_action_names=(
  "INVOICE_CREATE"
  "PAYMENT_INTENT_CREATE"
  "PAYMENT_INTENT_STATUS_UPDATE"
  "REFUND_REQUEST"
  "REFUND_REVIEW"
  "REFUND_PROCESS"
  "TOPUP_CREATE"
  "TOPUP_STATUS_UPDATE"
)
required_event_types=(
  "invoice.created"
  "payment.intent_created"
  "payment.status_updated"
  "refund.requested"
  "refund.reviewed"
  "refund.processed"
  "topup.created"
  "topup.status_updated"
)

echo "[iso-journey-events] checking handler files"
for file in "${handler_files[@]}"; do
  [[ -f "$file" ]] || {
    echo "[iso-journey-events] missing handler file: $file" >&2
    exit 1
  }
done

echo "[iso-journey-events] checking required journey action constants"
count="${#required_action_names[@]}"
for ((i = 0; i < count; i++)); do
  action="${required_action_names[$i]}"
  event_type="${required_event_types[$i]}"
  if ! grep -qE "EventType:[[:space:]]*\"${event_type}\"" "${handler_files[@]}"; then
    echo "[iso-journey-events] missing journey action: ${action} (EventType: \"${event_type}\")" >&2
    exit 1
  fi
done

echo "[iso-journey-events] checks passed"
