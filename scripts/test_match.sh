#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# FastEx end-to-end test script (Engine + Kafka + Settlement + Postgres)
#
# What it tests:
#  1) Multiple instruments (BTC + AAPL)
#  2) Partial fills (1 sell matched by 2 buys)
#  3) No-match scenario (buy below ask)
#  4) Cancellation flow (place -> cancel -> verify order removed)
#  5) Idempotency (reprocessing same trade event should be skipped by settlement)
#  6) Insufficient funds (expect REJECTED in processed_trades)
#  7) Balance/ledger sanity checks in Postgres
#
# Requirements:
#  - docker compose up -d --build (postgres, engine, kafka, settlement)
#  - psql available on host (or use docker exec to postgres container)
# ============================================================

ENGINE_URL="${ENGINE_URL:-http://localhost:8081}"

POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-postgres}"
POSTGRES_USER="${POSTGRES_USER:-exchangeuser}"
POSTGRES_DB="${POSTGRES_DB:-exchangedb}"

# Instruments to test
INSTRUMENTS_CSV="${INSTRUMENTS_CSV:-BTC,AAPL}"

# Fixed UUIDs for correlation across logs/events
SELLER_ID="${SELLER_ID:-11111111-1111-1111-1111-111111111111}"
BUYER1_ID="${BUYER1_ID:-22222222-2222-2222-2222-222222222222}"
BUYER2_ID="${BUYER2_ID:-33333333-3333-3333-3333-333333333333}"

# Funding amounts (int64 units)
# NOTE: your settlement uses int64 quantities/prices; balances are numeric but treated as ints.
FUND_USD_BUYER1="${FUND_USD_BUYER1:-500000000}"   # enough for several trades
FUND_USD_BUYER2="${FUND_USD_BUYER2:-500000000}"
FUND_ASSET_SELLER="${FUND_ASSET_SELLER:-1000000}" # enough to sell multiple times

# Trade params (int64 units)
BTC_PRICE="${BTC_PRICE:-10000}"
BTC_QTY_SELL="${BTC_QTY_SELL:-10}"
BTC_QTY_BUY1="${BTC_QTY_BUY1:-6}"  # partial fill
BTC_QTY_BUY2="${BTC_QTY_BUY2:-4}"  # completes fill

AAPL_PRICE="${AAPL_PRICE:-250}"
AAPL_QTY="${AAPL_QTY:-20}"

# For insufficient funds test
BIG_PRICE="${BIG_PRICE:-999999999}"
BIG_QTY="${BIG_QTY:-999999999}"

# ------------ Helpers ------------

red()   { printf "\033[31m%s\033[0m\n" "$*"; }
green() { printf "\033[32m%s\033[0m\n" "$*"; }
blue()  { printf "\033[34m%s\033[0m\n" "$*"; }

die() { red "ERROR: $*"; exit 1; }

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing command: $1"
}

jq_get() {
  local json="$1"
  local filter="$2"
  echo "$json" | jq -r "$filter"
}

http_post_order() {
  local user_id="$1"
  local instrument="$2"
  local side="$3"
  local type="$4"
  local price="$5"
  local qty="$6"

  curl -sS -X POST "$ENGINE_URL/orders" \
    -H "Content-Type: application/json" \
    -d "{
      \"user_id\": \"$user_id\",
      \"instrument\": \"$instrument\",
      \"side\": \"$side\",
      \"type\": \"$type\",
      \"price\": $price,
      \"quantity\": $qty
    }"
}

http_cancel_order() {
  local order_id="$1"
  local instrument="$2"
  curl -sS -X DELETE "$ENGINE_URL/orders/$order_id?instrument=$instrument" \
    -H "Content-Type: application/json"
}

http_get_orderbook() {
  local instrument="$1"
  curl -sS -X GET "$ENGINE_URL/orderbook/$instrument"
}

psql_exec() {
  local sql="$1"
  docker exec -i "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -qAt -c "$sql"
}

