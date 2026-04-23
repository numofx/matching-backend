# matching-backend

BTCVAR30 prices are submitted in variance, displayed in volatility, and settled in variance.

Thin offchain backend for the `matching` contracts.

Initial scope:

- one spot market: USDC/cNGN
- one market: BTC convex perp
- one variance market: BTCVAR30-PERP
- one order type: limit order
- one module path: `TradeModule`
- one executor
- one internal Deribit-backed oracle
- one matching loop

This repo is intentionally narrow. It is not a generic exchange backend.

## Responsibilities

- accept and persist signed BTC convex perp orders
- expose a minimal API for order entry and book inspection
- expose a minimal API for BTCVAR30 oracle reads
- run a price-time matching loop
- run a BTCVAR30 oracle poller and funding loop
- submit executor payloads for `Matching.verifyAndMatch(...)`

## Out of Scope

- RFQ
- liquidation
- multi-market support
- websocket market data
- a full frontend
- direct onchain execution from Go

## Layout

```text
cmd/
  api/        HTTP API for orders and health checks
  matcher/    background matching worker
internal/
  api/        HTTP server wiring and handlers
  config/     environment configuration
  db/         Postgres connection helpers
  funding/    BTCVAR30 funding calculation loop
  instruments/ instrument metadata and registry
  marketdata/ Deribit market data client
  matching/   matching loop and orchestration
  oracles/    internal oracle services and persistence
  orders/     order model and repository contracts
migrations/   database schema
```

## Configuration

Copy `.env.example` into your own environment and set the required values.

Important values:

- `DATABASE_URL`
- `API_ADDR`
- `MATCHER_POLL_INTERVAL`
- `CHAIN_ID`
- `MATCHING_ADDRESS`
- `TRADE_MODULE_ADDRESS`
- `BTC_PERP_ASSET_ADDRESS`
- `CNGN_SPOT_ASSET_ADDRESS`
- `CNGN_APR30_2026_FUTURE_ASSET_ADDRESS`
- `CNGN_APR30_2026_FUTURE_SUB_ID`
- optionally `EXPECTED_ORDER_OWNER`
- optionally `EXPECTED_ORDER_SIGNER`
- `EXECUTOR_URL`
- optionally `EXECUTOR_MANAGER_DATA`
- optionally `EXECUTOR_MANAGER_DATA_FILE`
- `DERIBIT_BASE_URL`
- `DERIBIT_WS_URL`
- `BTCVAR30_ENABLED`
- `BTCVAR30_PERP_ASSET_ADDRESS`
- `BTCVAR30_ORACLE_POLL_MS`
- `BTCVAR30_ORACLE_STALE_MS`
- `BTCVAR30_FUNDING_INTERVAL_MS`
- `BTCVAR30_FUNDING_COEFF`
- `BTCVAR30_FUNDING_CAP`
- optionally `BTCVAR30_ORACLE_SIGNING_KEY`

For spot-style `USDC/cNGN`, the market is enabled when `CNGN_SPOT_ASSET_ADDRESS` is set. The registry
resolves this instrument by exact `(asset_address, sub_id=0)` and exposes the canonical market symbol
`USDCcNGN-SPOT`. Human-readable pair formatting remains in display fields such as
`display_name` and `display_label`.

- `contract_type=spot`
- `settlement_type=spot`
- `base_asset_symbol=USDC`
- `quote_asset_symbol=cNGN`

Spot order-entry contract is explicit in market metadata as `order_entry_spec=usdc_cngn_spot_v1`.
That contract is:

- UI price unit: `cNGN per USDC`
- UI size unit: `USDC notional`
- UI side meaning: `BUY` acquires USDC, `SELL` disposes of USDC
- Engine price unit: `USDC per cNGN`
- Engine amount unit: `cNGN amount`
- Engine side policy: invert the UI side

Formulas:

