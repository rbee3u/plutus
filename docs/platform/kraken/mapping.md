# Kraken Adapter mapping

Package `internal/platform/kraken`. Kraken supports `kraken_separate` (Separate
wallets) and `kraken_unified` (Unified Wallet). Neither structure has a Margin
Mode: validation and every batch return an empty one. Every call is a read-only
balance, position, account, or market-data endpoint of Kraken's Spot REST API or
Futures REST API, plus one balances snapshot of the Futures WebSocket feed.

| Account Type                                | `kraken_separate`                                                                                 | `kraken_unified`                                                                  |
| ------------------------------------------- | ------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------- |
| `main` (Main)                               | the Spot wallet, with Kraken Rewards (Auto Earn) and the net result of open Spot Margin positions | the Unified Wallet: Spot, Spot Margin, and multi-collateral futures in one wallet |
| `earn` (Earn)                               | funds allocated to Earn strategies                                                                | same                                                                              |
| `multi_m` (Multi-M)                         | the multi-collateral futures wallet                                                               | not offered: it is Main                                                           |
| `coin_m` (Coin-M)                           | the single-collateral futures wallets                                                             | same                                                                              |
| `derivatives_holding` (Derivatives Holding) | the futures platform's non-trading holding wallet                                                 | same                                                                              |

Current test coverage, reviewed responses, real gaps, and pending exchange-app
checks are in [Kraken verification](./verification.md).

