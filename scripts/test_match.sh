#!/usr/bin/env bash
set -euo pipefail

ENGINE_URL="${ENGINE_URL:-http://localhost:8081}"
INSTRUMENT="${INSTRUMENT:-BTC}"

# Use fixed UUIDs so you can correlate logs/events easily
SELL_USER_ID="${SELL_USER_ID:-11111111-1111-1111-1111-111111111111}"
BUY_USER_ID="${BUY_USER_ID:-22222222-2222-2222-2222-222222222222}"

PRICE="${PRICE:-10000}"
QTY="${QTY:-5}"

echo "Engine:      $ENGINE_URL"
echo "Instrument:  $INSTRUMENT"
echo "Price:       $PRICE"
echo "Quantity:    $QTY"
echo

echo "1) Placing SELL (LIMIT)..."
SELL_RESP="$(curl -sS -X POST "$ENGINE_URL/orders" \
  -H "Content-Type: application/json" \
  -d "{
    \"user_id\": \"$SELL_USER_ID\",
    \"instrument\": \"$INSTRUMENT\",
    \"side\": \"SELL\",
    \"type\": \"LIMIT\",
    \"price\": $PRICE,
    \"quantity\": $QTY
  }")"

echo "$SELL_RESP"
echo

echo "2) Placing BUY (LIMIT) that matches..."
BUY_RESP="$(curl -sS -X POST "$ENGINE_URL/orders" \
  -H "Content-Type: application/json" \
  -d "{
    \"user_id\": \"$BUY_USER_ID\",
    \"instrument\": \"$INSTRUMENT\",
    \"side\": \"BUY\",
    \"type\": \"LIMIT\",
    \"price\": $PRICE,
    \"quantity\": $QTY
  }")"

echo "$BUY_RESP"
echo

echo "Done."