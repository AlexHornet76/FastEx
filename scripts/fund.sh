#!/usr/bin/env bash
set -euo pipefail

GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"
TOKEN="${TOKEN:-}"

USER_ASSET="${USER_ASSET:-USD}"
AMOUNT="${AMOUNT:-1000000}"

# For convenience: allow specifying multiple deposits:
#   FUND="USD:1000000,BTC:100"
FUND="${FUND:-}"

if [[ -z "$TOKEN" ]]; then
  echo "Missing TOKEN env var (JWT)."
  echo "Usage:"
  echo "  TOKEN=... USER_ASSET=USD AMOUNT=1000000 ./scripts/fund.sh"
  echo "or:"
  echo "  TOKEN=... FUND='USD:1000000,BTC:100' ./scripts/fund.sh"
  exit 1
fi

deposit_one() {
  local asset="$1"
  local amount="$2"

  echo "Depositing asset=$asset amount=$amount"
  curl -sS -X POST "$GATEWAY_URL/api/deposit" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -d "{
      \"asset\": \"$asset\",
      \"amount\": $amount
    }" | jq .
  echo
}

if [[ -n "$FUND" ]]; then
  IFS=',' read -r -a parts <<< "$FUND"
  for p in "${parts[@]}"; do
    asset="${p%%:*}"
    amount="${p##*:}"
    deposit_one "$asset" "$amount"
  done
else
  deposit_one "$USER_ASSET" "$AMOUNT"
fi

echo "Done."