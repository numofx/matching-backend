#!/usr/bin/env bash
set -euo pipefail

API_BASE="${API_BASE:-https://markets-service-production.up.railway.app}"
ASSET_ADDRESS="${ASSET_ADDRESS:-0xCE2846771074E20fEc739CF97a60E6075D1E464b}"
SUB_ID="${SUB_ID:-1777507200}"

MATCHING_ADDRESS="${MATCHING_ADDRESS:-0x1599636347FD5bA1fBE21D58AfE0b8B9cbe283FF}"
TRADE_MODULE_ADDRESS="${TRADE_MODULE_ADDRESS:-0x0AAE65AaA66Fe7f54486cDbD007956d3De611990}"
RPC_URL="${RPC_URL:-https://base-sepolia-rpc.publicnode.com}"
CHAIN_ID="${CHAIN_ID:-84532}"
PRIVATE_KEY="${PRIVATE_KEY:-}"
OWNER_PRIVATE_KEY="${OWNER_PRIVATE_KEY:-$PRIVATE_KEY}"
SIGNER_PRIVATE_KEY="${SIGNER_PRIVATE_KEY:-$PRIVATE_KEY}"

EXECUTION_SERVICE_DIR="${EXECUTION_SERVICE_DIR:-../execution-service}"
GENERATOR="${GENERATOR:-$EXECUTION_SERVICE_DIR/scripts/generate_trade_order.mjs}"

ASK_SUBACCOUNT_ID="${ASK_SUBACCOUNT_ID:-6}"
BUY_SUBACCOUNT_ID="${BUY_SUBACCOUNT_ID:-7}"

if [ -z "$PRIVATE_KEY" ] && [ -z "$OWNER_PRIVATE_KEY" ]; then
  echo "PRIVATE_KEY (or OWNER_PRIVATE_KEY) is required" >&2
  exit 1
fi
if [ ! -f "$GENERATOR" ]; then
  echo "generator script not found at $GENERATOR" >&2
  exit 1
fi

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

require_cmd curl
require_cmd jq
require_cmd node

NOW="$(date +%s)"
EXPIRY="$((NOW + 3600))"
ASK_NONCE="$((NOW * 10 + 1))"
BUY_NONCE="$((NOW * 10 + 2))"
ASK_ID="smoke:apr:ask:${NOW}"
BUY_ID="smoke:apr:buy:${NOW}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

TRADES_BEFORE="$(curl -fsS "$API_BASE/v1/trades?asset_address=$ASSET_ADDRESS&sub_id=$SUB_ID")"
BEFORE_COUNT="$(printf '%s' "$TRADES_BEFORE" | jq '.trades | length')"

make_order() {
  local order_id="$1"
  local subaccount_id="$2"
  local nonce="$3"
  local side="$4"
  local ui_limit_price="$5"
  local atomic_limit_price="$6"

  RPC_URL="$RPC_URL" \
  CHAIN_ID="$CHAIN_ID" \
  MATCHING_ADDRESS="$MATCHING_ADDRESS" \
  TRADE_MODULE_ADDRESS="$TRADE_MODULE_ADDRESS" \
  PRIVATE_KEY="$PRIVATE_KEY" \
  OWNER_PRIVATE_KEY="$OWNER_PRIVATE_KEY" \
  SIGNER_PRIVATE_KEY="$SIGNER_PRIVATE_KEY" \
  ORDER_ID="$order_id" \
  SUBACCOUNT_ID="$subaccount_id" \
  RECIPIENT_ID="$subaccount_id" \
  NONCE="$nonce" \
  EXPIRY="$EXPIRY" \
  ASSET_ADDRESS="$ASSET_ADDRESS" \
  SUB_ID="$SUB_ID" \
  LIMIT_PRICE="$atomic_limit_price" \
  DESIRED_AMOUNT="1000000000000000" \
  WORST_FEE="0" \
  SIDE="$side" \
  node "$GENERATOR" > "$TMP_DIR/${order_id}.raw.json"

  jq --arg dp "0.001" --arg lp "$ui_limit_price" \
    'del(.debug) | .desired_amount = $dp | .limit_price = $lp' \
    "$TMP_DIR/${order_id}.raw.json" > "$TMP_DIR/${order_id}.json"
}

make_order "$ASK_ID" "$ASK_SUBACCOUNT_ID" "$ASK_NONCE" "sell" "1390" "1390000000000000000000"
make_order "$BUY_ID" "$BUY_SUBACCOUNT_ID" "$BUY_NONCE" "buy" "1391" "1391000000000000000000"