```text
engine_price = 1 / ui_price
engine_amount = ui_size * ui_price
UI BUY  -> engine SELL
UI SELL -> engine BUY
```

Invariant:

```text
ui_size ≈ engine_amount * engine_price
```

Submitters may send raw engine fields only, or may additionally send:

- `order_entry_spec=usdc_cngn_spot_v1`
- `ui_intent.side`
- `ui_intent.price`
- `ui_intent.size`

When those UI fields are present, the API rejects the order unless they map back to the submitted
engine fields under the exact spot contract formulas and side inversion.

Order and trade responses for spot include a normalized `spot_contract` echo with:

- `ui_intent`
- `engine_order`
- `balance_delta`

For the physically delivered `USDC/cNGN APR-30-2026` future, the market is only enabled when both
`CNGN_APR30_2026_FUTURE_ASSET_ADDRESS` and `CNGN_APR30_2026_FUTURE_SUB_ID` are set. The registry
resolves this instrument by exact `(asset_address, sub_id)` and exposes the canonical market symbol
`USDCcNGN-APR30-2026`. Human-readable pair formatting remains in display fields such as
`display_name` and `display_label`.

- `contract_type=deliverable_fx_future`
- `settlement_type=physical_delivery`
- `base_asset_symbol=USDC`
- `quote_asset_symbol=cNGN`

If `EXPECTED_ORDER_OWNER` or `EXPECTED_ORDER_SIGNER` are set, the API rejects orders whose declared owner/signer do not match those configured addresses. The API also validates that `action_json.owner`, `action_json.signer`, `action_json.subaccount_id`, and `action_json.nonce` match the stored order fields.
With `ENFORCE_ACTION_DATA_INVARIANTS=true` (default), the API also rejects orders unless:

- `action_json.data.asset` matches `asset_address`
- `action_json.data.subId` matches `sub_id`
- `action_json.data.isBid` matches `side`
- `action_json.data.limitPrice` and `action_json.data.desiredAmount` are on the same canonical scale as normalized engine fields

Custody requirement for onchain execution:

- Orders submitted for `verifyAndMatch` must reference subaccounts already deposited into `Matching`.
- API pre-submit guard (enabled by default) checks both:
- `SubAccounts.ownerOf(subaccount_id) == MATCHING_ADDRESS`
- `Matching.subAccountToOwner(subaccount_id) != 0x0000000000000000000000000000000000000000`
- If these checks fail, order submit is rejected before persistence/executor.

Relevant env:

- `ENFORCE_MATCHING_CUSTODY=true`
- `ENFORCE_ACTION_DATA_INVARIANTS=true`
- `MATCHING_ADDRESS=0x...`
- `CHAIN_RPC_URL=https://...` (required when custody guard is enabled and matching is configured)

`EXECUTOR_URL` is the endpoint for a separate executor process, likely implemented in
TypeScript with `viem`, that performs simulation and submits `verifyAndMatch(...)`.

`EXECUTOR_MANAGER_DATA` lets the matcher attach the exact `manager_data` hex required by the
executor call. If the blob is too large for an env var, set `EXECUTOR_MANAGER_DATA_FILE`
instead. That file may contain either the raw hex string or a JSON object with a
`manager_data` field.

## BTCVAR30 Oracle

`BTCVAR30` is a first-pass internal oracle sourced from Deribit BTC volatility index data.
For v1, the backend polls Deribit and derives:

```text
variance_30d = (vol_30d / 100)^2
```

The implementation uses Deribit JSON-RPC style market-data methods and keeps the latest
signed payload in memory while persisting history to Postgres.

Relevant docs:

