#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# FastEx HARD MODE Outbox Correctness Test
#
# Verifies WAL-backed publishing correctness under failures:
#  - Engine crash window after matching (kill)
#  - Engine pause/unpause (stop-the-world)
#  - Kafka outage (stop/start kafka container)
#  - Settlement resilience + idempotency (no duplicate ledger)
#
# What it asserts (per trade_id):
#  1) processed_trades eventually contains trade_id (APPLIED)
#  2) ledger_entries count == 4
#  3) SUM(amount) == 0 for instrument and USD
#
# Notes:
#  - This test relies on trade_id returned by engine HTTP response.
#  - It funds balances directly in Postgres (USD for buyer, asset for seller).
#  - It is "hard mode": it will interrupt infra repeatedly.
#  - Tweak the knobs if your machine is slower.
# ============================================================

ENGINE_URL="${ENGINE_URL:-http://localhost:8081}"

POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-postgres}"
POSTGRES_USER="${POSTGRES_USER:-exchangeuser}"
POSTGRES_DB="${POSTGRES_DB:-exchangedb}"

ENGINE_CONTAINER="${ENGINE_CONTAINER:-engine}"
SETTLEMENT_CONTAINER="${SETTLEMENT_CONTAINER:-settlement}"

KAFKA_CONTAINER="${KAFKA_CONTAINER:-kafka}"

# Test instruments (comma separated)
INSTRUMENTS_CSV="${INSTRUMENTS_CSV:-BTC,AAPL}"

# Fixed IDs for correlation
SELLER_ID="${SELLER_ID:-11111111-1111-1111-1111-111111111111}"
BUYER_ID="${BUYER_ID:-22222222-2222-2222-2222-222222222222}"

# Funding (int units)
FUND_USD_BUYER="${FUND_USD_BUYER:-2000000000}"
SEED_ASSET_SELLER="${SEED_ASSET_SELLER:-2000000000}"

# Prices/quantities (int64 units)
BTC_PRICE="${BTC_PRICE:-10000}"
BTC_QTY="${BTC_QTY:-10}"

AAPL_PRICE="${AAPL_PRICE:-250}"
AAPL_QTY="${AAPL_QTY:-20}"

# Iterations
ITERATIONS="${ITERATIONS:-12}"

# Timeouts
ENGINE_HEALTH_ATTEMPTS="${ENGINE_HEALTH_ATTEMPTS:-80}"
ENGINE_HEALTH_SLEEP="${ENGINE_HEALTH_SLEEP:-0.25}"

SETTLEMENT_WAIT_SECS="${SETTLEMENT_WAIT_SECS:-45}"
POST_EVENT_SLEEP_SECS="${POST_EVENT_SLEEP_SECS:-0.8}"

# Failure knobs
CRASH_PROB="${CRASH_PROB:-0.45}"         # chance to kill engine after trade
PAUSE_PROB="${PAUSE_PROB:-0.25}"         # chance to pause engine
KAFKA_OUTAGE_PROB="${KAFKA_OUTAGE_PROB:-0.25}" # chance to stop kafka for a bit
KAFKA_OUTAGE_SECS_MIN="${KAFKA_OUTAGE_SECS_MIN:-2}"
KAFKA_OUTAGE_SECS_MAX="${KAFKA_OUTAGE_SECS_MAX:-6}"

# Crash delay window after placing BUY (ms)
CRASH_DELAY_MS_MIN="${CRASH_DELAY_MS_MIN:-10}"
CRASH_DELAY_MS_MAX="${CRASH_DELAY_MS_MAX:-120}"

# Random seed (optional). Example: RANDOM_SEED=42
RANDOM_SEED="${RANDOM_SEED:-}"

# ---- helpers ----
red()   { printf "\033[31m%s\033[0m\n" "$*"; }
green() { printf "\033[32m%s\033[0m\n" "$*"; }
blue()  { printf "\033[34m%s\033[0m\n" "$*"; }
yellow(){ printf "\033[33m%s\033[0m\n" "$*"; }

die() { red "FAIL: $*"; exit 1; }

require_cmd() { command -v "$1" >/dev/null 2>&1 || die "missing command: $1"; }

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

count_processed() {
  local trade_id="$1"
  psql_exec "SELECT COUNT(*) FROM processed_trades WHERE trade_id='$trade_id';"
}