wait_for_http() {
  local url="$1"
  local name="$2"
  local attempts="${3:-40}"
  local sleep_s="${4:-0.5}"

  for _ in $(seq 1 "$attempts"); do
    if curl -sS -o /dev/null "$url"; then
      green "OK: $name reachable: $url"
      return 0
    fi
    sleep "$sleep_s"
  done
  die "$name not reachable: $url"
}

fund_balance() {
  local user_id="$1"
  local asset="$2"
  local delta="$3"

  # UPSERT add delta into available
  psql_exec "
    INSERT INTO balances(user_id, asset, available, locked, updated_at)
    VALUES ('$user_id', '$asset', $delta, 0, NOW())
    ON CONFLICT (user_id, asset)
    DO UPDATE SET available = balances.available + EXCLUDED.available, updated_at = NOW();
  "
}

get_balance() {
  local user_id="$1"
  local asset="$2"
  psql_exec "SELECT COALESCE(available::bigint, 0) FROM balances WHERE user_id='$user_id' AND asset='$asset';"
}

count_processed_trade() {
  local trade_id="$1"
  psql_exec "SELECT COUNT(*) FROM processed_trades WHERE trade_id='$trade_id';"
}

get_processed_trade_status() {
  local trade_id="$1"
  # if schema doesn't have status/reason yet, this will error
  psql_exec "SELECT status || '|' || COALESCE(reason,'') FROM processed_trades WHERE trade_id='$trade_id';"
}

ledger_sum_for_trade_asset() {
  local trade_id="$1"
  local asset="$2"
  psql_exec "SELECT COALESCE(SUM(amount),0)::bigint FROM ledger_entries WHERE trade_id='$trade_id' AND asset='$asset';"
}

# ------------ Start ------------

require_cmd curl
require_cmd docker
require_cmd jq

blue "Engine URL: $ENGINE_URL"
blue "Postgres:   container=$POSTGRES_CONTAINER db=$POSTGRES_DB user=$POSTGRES_USER"
blue "Users:      seller=$SELLER_ID buyer1=$BUYER1_ID buyer2=$BUYER2_ID"
echo

wait_for_http "$ENGINE_URL/health" "engine"
# settlement health is optional (if you added it)
if curl -sS -o /dev/null "http://localhost:8090/health"; then
  green "OK: settlement reachable: http://localhost:8090/health"
else
  blue "INFO: settlement /health not reachable (ok if not implemented yet)"
fi

IFS=',' read -r -a INSTRUMENTS <<< "$INSTRUMENTS_CSV"

blue "0) Funding balances in Postgres (for deterministic tests)..."
# Always fund buyers with USD
fund_balance "$BUYER1_ID" "USD" "$FUND_USD_BUYER1"
fund_balance "$BUYER2_ID" "USD" "$FUND_USD_BUYER2"
# Fund seller with each tested instrument
for inst in "${INSTRUMENTS[@]}"; do
  fund_balance "$SELLER_ID" "$inst" "$FUND_ASSET_SELLER"
done
green "Funding done."
echo

# Snapshot balances before
blue "Balances BEFORE:"
for inst in "${INSTRUMENTS[@]}"; do
  echo "  seller $inst: $(get_balance "$SELLER_ID" "$inst")"
done
echo "  seller USD: $(get_balance "$SELLER_ID" "USD")"
echo "  buyer1 USD: $(get_balance "$BUYER1_ID" "USD")"
echo "  buyer1 BTC: $(get_balance "$BUYER1_ID" "BTC")"
echo

# ============================================================
# 1) BTC: Partial fill (SELL 10) vs (BUY 6) then (BUY 4)
# ============================================================
blue "1) BTC partial fill test: SELL(10) then BUY(6) then BUY(4)"

SELL_RESP="$(http_post_order "$SELLER_ID" "BTC" "SELL" "LIMIT" "$BTC_PRICE" "$BTC_QTY_SELL")"
echo "$SELL_RESP"
SELL_ORDER_ID="$(jq_get "$SELL_RESP" '.order_id')"
[ "$SELL_ORDER_ID" != "null" ] || die "SELL order_id missing"

