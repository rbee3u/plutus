# Kraken adapter mapping

Package `internal/platform/kraken`. Kraken supports `kraken_separate` (separate
wallets) and `kraken_unified` (unified wallet). Neither structure has a margin
mode: validation and every batch return an empty one. Every call is a read-only
balance, position, account, or market-data endpoint of Kraken's Spot REST API or
futures REST API, plus one balances snapshot of the futures WebSocket feed.

| Account type                                | `kraken_separate`                                                                                 | `kraken_unified`                                                                  |
| ------------------------------------------- | ------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------- |
| `main` (main)                               | The Spot wallet, with Kraken Rewards (Auto Earn) and the net result of open Spot margin positions | The unified wallet: Spot, Spot margin, and multi-collateral futures in one wallet |
| `earn` (earn)                               | Funds allocated to earn strategies                                                                | Same                                                                              |
| `multi_m` (Multi-M)                         | The multi-collateral futures wallet                                                               | Not offered: it is main                                                           |
| `coin_m` (Coin-M)                           | The single-collateral futures wallets                                                             | Same                                                                              |
| `derivatives_holding` (Derivatives holding) | The futures platform's non-trading holding wallet                                                 | Same                                                                              |

Current test coverage, reviewed responses, real gaps, and pending exchange-app
checks are in [Kraken verification](./verification.md).