ASK_RESP="$(curl -fsS -X POST "$API_BASE/v1/orders" -H 'content-type: application/json' --data @"$TMP_DIR/${ASK_ID}.json")"
BUY_RESP="$(curl -fsS -X POST "$API_BASE/v1/orders" -H 'content-type: application/json' --data @"$TMP_DIR/${BUY_ID}.json")"

if [ "$(printf '%s' "$ASK_RESP" | jq -r '.order.order_id // empty')" != "$ASK_ID" ]; then
  echo "ask submit failed: $ASK_RESP" >&2
  exit 1
fi
if [ "$(printf '%s' "$BUY_RESP" | jq -r '.order.order_id // empty')" != "$BUY_ID" ]; then
  echo "buy submit failed: $BUY_RESP" >&2
  exit 1
fi

ATTEMPTS=20
SLEEP_SECS=1
AFTER_COUNT="$BEFORE_COUNT"
LATEST_TRADE=""
for _ in $(seq 1 "$ATTEMPTS"); do
  TRADES_AFTER="$(curl -fsS "$API_BASE/v1/trades?asset_address=$ASSET_ADDRESS&sub_id=$SUB_ID")"
  AFTER_COUNT="$(printf '%s' "$TRADES_AFTER" | jq '.trades | length')"
  LATEST_TRADE="$(printf '%s' "$TRADES_AFTER" | jq '.trades[0]')"
  if [ "$AFTER_COUNT" -gt "$BEFORE_COUNT" ]; then
    break
  fi
  sleep "$SLEEP_SECS"
done

if [ "$AFTER_COUNT" -le "$BEFORE_COUNT" ]; then
  echo "smoke failed: /v1/trades did not increment (before=$BEFORE_COUNT after=$AFTER_COUNT)" >&2
  exit 1
fi

TAKER_ID="$(printf '%s' "$LATEST_TRADE" | jq -r '.taker_order_id // empty')"
MAKER_ID="$(printf '%s' "$LATEST_TRADE" | jq -r '.maker_order_id // empty')"
if [ "$TAKER_ID" != "$BUY_ID" ] || [ "$MAKER_ID" != "$ASK_ID" ]; then
  echo "smoke failed: latest trade mismatch taker=$TAKER_ID maker=$MAKER_ID expected=$BUY_ID/$ASK_ID" >&2
  exit 1
fi

BOOK="$(curl -fsS "$API_BASE/v1/book?asset_address=$ASSET_ADDRESS&sub_id=$SUB_ID")"
if printf '%s' "$BOOK" | rg -q "$ASK_ID|$BUY_ID"; then
  echo "smoke failed: one or both smoke orders still resting in book" >&2
  exit 1
fi

ORDER_STATUS_TIMEOUT_SECS="${ORDER_STATUS_TIMEOUT_SECS:-20}"
ORDER_STATUS_POLL_SECS="${ORDER_STATUS_POLL_SECS:-1}"
TERMINAL=0
elapsed=0
while [ "$elapsed" -lt "$ORDER_STATUS_TIMEOUT_SECS" ]; do
  ASK_STATUS_JSON="$(curl -fsS "$API_BASE/v1/orders/$ASK_ID")" || {
    echo "smoke failed: /v1/orders/$ASK_ID returned non-2xx" >&2
    exit 1
  }
  BUY_STATUS_JSON="$(curl -fsS "$API_BASE/v1/orders/$BUY_ID")" || {
    echo "smoke failed: /v1/orders/$BUY_ID returned non-2xx" >&2
    exit 1
  }
  ASK_STATUS="$(printf '%s' "$ASK_STATUS_JSON" | jq -r '.status')"
  BUY_STATUS="$(printf '%s' "$BUY_STATUS_JSON" | jq -r '.status')"

  if [ "$ASK_STATUS" != "active" ] && [ "$ASK_STATUS" != "matching" ] && [ "$ASK_STATUS" != "match_failed" ] \
    && [ "$BUY_STATUS" != "active" ] && [ "$BUY_STATUS" != "matching" ] && [ "$BUY_STATUS" != "match_failed" ]; then
    TERMINAL=1
    break
  fi

  sleep "$ORDER_STATUS_POLL_SECS"
  elapsed=$((elapsed + ORDER_STATUS_POLL_SECS))
done

if [ "$TERMINAL" -ne 1 ]; then
  echo "smoke failed: non-terminal statuses persisted after timeout ask=$ASK_STATUS buy=$BUY_STATUS" >&2
  exit 1
fi

echo "smoke passed"
echo "order_ids: $ASK_ID $BUY_ID"
echo "trade_count: $BEFORE_COUNT -> $AFTER_COUNT"
echo "latest_trade: $(printf '%s' "$LATEST_TRADE" | jq -c .)"
