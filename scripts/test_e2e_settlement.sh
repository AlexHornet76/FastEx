#!/usr/bin/env bash
set -euo pipefail

ENGINE_URL="${ENGINE_URL:-http://localhost:8081}"

POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-postgres}"
POSTGRES_USER="${POSTGRES_USER:-exchangeuser}"
POSTGRES_DB="${POSTGRES_DB:-exchangedb}"

KAFKA_CONTAINER="${KAFKA_CONTAINER:-kafka}"
KAFKA_BROKER="${KAFKA_BROKER:-kafka:9092}"
KAFKA_TOPIC="${KAFKA_TOPIC:-trade.executed}"

SELLER_ID="${SELLER_ID:-11111111-1111-1111-1111-111111111111}"
BUYER1_ID="${BUYER1_ID:-22222222-2222-2222-2222-222222222222}"
BUYER2_ID="${BUYER2_ID:-33333333-3333-3333-3333-333333333333}"

FUND_USD_BUYER1="${FUND_USD_BUYER1:-500000000}"
FUND_USD_BUYER2="${FUND_USD_BUYER2:-500000000}"
FUND_ASSET_SELLER="${FUND_ASSET_SELLER:-1000000}"

BTC_PRICE="${BTC_PRICE:-10000}"
BTC_QTY_SELL="${BTC_QTY_SELL:-10}"
BTC_QTY_BUY1="${BTC_QTY_BUY1:-6}"
BTC_QTY_BUY2="${BTC_QTY_BUY2:-4}"

# ---- helpers ----
red()   { printf "\033[31m%s\033[0m\n" "$*"; }
green() { printf "\033[32m%s\033[0m\n" "$*"; }
blue()  { printf "\033[34m%s\033[0m\n" "$*"; }

die() { red "FAIL: $*"; exit 1; }

require_cmd() { command -v "$1" >/dev/null 2>&1 || die "missing command: $1"; }

jq_get() { echo "$1" | jq -r "$2"; }

http_post_order() {
  curl -sS -X POST "$ENGINE_URL/orders" \
    -H "Content-Type: application/json" \
    -d "$1"
}

psql_exec() {
  docker exec -i "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -qAt -c "$1"
}

fund_balance() {
  local user_id="$1" asset="$2" delta="$3"
  psql_exec "
    INSERT INTO balances(user_id, asset, available, locked, updated_at)
    VALUES ('$user_id', '$asset', $delta, 0, NOW())
    ON CONFLICT (user_id, asset)
    DO UPDATE SET available = balances.available + EXCLUDED.available, updated_at = NOW();
  " >/dev/null
}

get_balance() {
  psql_exec "SELECT COALESCE(available::bigint, 0) FROM balances WHERE user_id='$1' AND asset='$2';"
}

count_ledger_entries() {
  psql_exec "SELECT COUNT(*) FROM ledger_entries WHERE trade_id='$1';"
}

sum_ledger_asset() {
  psql_exec "SELECT COALESCE(SUM(amount),0)::bigint FROM ledger_entries WHERE trade_id='$1' AND asset='$2';"
}

processed_status() {
  # expects status/reason columns exist; if not, returns empty
  psql_exec "SELECT COALESCE(status,'') || '|' || COALESCE(reason,'') FROM processed_trades WHERE trade_id='$1';" 2>/dev/null || true
}

fetch_trade_event_json() {
  # Extract the event payload saved in ledger? We don't have it.
  # So for replay, we rebuild JSON based on known fields from order response (trade_id + ids + price/qty/instrument).
  local trade_id="$1" instrument="$2" buyer="$3" seller="$4" price="$5" qty="$6"
  cat <<JSON
{"event_type":"trade.executed","event_time":"$(date -u +"%Y-%m-%dT%H:%M:%SZ")","instrument":"$instrument","trade_id":"$trade_id","buy_order_id":"00000000-0000-0000-0000-000000000000","sell_order_id":"00000000-0000-0000-0000-000000000000","buyer_user_id":"$buyer","seller_user_id":"$seller","price":$price,"quantity":$qty}
JSON
}

kafka_replay_trade() {
  local json="$1"
  echo "$json" | docker exec -i "$KAFKA_CONTAINER" bash -lc \
    "kafka-console-producer --bootstrap-server '$KAFKA_BROKER' --topic '$KAFKA_TOPIC' >/dev/null"
}

assert_eq() {
  local got="$1" exp="$2" msg="$3"
  if [ "$got" != "$exp" ]; then
    die "$msg (got=$got expected=$exp)"
  fi
}

# ---- start ----
require_cmd curl
require_cmd docker
require_cmd jq

blue "Waiting for engine..."
curl -sS "$ENGINE_URL/health" >/dev/null || die "engine not reachable"

blue "Funding balances..."
fund_balance "$BUYER1_ID" "USD" "$FUND_USD_BUYER1"
fund_balance "$BUYER2_ID" "USD" "$FUND_USD_BUYER2"
fund_balance "$SELLER_ID" "BTC" "$FUND_ASSET_SELLER"
green "Funding done."
echo

# snapshot before
b_buyer1_usd="$(get_balance "$BUYER1_ID" "USD")"
b_buyer2_usd="$(get_balance "$BUYER2_ID" "USD")"
b_seller_usd="$(get_balance "$SELLER_ID" "USD")"
b_buyer1_btc="$(get_balance "$BUYER1_ID" "BTC")"
b_buyer2_btc="$(get_balance "$BUYER2_ID" "BTC")"
b_seller_btc="$(get_balance "$SELLER_ID" "BTC")"

blue "Balances BEFORE:"
echo "  buyer1 USD=$b_buyer1_usd BTC=$b_buyer1_btc"
echo "  buyer2 USD=$b_buyer2_usd BTC=$b_buyer2_btc"
echo "  seller USD=$b_seller_usd BTC=$b_seller_btc"
echo