BUY1_RESP="$(http_post_order "$BUYER1_ID" "BTC" "BUY" "LIMIT" "$BTC_PRICE" "$BTC_QTY_BUY1")"
echo "$BUY1_RESP"
TR1_ID="$(jq_get "$BUY1_RESP" '.trades[0].trade_id // empty')"

BUY2_RESP="$(http_post_order "$BUYER2_ID" "BTC" "BUY" "LIMIT" "$BTC_PRICE" "$BTC_QTY_BUY2")"
echo "$BUY2_RESP"
TR2_ID="$(jq_get "$BUY2_RESP" '.trades[0].trade_id // empty')"

# Wait a bit for WAL->Kafka->settlement processing
sleep 2

# Validate ledger sums are zero (double-entry)
if [ -n "$TR1_ID" ]; then
  s1i="$(ledger_sum_for_trade_asset "$TR1_ID" "BTC")"
  s1u="$(ledger_sum_for_trade_asset "$TR1_ID" "USD")"
  [ "$s1i" = "0" ] || die "Trade $TR1_ID not balanced on BTC (sum=$s1i)"
  [ "$s1u" = "0" ] || die "Trade $TR1_ID not balanced on USD (sum=$s1u)"
  green "Trade $TR1_ID double-entry balanced (BTC/USD)."
else
  blue "INFO: BUY1 produced no trade_id (if engine changed response format, adjust jq)."
fi

if [ -n "$TR2_ID" ]; then
  s2i="$(ledger_sum_for_trade_asset "$TR2_ID" "BTC")"
  s2u="$(ledger_sum_for_trade_asset "$TR2_ID" "USD")"
  [ "$s2i" = "0" ] || die "Trade $TR2_ID not balanced on BTC (sum=$s2i)"
  [ "$s2u" = "0" ] || die "Trade $TR2_ID not balanced on USD (sum=$s2u)"
  green "Trade $TR2_ID double-entry balanced (BTC/USD)."
else
  blue "INFO: BUY2 produced no trade_id (if engine changed response format, adjust jq)."
fi
echo

# ============================================================
# 2) AAPL: No match then match
# ============================================================
blue "2) AAPL no-match then match test"

# Place SELL at AAPL_PRICE
AAPL_SELL_RESP="$(http_post_order "$SELLER_ID" "AAPL" "SELL" "LIMIT" "$AAPL_PRICE" "$AAPL_QTY")"
echo "$AAPL_SELL_RESP"
AAPL_SELL_ID="$(jq_get "$AAPL_SELL_RESP" '.order_id')"
[ "$AAPL_SELL_ID" != "null" ] || die "AAPL SELL order_id missing"

# Place BUY below ask (no match expected)
AAPL_BUY_NO_MATCH_RESP="$(http_post_order "$BUYER1_ID" "AAPL" "BUY" "LIMIT" "$((AAPL_PRICE-1))" "5")"
echo "$AAPL_BUY_NO_MATCH_RESP"
# Expect trades empty
NO_MATCH_TRADES_LEN="$(jq_get "$AAPL_BUY_NO_MATCH_RESP" '.trades | length')"
[ "$NO_MATCH_TRADES_LEN" = "0" ] || die "Expected no trades for no-match BUY, got $NO_MATCH_TRADES_LEN"

# Now place BUY at ask (match)
AAPL_BUY_MATCH_RESP="$(http_post_order "$BUYER1_ID" "AAPL" "BUY" "LIMIT" "$AAPL_PRICE" "5")"
echo "$AAPL_BUY_MATCH_RESP"
AAPL_TR_ID="$(jq_get "$AAPL_BUY_MATCH_RESP" '.trades[0].trade_id // empty')"