Kraken dual investment is not offered by this adapter. Kraken has the product,
but the 2026-10-05 review did not find a published read-only holdings contract
in its Spot or futures OpenAPI specifications. Ordinary earn support does not
establish dual coverage. See the
[contract review and implementation prerequisites](./verification.md#dual-investment-contract-review).

## Credentials

One connection holds a Spot key pair and a futures key pair, replaced together.
Main and earn need the Spot pair; main, Multi-M, Coin-M, and Derivatives holding
need the futures pair, which main uses to check the account structure. An
identity with earn alone needs only the Spot pair. Validation reads `iban` from
the Spot key and `iiban` from the futures key, removes spaces, uppercases both,
and refuses keys of different customers with `authentication`. The identity's
external reference is the Spot `iban`, else the futures `iiban`. A configured
futures pair without a usable `iiban` fails normalization, including when a Spot
pair is present. `accountUid` is not a substitute for the canonical customer
reference. Both configured pairs must establish the same normalized login before
binding or enabling the identity. Read-only key permissions are guidance, not
inspected; use only read-only keys as required by the
[adapter boundary](../../architecture.md#adapter-boundary).

## Endpoints

| Use                                                                                      | Endpoint                                                                                                                                                        | Signed                 |
| ---------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------- |
| Validate: customer reference                                                             | Spot `POST /0/private/GetApiKeyInfo`, futures `GET /api/auth/v1/api-keys/v3/check`                                                                              | Yes                    |
| Account structure check; main in unified wallet, Multi-M, Coin-M, Derivatives holding    | Futures WebSocket `wss://futures.kraken.com/ws/v1`, feed `balances`, its first `balances_snapshot`                                                              | Yes (signed challenge) |
| Main in separate wallets                                                                 | Spot `POST /0/private/BalanceEx`, `POST /0/private/TradeBalance` with `asset=ZUSD`                                                                              | Yes                    |
| Positions, read once per run                                                             | Futures `GET /derivatives/api/v3/openpositions`                                                                                                                 | Yes                    |
| Futures contracts and mark prices, fetched once per run and only when a position is open | Futures `GET /derivatives/api/v3/instruments?contractType=all`, `GET /derivatives/api/v3/tickers`                                                               | No                     |
| `earn`                                                                                   | Spot `POST /0/private/Earn/Strategies`, following `next_cursor`; `POST /0/private/Earn/Allocations` with `converted_asset=USD` and `hide_zero_allocations=true` | Yes                    |
| Asset names                                                                              | Spot `GET /0/public/Assets`                                                                                                                                     | No                     |
| Prices                                                                                   | Spot `GET /0/public/AssetPairs`, `GET /0/public/Ticker` (every pair, one call each, fetched before the account calls so their time precedes every report)       | No                     |

- Spot private requests are POST because Kraken signs a form body. Each carries
  a strictly increasing microsecond `nonce`, `API-Key`, and `API-Sign`: the
  Base64 HMAC-SHA512 of the path and SHA-256(nonce + body), keyed with the
  Base64-decoded secret.
- Futures private requests carry `APIKey` and `Authent`: the Base64 HMAC-SHA512
  of SHA-256(query + path without `/derivatives`), keyed with the decoded
  futures secret.
- The balances feed opens with a `challenge` request; the `subscribe` message
  carries the challenge signed like `Authent`. The adapter reads the first
  `balances_snapshot`, within 30 messages and 20 seconds, and closes the
  connection.
- With every account selected, a unified wallet run reads one balances snapshot
  and makes three signed REST calls (earn strategies, one more per additional
  page; earn allocations; open positions) and three to five public ones.
  Separate wallets add BalanceEx and TradeBalance.

## Account structure check

Kraken reports the structure only as `flex_futures.unified_balances` on the
balances snapshot: `true` is `kraken_unified`, `false` is `kraken_separate`. A
missing flag fails the batch with `normalization`; it is never read as separate
wallets. A structure other than the identity's fails with `unsupported` at
validation when the identity has a futures key, and at the start of every
collection run that selects an account other than earn, before any account is
read ([account model](../../adr/0001-ownership-and-account-boundaries.md)):
"Kraken reports unified_balances = true (unified wallet), but this identity is
configured as separate wallets. Create a new identity for the new structure and
archive this one." The connection stays enabled and its accounts turn stale. A
run with earn alone reads no snapshot: the Spot key cannot read the feed, and
earn is the same account in both structures.

## Account report

| Stored fact or read projection        | Multi-collateral wallet: `main` in unified wallet, `multi_m` (`flex_futures`)                               | `coin_m`                                                                                                 | `main` in separate wallets                                    | `earn`, `derivatives_holding`                                               |
| ------------------------------------- | ----------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------- | --------------------------------------------------------------------------- |
| `reported_equity_usd`                 | `portfolio_value` (USD, with unrealized profit and funding); required                                       | `futures_upnl_cash_value` (USD value of every single-collateral wallet with unrealized profit); required | `null`; calculated value includes `TradeBalance.n`, see below | `null`, so the value is `calculated` from balances                          |
| `adjusted_equity_usd`                 | `margin_equity` (after collateral haircuts)                                                                 | `null`                                                                                                   | `null`                                                        | `null`                                                                      |
| `available_equity_usd`                | `available_margin`                                                                                          | `null`                                                                                                   | `null`                                                        | `null`                                                                      |
| Initial margin ratio (projection)     | Derived: `initial_margin ÷ margin_equity`, 8 decimals; null when `margin_equity ≤ 0` or the field is absent | `null`                                                                                                   | `null`                                                        | `null`                                                                      |
| Maintenance margin ratio (projection) | Derived: `maintenance_margin ÷ margin_equity`, same rules                                                   | `null`                                                                                                   | `null`                                                        | `null`                                                                      |
| `reported_margin_ratio`               | `null`                                                                                                      | `null`                                                                                                   | `null`                                                        | `null`                                                                      |
| `collected_at`                        | Local time after the account collection completes                                                           | Same                                                                                                     | Same                                                          | Same                                                                        |
| `source_observed_at`                  | Snapshot `timestamp` when supplied                                                                          | Same                                                                                                     | `null`                                                        | `null` (`earn`); snapshot `timestamp` when supplied (`derivatives_holding`) |

- `portfolio_value` already holds unrealized profit and funding and is used
  alone. The haircut-adjusted `margin_equity` is only the adjusted equity.
- Main in separate wallets never uses `TradeBalance.e`, which values the whole
  balance by Kraken's own rules. The report stores `TradeBalance.n`, the net
  result of open Spot margin positions in USD, in
  `spot_margin_unrealized_pnl_usd`. Valuation adds that fact to the known
  balance subtotal. An unpriced balance marks the calculated value incomplete
  even when the margin result is nonzero; it does not discard the known subtotal
  or fail normalization. Spot margin positions are not itemized, as with OKX.
- A nonzero `credit_used` on any BalanceEx row fails main with `unsupported`:
  Plutus does not support Kraken credit.
- Not stored: `flex_futures` `balance_value`, `collateral_value`, `pnl`,
  `unrealized_funding`, `total_unrealized`, `total_unrealized_as_margin`,
  `initial_margin_without_orders` at wallet level, `total_position_size`,
  `isolated_list`, and `cross`; isolated position allocation is mapped below;
  the snapshot's `futures_cash_value`; TradeBalance fields other than `n`.

Raw initial and maintenance requirements (`initial_margin` /
`maintenance_margin`) are stored in `initial_margin_requirement_usd` and
`maintenance_margin_requirement_usd`. The read projection derives ratios from
these facts and positive `adjusted_equity_usd`; derived ratios are not persisted
as reported ratios. Missing or nonpositive denominators produce null.

## Balance

| Plutus column         | Multi-collateral wallet (`flex_futures.currencies`) | `coin_m` (snapshot `futures`)      | `derivatives_holding` (snapshot `holding`) | `main` in separate wallets (`BalanceEx`) | `earn` (`Earn/Allocations`)                                                                     |
| --------------------- | --------------------------------------------------- | ---------------------------------- | ------------------------------------------ | ---------------------------------------- | ----------------------------------------------------------------------------------------------- |
| `AssetSymbol`         | Currency code                                       | The wallet's `unit`                | Currency code                              | Asset code                               | `native_asset`                                                                                  |
| `equity` (coin units) | `quantity`                                          | `portfolio_value`, summed per unit | The amount, summed per symbol              | `balance`, summed with its `.F` row      | `amount_allocated.total.native` of `instant`, `bonded`, and `timed` strategies, summed per coin |
| `available_to_trade`  | `available`                                         | `null`                             | `null`                                     | `null`                                   | `null`                                                                                          |
| Other columns         | `null`                                              | `null`                             | `null`                                     | `null`                                   | `null`                                                                                          |

- A code assets lists becomes its `altname` (`XXBT` → `XBT`, `ZUSD` → `USD`),
  then the aliases `XBT` → `BTC` and `XDG` → `DOGE` apply
  ([valuation policy](../../adr/0003-valuation-and-asset-policy.md)). Other
  suffixes, such as `.T` for tokenized assets, are kept. A failed assets call
  fails every account.
- Zero totals are dropped. Every amount is required: a missing or non-decimal
  one fails its account with `normalization`, naming the field; it is never read
  as zero.
- The multi-collateral wallet's derivative profit is in its equity only; it is
  not split among currencies.
- BalanceEx rows ending in `.B` (yield-bearing), `.S` (staked), and `.M` (opt-in
  rewards) are earn allocations and are left to `earn`. `.F` rows are Kraken
  Rewards (Auto Earn), which earn on main balances and stay in main.
  `hold_trade` is part of `balance` and is not subtracted.
- An earn allocation's total already holds its bonding, unbonding, exit-queue,
  and timed amounts; paid rewards are not added again. `flex` strategies are
  Kraken Rewards, reported in main and skipped here. Kraken can return
  historical zero-balance allocations whose strategies are no longer available,
  so earn requests `hide_zero_allocations=true` and ignores any zero total still
  returned. Every total must still be a valid decimal. A nonzero allocation
  whose strategy earn/Strategies does not list, or whose lock type is none of
  `flex`, `instant`, `bonded`, and `timed`, fails earn with `normalization`, as
  does an allocation list with a `next_cursor`, which Kraken documents as
  unpaged. Skipping such a holding would violate the complete set rule.
- `name` is never set: these endpoints report no full asset name.

## Earn position descriptions

Each nonzero
[allocation](https://docs.kraken.com/api-reference/earn/list-earn-allocations)
becomes one strategy aggregate, with `native.total` as allocated Amount.
Strategy estimates retain the source APR range (percentage units normalized to
ratios); `total_rewarded` remains strategy-history Cumulative rewards.
Current-period accumulated and estimated payouts stay separate. Supplied
bonding, pending, exit-queue, unbonding, and timed buckets retain their
quantities and timestamps in details. Their phases do not imply a single
subscription time or Maturity. Descriptions neither create subscriptions nor add
their rewards or bucket amounts to valuation. See
[ADR 0005](../../adr/0005-earn-and-dual-products.md).

## Position

Futures `openpositions` lists every futures wallet's positions. Contracts with
type `flexible_futures`, including `PF_` perpetuals and `FF_` delivery
contracts, belong to the multi-collateral wallet: `main` in unified wallet,
`multi_m` in separate wallets. Inverse and vanilla futures belong to `coin_m`;
routing uses the futures contract type, not its symbol prefix or expiry.

| Plutus column                             | Source                                                                                                                                                                           |
| ----------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `position_key`                            | `<symbol>:<side>`, such as `PF_DOGEUSD:long`                                                                                                                                     |
| `side`                                    | `side`, `long` or `short`                                                                                                                                                        |
| `source_quantity`, `source_quantity_unit` | `abs(size)` in contracts                                                                                                                                                         |
| `entry_price`                             | `price`                                                                                                                                                                          |
| `mark_price`                              | `markPrice` from futures tickers                                                                                                                                                 |
| `unrealized_pnl`                          | `unrealizedPnl` when `pnlCurrency` is the settlement coin, or a USD equivalent for a USD-settled contract; a flexible position without `pnlCurrency` reports USD. Null otherwise |
| `liquidation_price`, `leverage`           | Not mapped                                                                                                                                                                       |

- Rows with `size = 0` are dropped.
- An open option position fails the multi-collateral report with
  `normalization`: Plutus has no futures contract kind for options.
- A position whose contract is missing from the futures contract list, or has an
  undocumented type, fails its report with `normalization`; it is never dropped.

| Futures contract column                   | Source (`instruments`)                                                                                                                                        |
| ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `symbol`                                  | `symbol`                                                                                                                                                      |
| `kind`                                    | `flexible_futures` and `futures_vanilla` → `linear_perpetual`, `futures_inverse` → `inverse_perpetual`; with a `lastTradingTime`, the `*_delivery` equivalent |
| `base_asset_symbol`, `quote_asset_symbol` | `base` and `quote`, else the two halves of `pair` (`XBT:USD`), normalized as balance symbols                                                                  |
| `settlement_asset_symbol`                 | USD for flexible futures, the base coin for inverse futures, the quote for vanilla futures                                                                    |
| `expiry_at`                               | `lastTradingTime`; a contract without one whose ticker `tag` is not `perpetual` fails with `normalization`                                                    |
| `contract_size`                           | `contractSize`, required: base coins per contract for linear contracts, USD per contract for inverse ones                                                     |

## Prices

For every balance symbol other than USD, USDT, and USDC, the Spot last trade
price (`c[0]`) of its USD pair, else its USDT pair, else its USDC pair, observed
when the tickers were fetched. USD, USDT, and USDC get no price row
([valuation policy](../../adr/0003-valuation-and-asset-policy.md)). A symbol
with none of the three pairs gets no price and is valued as `no_price`. A failed
AssetPairs or Ticker call leaves the batch without prices, and valuation falls
back to the latest price within 24 hours. The same prices value main in separate
wallets. Positions carry their own mark price; contracts get no separate price.

## Earn completeness

Kraken is not checked against an earn valuation of its own
([dual investment decision](../../adr/0005-earn-and-dual-products.md)): the only
USD figure it reports, `total_allocated` on earn/Allocations, values the same
allocation list Plutus itemizes, including Kraken Rewards that stay in main. The
list is documented as complete and unpaged, and a nonzero allocation that cannot
be classified fails earn instead of being skipped.

## Errors

| Kraken answer                                                                                                                                                                                                                                                                                                                        | Class                                                  | Scope                                                                           |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------ | ------------------------------------------------------------------------------- |
| Spot `EAPI:Invalid key`, and `EAPI:Invalid signature`; futures `authenticationError`; HTTP 401, which the key check answers for an unknown key (`api_key_not_found`) or a wrong signature (`request_signature_invalid`); a secret that is not Base64; keys of different customers; a key group the selected accounts need is missing | `authentication`                                       | The whole batch; the connection becomes `invalid`                               |
| Spot `EGeneral:Permission denied`; HTTP 403; a refused balances subscription                                                                                                                                                                                                                                                         | `authorization`                                        | The whole batch; the connection becomes `invalid`                               |
| Spot `EAPI:Rate limit exceeded` (and the `EOrder` and `EAuth` variants), `EAuth:Too many requests`, `EService:Throttled`; futures `apiLimitExceeded`; HTTP 429                                                                                                                                                                       | `rate_limited`                                         | The account; the batch when every account failed                                |
| `EAPI:Invalid nonce`, `EService:Unavailable`, HTTP 5xx, timeouts, network errors, a balances feed that ends before its snapshot                                                                                                                                                                                                      | `temporary`                                            | The account, or the batch for the snapshot; the batch when every account failed |
| Any other error                                                                                                                                                                                                                                                                                                                      | `temporary`, detail names the endpoint and HTTP status | The account                                                                     |
| The other account structure; an account type the structure lacks; used credit on main in separate wallets                                                                                                                                                                                                                            | `unsupported`                                          | The whole batch for the structure, otherwise the account; not retried           |
| A missing `unified_balances`; unparseable or non-decimal fields; a missing required field; unknown contracts; option positions in the multi-collateral wallet; unclassified earn allocations; a Spot margin result beside an unpriced balance                                                                                        | `normalization`                                        | The account, or the batch for the structure flag                                |

Kraken answers many Spot failures with HTTP 200 and an `error` list, so both the
status and the list decide. Details name the endpoint, the HTTP status, and the
matched error name ("Invalid key.", or "API key not found." for the key check's
401), never Kraken's text, which can echo the request. On a 401, 403, 429, or
5xx the status decides the class, and a documented error name in the body only
sharpens the detail.

### Position presentation provenance

Kraken contracts are one-way; opposing trades net against the same contract.
Coin-M is cross. For Multi-M/unified main, an explicit isolated map entry
identifies isolated, and `initial_margin_without_orders` supplies the USD funds
set aside for that contract, excluding order reservations and PnL. Kraken
defines this isolated initial amount as allocated funds; this platform-specific
meaning is distinct from a generic initial requirement. A returned complete
empty map identifies cross; an absent map leaves the setting unknown. Cross
wallet totals are not split among positions. No new requests are added.