# ---- Execute scenario: partial fill ----
blue "Placing SELL BTC 10 @ 10000"
SELL_PAYLOAD="$(cat <<JSON
{"user_id":"$SELLER_ID","instrument":"BTC","side":"SELL","type":"LIMIT","price":$BTC_PRICE,"quantity":$BTC_QTY_SELL}
JSON
)"
SELL_RESP="$(http_post_order "$SELL_PAYLOAD")"
SELL_ORDER_ID="$(jq_get "$SELL_RESP" '.order_id')"
[ "$SELL_ORDER_ID" != "null" ] || die "SELL order_id missing"

blue "Placing BUY1 BTC 6 @ 10000"
BUY1_PAYLOAD="$(cat <<JSON
{"user_id":"$BUYER1_ID","instrument":"BTC","side":"BUY","type":"LIMIT","price":$BTC_PRICE,"quantity":$BTC_QTY_BUY1}
JSON
)"
BUY1_RESP="$(http_post_order "$BUY1_PAYLOAD")"
TR1_ID="$(jq_get "$BUY1_RESP" '.trades[0].trade_id // empty')"
[ -n "$TR1_ID" ] || die "BUY1 expected trade_id"

blue "Placing BUY2 BTC 4 @ 10000"
BUY2_PAYLOAD="$(cat <<JSON
{"user_id":"$BUYER2_ID","instrument":"BTC","side":"BUY","type":"LIMIT","price":$BTC_PRICE,"quantity":$BTC_QTY_BUY2}
JSON
)"
BUY2_RESP="$(http_post_order "$BUY2_PAYLOAD")"
TR2_ID="$(jq_get "$BUY2_RESP" '.trades[0].trade_id // empty')"
[ -n "$TR2_ID" ] || die "BUY2 expected trade_id"

# wait settlement
sleep 2

# ---- Assertions: double-entry ----
blue "Asserting ledger double-entry for TR1/TR2..."
assert_eq "$(count_ledger_entries "$TR1_ID")" "4" "TR1 should create 4 ledger entries"
assert_eq "$(sum_ledger_asset "$TR1_ID" "BTC")" "0" "TR1 BTC sum must be 0"
assert_eq "$(sum_ledger_asset "$TR1_ID" "USD")" "0" "TR1 USD sum must be 0"

assert_eq "$(count_ledger_entries "$TR2_ID")" "4" "TR2 should create 4 ledger entries"
assert_eq "$(sum_ledger_asset "$TR2_ID" "BTC")" "0" "TR2 BTC sum must be 0"
assert_eq "$(sum_ledger_asset "$TR2_ID" "USD")" "0" "TR2 USD sum must be 0"
green "Ledger balanced."
echo

# ---- Assertions: balances delta ----
# expected:
# buyer1: BTC +6, USD -(6*10000)
# buyer2: BTC +4, USD -(4*10000)
# seller: BTC -10, USD +(10*10000)
exp_buyer1_usd="$((b_buyer1_usd - (BTC_QTY_BUY1*BTC_PRICE)))"
exp_buyer2_usd="$((b_buyer2_usd - (BTC_QTY_BUY2*BTC_PRICE)))"
exp_seller_usd="$((b_seller_usd + (BTC_QTY_SELL*BTC_PRICE)))"

exp_buyer1_btc="$((b_buyer1_btc + BTC_QTY_BUY1))"
exp_buyer2_btc="$((b_buyer2_btc + BTC_QTY_BUY2))"
exp_seller_btc="$((b_seller_btc - BTC_QTY_SELL))"

a_buyer1_usd="$(get_balance "$BUYER1_ID" "USD")"
a_buyer2_usd="$(get_balance "$BUYER2_ID" "USD")"
a_seller_usd="$(get_balance "$SELLER_ID" "USD")"
a_buyer1_btc="$(get_balance "$BUYER1_ID" "BTC")"
a_buyer2_btc="$(get_balance "$BUYER2_ID" "BTC")"
a_seller_btc="$(get_balance "$SELLER_ID" "BTC")"

blue "Asserting balances AFTER..."
assert_eq "$a_buyer1_usd" "$exp_buyer1_usd" "buyer1 USD mismatch"
assert_eq "$a_buyer2_usd" "$exp_buyer2_usd" "buyer2 USD mismatch"
assert_eq "$a_seller_usd" "$exp_seller_usd" "seller USD mismatch"

assert_eq "$a_buyer1_btc" "$exp_buyer1_btc" "buyer1 BTC mismatch"
assert_eq "$a_buyer2_btc" "$exp_buyer2_btc" "buyer2 BTC mismatch"
assert_eq "$a_seller_btc" "$exp_seller_btc" "seller BTC mismatch"
green "Balances correct."
echo

# ---- Assertions: idempotency replay ----
blue "Replaying TR1 event into Kafka (idempotency test)..."
before_cnt="$(count_ledger_entries "$TR1_ID")"

replay_json="$(fetch_trade_event_json "$TR1_ID" "BTC" "$BUYER1_ID" "$SELLER_ID" "$BTC_PRICE" "$BTC_QTY_BUY1")"
kafka_replay_trade "$replay_json"

sleep 2
after_cnt="$(count_ledger_entries "$TR1_ID")"
assert_eq "$after_cnt" "$before_cnt" "Idempotency failed: ledger entries changed after replay"

status="$(processed_status "$TR1_ID")"
if [ -n "$status" ]; then
  echo "processed_trades: $status"
fi
green "Idempotency OK (no extra ledger entries)."
echo

green "PASS: Settlement end-to-end assertions succeeded."