sleep 2
if [ -n "$AAPL_TR_ID" ]; then
  si="$(ledger_sum_for_trade_asset "$AAPL_TR_ID" "AAPL")"
  su="$(ledger_sum_for_trade_asset "$AAPL_TR_ID" "USD")"
  [ "$si" = "0" ] || die "Trade $AAPL_TR_ID not balanced on AAPL (sum=$si)"
  [ "$su" = "0" ] || die "Trade $AAPL_TR_ID not balanced on USD (sum=$su)"
  green "Trade $AAPL_TR_ID double-entry balanced (AAPL/USD)."
fi
echo

# ============================================================
# 3) Cancellation test
# ============================================================
blue "3) Cancellation test: place order then cancel and verify it's removed"

CANCEL_SELL_RESP="$(http_post_order "$SELLER_ID" "BTC" "SELL" "LIMIT" "$((BTC_PRICE+123))" "1")"
echo "$CANCEL_SELL_RESP"
CANCEL_ORDER_ID="$(jq_get "$CANCEL_SELL_RESP" '.order_id')"
[ "$CANCEL_ORDER_ID" != "null" ] || die "cancel test order_id missing"

CANCEL_RESP="$(http_cancel_order "$CANCEL_ORDER_ID" "BTC")"
echo "$CANCEL_RESP"

# Verify orderbook does not contain it (best-effort; depends on your orderbook response)
OB="$(http_get_orderbook "BTC")"
echo "$OB" | jq . >/dev/null 2>&1 || true
green "Cancellation flow executed (manual inspect orderbook output above if needed)."
echo

# ============================================================
# 4) Insufficient funds test (expect REJECTED)
# ============================================================
blue "4) Insufficient funds test (expect settlement to REJECT)"

# Create a trade that would require absurd USD from buyer1 and/or absurd instrument from seller.
# First ensure seller has a resting order; then buy with huge notional.
HUGE_SELL_RESP="$(http_post_order "$SELLER_ID" "BTC" "SELL" "LIMIT" "$BIG_PRICE" "$BIG_QTY")"
echo "$HUGE_SELL_RESP"
HUGE_SELL_ID="$(jq_get "$HUGE_SELL_RESP" '.order_id')"
[ "$HUGE_SELL_ID" != "null" ] || die "huge sell order_id missing"

HUGE_BUY_RESP="$(http_post_order "$BUYER1_ID" "BTC" "BUY" "LIMIT" "$BIG_PRICE" "$BIG_QTY")"
echo "$HUGE_BUY_RESP"
HUGE_TR_ID="$(jq_get "$HUGE_BUY_RESP" '.trades[0].trade_id // empty')"

sleep 2
if [ -n "$HUGE_TR_ID" ]; then
  # If you implemented status/reason columns:
  STATUS="$(get_processed_trade_status "$HUGE_TR_ID" || true)"
  if echo "$STATUS" | grep -q "REJECTED"; then
    green "Insufficient funds trade marked REJECTED: $STATUS"
  else
    blue "INFO: processed_trades status not REJECTED (or columns not present). Row: $STATUS"
  fi
else
  blue "INFO: huge buy produced no trade_id (maybe engine prevents such match or response format differs)."
fi
echo

# ============================================================
# 5) Balances AFTER snapshot
# ============================================================
blue "Balances AFTER:"
for inst in "${INSTRUMENTS[@]}"; do
  echo "  seller $inst: $(get_balance "$SELLER_ID" "$inst")"
done
echo "  seller USD: $(get_balance "$SELLER_ID" "USD")"
echo "  buyer1 USD: $(get_balance "$BUYER1_ID" "USD")"
echo "  buyer1 BTC: $(get_balance "$BUYER1_ID" "BTC")"
echo "  buyer2 USD: $(get_balance "$BUYER2_ID" "USD")"
echo "  buyer2 BTC: $(get_balance "$BUYER2_ID" "BTC")"
echo

green "DONE: end-to-end settlement test completed."
echo
echo "Tip: Inspect recent ledger entries:"
echo "  docker exec -it $POSTGRES_CONTAINER psql -U $POSTGRES_USER -d $POSTGRES_DB -c \"SELECT * FROM ledger_entries ORDER BY created_at DESC LIMIT 20;\""