count_ledger_entries() {
  local trade_id="$1"
  psql_exec "SELECT COUNT(*) FROM ledger_entries WHERE trade_id='$trade_id';"
}

sum_ledger_asset() {
  local trade_id="$1" asset="$2"
  psql_exec "SELECT COALESCE(SUM(amount),0)::bigint FROM ledger_entries WHERE trade_id='$trade_id' AND asset='$asset';"
}

wait_for_engine() {
  for _ in $(seq 1 "$ENGINE_HEALTH_ATTEMPTS"); do
    if curl -sS "$ENGINE_URL/health" >/dev/null 2>&1; then
      return 0
    fi
    sleep "$ENGINE_HEALTH_SLEEP"
  done
  return 1
}

place_order() {
  local instrument="$1" user_id="$2" side="$3" type="$4" price="$5" qty="$6"
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

rand_int() {
  local min="$1" max="$2"
  if [ "$max" -lt "$min" ]; then
    echo "$min"
    return 0
  fi
  echo $(( min + (RANDOM % (max - min + 1)) ))
}

rand_float_0_1() {
  # returns a float like 0.123 using awk
  awk -v r="$RANDOM" 'BEGIN{srand(r); printf "%.6f\n", rand()}'
}

prob_hit() {
  local p="$1"
  local x
  x="$(rand_float_0_1)"
  awk -v x="$x" -v p="$p" 'BEGIN{exit (x<p)?0:1}'
}

sleep_ms() {
  local ms="$1"
  awk -v ms="$ms" 'BEGIN{printf "%.3f\n", ms/1000}' | xargs sleep
}

kafka_outage() {
  local secs="$1"
  yellow "KAFKA OUTAGE: stopping kafka for ${secs}s..."
  docker stop "$KAFKA_CONTAINER" >/dev/null || true
  sleep "$secs"
  yellow "KAFKA OUTAGE: starting kafka..."
  docker start "$KAFKA_CONTAINER" >/dev/null
  # give kafka time to boot
  sleep 2
}

engine_pause_window() {
  local secs="$1"
  yellow "ENGINE PAUSE: pausing engine for ${secs}s..."
  docker pause "$ENGINE_CONTAINER" >/dev/null || true
  sleep "$secs"
  yellow "ENGINE PAUSE: unpausing engine..."
  docker unpause "$ENGINE_CONTAINER" >/dev/null || true
}

engine_crash_window_after_ms() {
  local ms="$1"
  yellow "ENGINE CRASH: will kill engine after ${ms}ms..."
  sleep_ms "$ms"
  docker kill "$ENGINE_CONTAINER" >/dev/null || true
  yellow "ENGINE CRASH: killed."
  # restart it
  docker start "$ENGINE_CONTAINER" >/dev/null || true
}

assert_trade_invariants() {
  local trade_id="$1" instrument="$2"

  local deadline=$(( $(date +%s) + SETTLEMENT_WAIT_SECS ))
  while true; do
    local c
    c="$(count_processed "$trade_id")"
    if [ "$c" = "1" ]; then
      break
    fi
    if [ "$(date +%s)" -ge "$deadline" ]; then
      die "Timeout waiting processed_trades for trade_id=$trade_id instrument=$instrument"
    fi
    sleep 0.5
  done

  local le_cnt
  le_cnt="$(count_ledger_entries "$trade_id")"
  [ "$le_cnt" = "4" ] || die "Expected 4 ledger entries for trade_id=$trade_id, got $le_cnt"

  local inst_sum usd_sum
  inst_sum="$(sum_ledger_asset "$trade_id" "$instrument")"
  usd_sum="$(sum_ledger_asset "$trade_id" "USD")"
  [ "$inst_sum" = "0" ] || die "Instrument sum not zero for trade_id=$trade_id ($instrument sum=$inst_sum)"
  [ "$usd_sum" = "0" ] || die "USD sum not zero for trade_id=$trade_id (USD sum=$usd_sum)"
}

# ---- start ----
require_cmd curl
require_cmd docker
require_cmd jq
require_cmd awk

if [ -n "$RANDOM_SEED" ]; then
  RANDOM="$RANDOM_SEED"
fi

IFS=',' read -r -a INSTRUMENTS <<< "$INSTRUMENTS_CSV"

blue "HARD MODE Outbox Correctness Test"
blue "Iterations: $ITERATIONS"
blue "Engine:      $ENGINE_URL (container=$ENGINE_CONTAINER)"
blue "Kafka:       container=$KAFKA_CONTAINER"
blue "Settlement:  container=$SETTLEMENT_CONTAINER"
blue "Users:       seller=$SELLER_ID buyer=$BUYER_ID"
echo

blue "0) Ensure engine is up..."
wait_for_engine || die "engine not reachable at $ENGINE_URL"
green "Engine is up."
echo

blue "1) Fund buyer USD and seed seller assets (DB direct)..."
fund_balance "$BUYER_ID" "USD" "$FUND_USD_BUYER"
for inst in "${INSTRUMENTS[@]}"; do
  fund_balance "$SELLER_ID" "$inst" "$SEED_ASSET_SELLER"
done
green "Funding done."
echo

# Make sure kafka is running initially (best-effort)
docker start "$KAFKA_CONTAINER" >/dev/null 2>&1 || true

passed=0
for i in $(seq 1 "$ITERATIONS"); do
  inst="${INSTRUMENTS[$(( (i-1) % ${#INSTRUMENTS[@]} ))]}"

  # pick price/qty based on instrument
  if [ "$inst" = "BTC" ]; then
    price="$BTC_PRICE"
    qty="$BTC_QTY"
  else
    price="$AAPL_PRICE"
    qty="$AAPL_QTY"
  fi

  blue "------------------------------"
  blue "Iteration $i/$ITERATIONS instrument=$inst price=$price qty=$qty"
  echo

  # Maybe introduce a kafka outage BEFORE matching (publisher will fail and must resume)
  if prob_hit "$KAFKA_OUTAGE_PROB"; then
    outage_secs="$(rand_int "$KAFKA_OUTAGE_SECS_MIN" "$KAFKA_OUTAGE_SECS_MAX")"
    kafka_outage "$outage_secs"
  fi

  # Place SELL (resting)
  blue "Place SELL..."
  sell_resp="$(place_order "$inst" "$SELLER_ID" "SELL" "LIMIT" "$price" "$qty")"
  echo "$sell_resp" | jq . >/dev/null 2>&1 || true
  sell_oid="$(echo "$sell_resp" | jq -r '.order_id')"
  [ "$sell_oid" != "null" ] || die "SELL order_id missing (iter=$i)"

  # Maybe pause engine (simulates GC stop-the-world / scheduling issues)
  if prob_hit "$PAUSE_PROB"; then
    pause_secs="$(rand_int 1 3)"
    engine_pause_window "$pause_secs"
  fi

  # Place BUY (match)
  blue "Place BUY (match)..."
  buy_resp="$(place_order "$inst" "$BUYER_ID" "BUY" "LIMIT" "$price" "$qty")"
  echo "$buy_resp" | jq . >/dev/null 2>&1 || true

  trade_id="$(echo "$buy_resp" | jq -r '.trades[0].trade_id // empty')"
  [ -n "$trade_id" ] || die "Expected trade_id in BUY response (iter=$i)"

  echo "trade_id=$trade_id"

  # After trade, maybe crash engine quickly
  if prob_hit "$CRASH_PROB"; then
    crash_ms="$(rand_int "$CRASH_DELAY_MS_MIN" "$CRASH_DELAY_MS_MAX")"
    engine_crash_window_after_ms "$crash_ms"
    # ensure engine is healthy again
    wait_for_engine || die "engine did not come back after crash (iter=$i)"
  fi

  # If kafka is down right now (from outage), bring it up so system can converge
  docker start "$KAFKA_CONTAINER" >/dev/null 2>&1 || true

  # Give some time for WAL publisher + settlement
  sleep "$POST_EVENT_SLEEP_SECS"

  # Assertions
  blue "Asserting invariants for trade_id=$trade_id ..."
  assert_trade_invariants "$trade_id" "$inst"
  green "PASS: trade_id=$trade_id invariants OK"
  passed=$((passed+1))
done

echo
green "ALL DONE: $passed/$ITERATIONS trades satisfied outbox correctness invariants."
echo
echo "Tip: For even harder mode, increase ITERATIONS and probabilities:"
echo "  ITERATIONS=50 CRASH_PROB=0.7 KAFKA_OUTAGE_PROB=0.4 PAUSE_PROB=0.4 ./scripts/outbox_correctness.sh"