- [Deribit API docs](https://docs.deribit.com/)
- [public/get_volatility_index_data](https://docs.deribit.com/api-reference/market-data/public-get_volatility_index_data)
- [public/get_instruments](https://docs.deribit.com/api-reference/market-data/public-get_instruments)
- [public/get_order_book](https://docs.deribit.com/api-reference/market-data/public-get_order_book)

Public endpoints:

- `GET /oracle/btcvar30/latest`
- `GET /oracle/btcvar30/history?limit=100`
- `GET /v1/orders/{order_id}` (status snapshot for runners/diagnostics)

Example latest response:

```json
{
  "symbol": "BTCVAR30",
  "source": "deribit",
  "timestamp": "2026-03-21T09:00:00Z",
  "vol_30d": 61.25,
  "variance_30d": 0.37515625,
  "methodology_version": "deribit-vol-index-v1",
  "signature": "sha256:...",
  "stale": false
}
```

## BTCVAR30 Funding

`BTCVAR30-PERP` uses the instrument's own conservative mid-price mark and computes funding from:

```text
funding_rate = clamp((mark_price - oracle_variance_30d) * BTCVAR30_FUNDING_COEFF, -BTCVAR30_FUNDING_CAP, BTCVAR30_FUNDING_CAP)
```

`BTCVAR30-PERP` is canonical in the backend and is variance-native end to end:

- engine, matching, executor, funding, and persistence operate on 30D implied variance
- canonical internal price is fixed-point variance ticks
- conversion to vol percent is presentation-only
- all prices are variance; volatility is display-only

Example:

```text
displayed variance price = 0.2728
tick size = 0.0001
internal ticks = 2728
displayed vol percent = sqrt(0.2728) * 100 = 52.23%
```

- `0.25` variance = `50%` implied volatility
- `BTCVAR30` prices are submitted in variance, not vol points

Canonical API example:

```json
{
  "market": "BTCVAR30-PERP",
  "limit_price": 0.2728,
  "variance_price": 0.2728,
  "vol_percent": 52.23,
  "price_semantics": "variance"
}
```

Invariant:

```text
pnl = (var_exit - var_entry) * notional
```

Never:

```text
pnl = (vol_exit - vol_entry) * notional
```

FAQ:

- Why is the UI in vol if the engine is variance?
  Traders think in implied vol, but variance gives linear settlement and linear PnL.
- Why does `0.25` correspond to `50%` vol?
  Because `sqrt(0.25) * 100 = 50`.
- Why is BTCVAR30 still called a vol perpetual externally?
  It is marketed as a vol product while the backend settles in variance.

| Concept | Unit |
|---------|------|
| Canonical submitted price | variance |
| Internal ticks | 0.0001 variance |
| Display mark | vol percent |
| PnL | variance change |
| Funding | variance space |

Safety rules:

- funding pauses if the oracle is stale
- last known oracle value is preserved when Deribit is unavailable
- stale oracle status is logged explicitly
- no last-trade mark is used

Current limitation:

- there is no existing risk engine in this repo, so position caps / leverage caps are not enforced here yet

Expected request body:

```json
{
  "market": "BTCUSDC-CVXPERP",
  "asset_address": "0x...",
  "module_address": "0x...",
  "maker_order_id": "maker-order-id",
  "taker_order_id": "taker-order-id",
  "actions": [
    {
      "subaccount_id": "123",
      "nonce": "1",
      "module": "0x...",
      "data": "0x...",
      "expiry": "1710000000",
      "owner": "0x...",
      "signer": "0x..."
    }
  ],
  "signatures": ["0x..."],
  "order_data": {
    "taker_account": "123",
    "taker_fee": "0",
    "fill_details": [
      {
        "filled_account": "456",
        "amount_filled": "1000000000000000000",
        "price": "78000000000000000000",
        "fee": "0"
      }
    ],
    "manager_data": "0x..."
  }
}
```

The executor may return an empty `2xx` response or JSON like:

```json
{
  "accepted": true,
  "tx_hash": "0x..."
}
```

## Development

Expected local stack:

- Go 1.24+
- PostgreSQL 16+

Suggested flow:

1. Start Postgres.
2. Apply migrations:

```bash
go run ./cmd/migrate
```
3. Run the API:

```bash
env $(cat .env.example | xargs) go run ./cmd/api
```

4. Run the matcher:

```bash
env $(cat .env.example | xargs) go run ./cmd/matcher
```

For a cleaner local env, export the variables from `.env.example` or use your usual dotenv tooling.

## Railway Deploy Contract

Production deploys are expected to run database migrations before the API starts.
This repository encodes that in `railway.toml`:

- Railway builds both the API binary and the migration binary.
- Railway runs `./migrate` as the pre-deploy command.
- Railway starts the service only after the migration step succeeds.

`DATABASE_URL` in Railway should be a reference variable to the Postgres service, for example
`${{Postgres.DATABASE_URL}}`, rather than a copied literal URL.

### EOA-Owned Order Submission

For an EOA-owned deployment, set:

```dotenv
EXPECTED_ORDER_OWNER=0xC7bE60b228b997c23094DdfdD71e22E2DE6C9310
EXPECTED_ORDER_SIGNER=0xC7bE60b228b997c23094DdfdD71e22E2DE6C9310
```

Then submit orders whose top-level fields and `action_json` agree on:

- `owner_address` / `action_json.owner`
- `signer_address` / `action_json.signer`
- `subaccount_id` / `action_json.subaccount_id`
- `nonce` / `action_json.nonce`

Example EOA-owned order templates are in:

- [examples/eoa_taker_order.json](/Users/robertleifke/Code/work/matching-backend/examples/eoa_taker_order.json)
- [examples/eoa_maker_order.json](/Users/robertleifke/Code/work/matching-backend/examples/eoa_maker_order.json)

A helper script is available at:

- [scripts/submit_eoa_order_pair.sh](/Users/robertleifke/Code/work/matching-backend/scripts/submit_eoa_order_pair.sh)

It posts a crossed taker/maker pair to `/v1/orders`, but you still need to provide real `TAKER_ACTION_DATA`, `MAKER_ACTION_DATA`, `TAKER_SIGNATURE`, and `MAKER_SIGNATURE` values for the orders to execute successfully through the onchain matcher.

To reproduce the verified Base dry-run path for BTC convex perp, point the backend at the generated
manager data file from the executor repo:

```dotenv
EXECUTOR_MANAGER_DATA_FILE=/tmp/perp-manager-data.json
```

That file can be generated with:

- [generate_perp_manager_data.mjs](/Users/robertleifke/Code/work/matching-executor/scripts/generate_perp_manager_data.mjs)

The matcher will then forward the `manager_data` blob automatically in every executor payload
instead of hardcoding `0x`.

### Namespace Separation For Cancels

Service-tagged cancels (`/v1/orders/cancel` requests with `service`) are blocked for protected
order namespaces so bot sweeps cannot cancel manual/smoke/validation orders.

- `CANCEL_PROTECTED_ORDER_ID_PREFIXES=validation:,smoke:,manual:`

Manual cancels without a `service` tag are still allowed.

### Production Smoke: Deposited APR Cross

Use the built-in smoke script to run the exact deposited cross flow (`ask 0.001 @ 1390`,
`buy 0.001 @ 1391`) with real signed orders and assert `/v1/trades` increments:

```bash
PRIVATE_KEY=0x... \
./scripts/smoke_deposited_cross.sh
```

The script submits namespaced order IDs (`smoke:apr:...`) so they stay separated from bot order
namespaces and cancel sweeps, and then verifies terminal order state through `GET /v1/orders/{order_id}`.

## First Milestone

The first milestone is one successful matched BTC convex perp trade through `TradeModule`:

1. store two signed crossed orders
2. match them offchain
3. produce executor payloads from stored signed actions
4. send them to the executor
5. update both orders on success

For a focused architecture note covering the oracle, funding loop, and enablement flow, see
- [docs/btcvar30.md](/Users/robertleifke/Code/work/matching-backend/docs/btcvar30.md)