Kraken Dual Investment is not offered by this Adapter. Kraken has the product,
but the 2026-10-05 review did not find a published read-only holdings contract
in its Spot or Futures OpenAPI specifications. Ordinary Earn support does not
establish Dual coverage. See the
[contract review and implementation prerequisites](./verification.md#dual-investment-contract-review).

## Credentials

One Connection holds a Spot key pair and a Futures key pair, replaced together.
Main and Earn need the Spot pair; Main, Multi-M, Coin-M, and Derivatives Holding
need the Futures pair, which Main uses to check the Account Structure. An
Identity with Earn alone needs only the Spot pair. Validation reads `iban` from
the Spot key and `iiban` from the Futures key, removes spaces, uppercases both,
and refuses keys of different customers with `authentication`. The Identity's
external reference is the Spot `iban`, else the Futures `iiban`. A configured
Futures pair without a usable `iiban` fails normalization, including when a Spot
pair is present. `accountUid` is not a substitute for the canonical customer
reference. Both configured pairs must establish the same normalized login before
binding or enabling the Identity. Read-only key permissions are guidance, not
inspected; use only read-only keys as required by the
[Adapter boundary](../../architecture.md#adapter-boundary).

## Endpoints

| Use                                                                                      | Endpoint                                                                                                                                                        | Signed                 |
| ---------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------- |
| Validate: customer reference                                                             | Spot `POST /0/private/GetApiKeyInfo`, Futures `GET /api/auth/v1/api-keys/v3/check`                                                                              | yes                    |
| Account Structure check; Main in Unified Wallet, Multi-M, Coin-M, Derivatives Holding    | Futures WebSocket `wss://futures.kraken.com/ws/v1`, feed `balances`, its first `balances_snapshot`                                                              | yes (signed challenge) |
| Main in Separate wallets                                                                 | Spot `POST /0/private/BalanceEx`, `POST /0/private/TradeBalance` with `asset=ZUSD`                                                                              | yes                    |
| Positions, read once per run                                                             | Futures `GET /derivatives/api/v3/openpositions`                                                                                                                 | yes                    |
| Futures Contracts and mark prices, fetched once per run and only when a position is open | Futures `GET /derivatives/api/v3/instruments?contractType=all`, `GET /derivatives/api/v3/tickers`                                                               | no                     |
| `earn`                                                                                   | Spot `POST /0/private/Earn/Strategies`, following `next_cursor`; `POST /0/private/Earn/Allocations` with `converted_asset=USD` and `hide_zero_allocations=true` | yes                    |
| Asset names                                                                              | Spot `GET /0/public/Assets`                                                                                                                                     | no                     |
| Prices                                                                                   | Spot `GET /0/public/AssetPairs`, `GET /0/public/Ticker` (every pair, one call each, fetched before the Account calls so their time precedes every report)       | no                     |

- Spot private requests are POST because Kraken signs a form body. Each carries
  a strictly increasing microsecond `nonce`, `API-Key`, and `API-Sign`: the
  Base64 HMAC-SHA512 of the path and SHA-256(nonce + body), keyed with the
  Base64-decoded secret.
- Futures private requests carry `APIKey` and `Authent`: the Base64 HMAC-SHA512
  of SHA-256(query + path without `/derivatives`), keyed with the decoded
  Futures secret.
- The balances feed opens with a `challenge` request; the `subscribe` message
  carries the challenge signed like `Authent`. The adapter reads the first
  `balances_snapshot`, within 30 messages and 20 seconds, and closes the
  connection.
- With every Account selected, a Unified Wallet run reads one balances snapshot
  and makes three signed REST calls (Earn strategies, one more per additional
  page; Earn allocations; open positions) and three to five public ones.
  Separate wallets add BalanceEx and TradeBalance.

## Account Structure check

Kraken reports the structure only as `flex_futures.unified_balances` on the
balances snapshot: `true` is `kraken_unified`, `false` is `kraken_separate`. A
missing flag fails the batch with `normalization`; it is never read as Separate
wallets. A structure other than the Identity's fails with `unsupported` at
validation when the Identity has a Futures key, and at the start of every
Collection Run that selects an Account other than Earn, before any Account is
read ([Account model](../../adr/0001-ownership-and-account-boundaries.md)):
"Kraken reports unified_balances = true (Unified Wallet), but this Identity is
configured as Separate wallets. Create a new Identity for the new structure and
archive this one." The Connection stays enabled and its Accounts turn stale. A
run with Earn alone reads no snapshot: the Spot key cannot read the feed, and
Earn is the same Account in both structures.

## Account Report

| Stored fact or read projection        | multi-collateral wallet: `main` in Unified Wallet, `multi_m` (`flex_futures`)                               | `coin_m`                                                                                                 | `main` in Separate wallets                                  | `earn`, `derivatives_holding`                                             |
| ------------------------------------- | ----------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------- | ------------------------------------------------------------------------- |
| `reported_equity_usd`                 | `portfolio_value` (USD, with unrealized profit and funding); required                                       | `futures_upnl_cash_value` (USD value of every single-collateral wallet with unrealized profit); required | null; calculated value includes `TradeBalance.n`, see below | null, so the value is `calculated` from Balances                          |
| `adjusted_equity_usd`                 | `margin_equity` (after collateral haircuts)                                                                 | null                                                                                                     | null                                                        | null                                                                      |
| `available_equity_usd`                | `available_margin`                                                                                          | null                                                                                                     | null                                                        | null                                                                      |
| Initial margin ratio (projection)     | derived: `initial_margin ÷ margin_equity`, 8 decimals; null when `margin_equity ≤ 0` or the field is absent | null                                                                                                     | null                                                        | null                                                                      |
| Maintenance margin ratio (projection) | derived: `maintenance_margin ÷ margin_equity`, same rules                                                   | null                                                                                                     | null                                                        | null                                                                      |
| `reported_margin_ratio`               | null                                                                                                        | null                                                                                                     | null                                                        | null                                                                      |
| `collected_at`                        | local time after the Account collection completes                                                           | same                                                                                                     | same                                                        | same                                                                      |
| `source_observed_at`                  | snapshot `timestamp` when supplied                                                                          | same                                                                                                     | null                                                        | null (`earn`); snapshot `timestamp` when supplied (`derivatives_holding`) |

- `portfolio_value` already holds unrealized profit and funding and is used
  alone. The haircut-adjusted `margin_equity` is only the adjusted equity.
- Main in Separate wallets never uses `TradeBalance.e`, which values the whole
  balance by Kraken's own rules. The report stores `TradeBalance.n`, the net
  result of open Spot Margin positions in USD, in
  `spot_margin_unrealized_pnl_usd`. Valuation adds that fact to the known
  Balance subtotal. An unpriced Balance marks the calculated value incomplete
  even when the margin result is nonzero; it does not discard the known subtotal
  or fail normalization. Spot Margin positions are not itemized, as with OKX.
- A nonzero `credit_used` on any BalanceEx row fails Main with `unsupported`:
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

| Plutus column         | multi-collateral wallet (`flex_futures.currencies`) | `coin_m` (snapshot `futures`)      | `derivatives_holding` (snapshot `holding`) | `main` in Separate wallets (`BalanceEx`) | `earn` (`Earn/Allocations`)                                                                     |
| --------------------- | --------------------------------------------------- | ---------------------------------- | ------------------------------------------ | ---------------------------------------- | ----------------------------------------------------------------------------------------------- |
| `AssetSymbol`         | currency code                                       | the wallet's `unit`                | currency code                              | asset code                               | `native_asset`                                                                                  |
| `equity` (coin units) | `quantity`                                          | `portfolio_value`, summed per unit | the amount, summed per symbol              | `balance`, summed with its `.F` row      | `amount_allocated.total.native` of `instant`, `bonded`, and `timed` strategies, summed per coin |
| `available_to_trade`  | `available`                                         | null                               | null                                       | null                                     | null                                                                                            |
| other columns         | null                                                | null                               | null                                       | null                                     | null                                                                                            |

- A code Assets lists becomes its `altname` (`XXBT` → `XBT`, `ZUSD` → `USD`),
  then the aliases `XBT` → `BTC` and `XDG` → `DOGE` apply
  ([Valuation policy](../../adr/0003-valuation-and-asset-policy.md)). Other
  suffixes, such as `.T` for tokenized assets, are kept. A failed Assets call
  fails every Account.
- Zero totals are dropped. Every amount is required: a missing or non-decimal
  one fails its Account with `normalization`, naming the field; it is never read
  as zero.
- The multi-collateral wallet's derivative profit is in its equity only; it is
  not split among currencies.
- BalanceEx rows ending in `.B` (yield-bearing), `.S` (staked), and `.M` (opt-in
  rewards) are Earn allocations and are left to `earn`. `.F` rows are Kraken
  Rewards (Auto Earn), which earn on Main balances and stay in Main.
  `hold_trade` is part of `balance` and is not subtracted.
- An Earn allocation's total already holds its bonding, unbonding, exit-queue,
  and timed amounts; paid rewards are not added again. `flex` strategies are
  Kraken Rewards, reported in Main and skipped here. Kraken can return
  historical zero-balance allocations whose strategies are no longer available,
  so Earn requests `hide_zero_allocations=true` and ignores any zero total still
  returned. Every total must still be a valid decimal. A nonzero allocation
  whose strategy Earn/Strategies does not list, or whose lock type is none of
  `flex`, `instant`, `bonded`, and `timed`, fails Earn with `normalization`, as
  does an allocation list with a `next_cursor`, which Kraken documents as
  unpaged. Skipping such a holding would violate the Complete Set rule.
- `name` is never set: these endpoints report no full asset name.

## Earn Position descriptions

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
contracts, belong to the multi-collateral wallet: `main` in Unified Wallet,
`multi_m` in Separate wallets. Inverse and vanilla futures belong to `coin_m`;
routing uses the futures contract type, not its symbol prefix or expiry.

| Plutus column                             | Source                                                                                                                                                                           |
| ----------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `position_key`                            | `<symbol>:<side>`, such as `PF_DOGEUSD:long`                                                                                                                                     |
| `side`                                    | `side`, `long` or `short`                                                                                                                                                        |
| `source_quantity`, `source_quantity_unit` | `abs(size)` in contracts                                                                                                                                                         |
| `entry_price`                             | `price`                                                                                                                                                                          |
| `mark_price`                              | `markPrice` from Futures tickers                                                                                                                                                 |
| `unrealized_pnl`                          | `unrealizedPnl` when `pnlCurrency` is the settlement coin, or a USD equivalent for a USD-settled contract; a flexible position without `pnlCurrency` reports USD. Null otherwise |
| `liquidation_price`, `leverage`           | not mapped                                                                                                                                                                       |

- Rows with `size = 0` are dropped.
- An open option position fails the multi-collateral report with
  `normalization`: Plutus has no Futures Contract kind for options.
- A position whose contract is missing from the futures contract list, or has an
  undocumented type, fails its report with `normalization`; it is never dropped.

| Futures Contract column                   | Source (`instruments`)                                                                                                                                        |
| ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `symbol`                                  | `symbol`                                                                                                                                                      |
| `kind`                                    | `flexible_futures` and `futures_vanilla` → `linear_perpetual`, `futures_inverse` → `inverse_perpetual`; with a `lastTradingTime`, the `*_delivery` equivalent |
| `base_asset_symbol`, `quote_asset_symbol` | `base` and `quote`, else the two halves of `pair` (`XBT:USD`), normalized as Balance symbols                                                                  |
| `settlement_asset_symbol`                 | USD for flexible futures, the base coin for inverse futures, the quote for vanilla futures                                                                    |
| `expiry_at`                               | `lastTradingTime`; a contract without one whose ticker `tag` is not `perpetual` fails with `normalization`                                                    |
| `contract_size`                           | `contractSize`, required: base coins per contract for linear contracts, USD per contract for inverse ones                                                     |

## Prices

For every Balance symbol other than USD, USDT, and USDC, the Spot last trade
price (`c[0]`) of its USD pair, else its USDT pair, else its USDC pair, observed
when the tickers were fetched. USD, USDT, and USDC get no price row
([Valuation policy](../../adr/0003-valuation-and-asset-policy.md)). A symbol
with none of the three pairs gets no price and is valued as `no_price`. A failed
AssetPairs or Ticker call leaves the batch without prices, and valuation falls
back to the latest price within 24 hours. The same prices value Main in Separate
wallets. Positions carry their own mark price; contracts get no separate price.

## Earn completeness

Kraken is not checked against an Earn valuation of its own
([Dual Investment decision](../../adr/0005-earn-and-dual-products.md)): the only
USD figure it reports, `total_allocated` on Earn/Allocations, values the same
allocation list Plutus itemizes, including Kraken Rewards that stay in Main. The
list is documented as complete and unpaged, and a nonzero allocation that cannot
be classified fails Earn instead of being skipped.

## Errors

| Kraken answer                                                                                                                                                                                                                                                                                                                        | Class                                                  | Scope                                                                           |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------ | ------------------------------------------------------------------------------- |
| Spot `EAPI:Invalid key`, and `EAPI:Invalid signature`; Futures `authenticationError`; HTTP 401, which the key check answers for an unknown key (`api_key_not_found`) or a wrong signature (`request_signature_invalid`); a secret that is not Base64; keys of different customers; a key group the selected Accounts need is missing | `authentication`                                       | the whole batch; the Connection becomes `invalid`                               |
| Spot `EGeneral:Permission denied`; HTTP 403; a refused balances subscription                                                                                                                                                                                                                                                         | `authorization`                                        | the whole batch; the Connection becomes `invalid`                               |
| Spot `EAPI:Rate limit exceeded` (and the `EOrder` and `EAuth` variants), `EAuth:Too many requests`, `EService:Throttled`; Futures `apiLimitExceeded`; HTTP 429                                                                                                                                                                       | `rate_limited`                                         | the Account; the batch when every Account failed                                |
| `EAPI:Invalid nonce`, `EService:Unavailable`, HTTP 5xx, timeouts, network errors, a balances feed that ends before its snapshot                                                                                                                                                                                                      | `temporary`                                            | the Account, or the batch for the snapshot; the batch when every Account failed |
| any other error                                                                                                                                                                                                                                                                                                                      | `temporary`, detail names the endpoint and HTTP status | the Account                                                                     |
| the other Account Structure; an Account Type the structure lacks; used credit on Main in Separate wallets                                                                                                                                                                                                                            | `unsupported`                                          | the whole batch for the structure, otherwise the Account; not retried           |
| a missing `unified_balances`; unparseable or non-decimal fields; a missing required field; unknown contracts; option positions in the multi-collateral wallet; unclassified Earn allocations; a Spot Margin result beside an unpriced Balance                                                                                        | `normalization`                                        | the Account, or the batch for the structure flag                                |

Kraken answers many Spot failures with HTTP 200 and an `error` list, so both the
status and the list decide. Details name the endpoint, the HTTP status, and the
matched error name ("Invalid key.", or "API key not found." for the key check's
401), never Kraken's text, which can echo the request. On a 401, 403, 429, or
5xx the status decides the class, and a documented error name in the body only
sharpens the detail.

### Position presentation provenance

Kraken contracts are one-way; opposing trades net against the same contract.
Coin-M is Cross. For Multi-M/Unified Main, an explicit isolated map entry
identifies Isolated, and `initial_margin_without_orders` supplies the USD funds
set aside for that contract, excluding order reservations and PnL. Kraken
defines this isolated initial amount as allocated funds; this platform-specific
meaning is distinct from a generic initial requirement. A returned complete
empty map identifies Cross; an absent map leaves the setting unknown. Cross
wallet totals are not split among positions. No new requests are added.
