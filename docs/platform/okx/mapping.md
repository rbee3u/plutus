# OKX adapter mapping

Package `internal/platform/okx`. OKX has one account, the unified account, so it
has one account structure, `okx_unified`, with the account types `trading`,
`funding`, `earn`, and `dual_investment`. The margin mode is OKX's account mode
(the `acctLv` level). Plutus supports all four levels: Spot mode, futures mode,
Multi-currency margin, and portfolio margin. The adapter observes the level at
validation and at the start of every collection run. It is never chosen in
Plutus ([account model](../../adr/0001-ownership-and-account-boundaries.md)).

Every call is a read-only GET for account, balance, position, earn holdings, or
market data.

A sub-account is its own identity with its own read-only key
([account model](../../adr/0001-ownership-and-account-boundaries.md)). OKX
answers `account/config` with the sub-account's own UID and level, so it
validates like a main account. Each sub-account can use a different margin mode.
Nothing is collected through a main-account key.

The adapter calls `https://openapi.okx.com`. OKX requires accounts registered on
its US or Australian app to use `us.okx.com`, and European ones to use
`eea.okx.com`. Those hosts are not configurable yet.

## Margin modes

`account/config` reports the level as `acctLv`. Every validation and every
collection run read it before any account is collected. The user can switch the
level in the OKX app between two runs. The next run then reports the new margin
mode and maps what that level fills, with no failure.

| `acctLv`  | Margin mode   | OKX name              | What the trading account holds                                                                                         |
| --------- | ------------- | --------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| `1`       | `okx_spot`    | Spot mode             | Spot, spot borrowing once enabled, bought options. No swaps or futures.                                                |
| `2`       | `okx_futures` | Futures mode          | Spot, margin, swaps, futures, options. Margin is assessed per coin, and spot margin borrowings are `MARGIN` positions. |
| `3`       | `okx_mcm`     | Multi-currency margin | Every product, cross-collateralized in USD                                                                             |
| `4`       | `okx_pm`      | Portfolio margin      | As Multi-currency margin, margined by portfolio risk                                                                   |
| Empty     | None          |                       | `normalization`                                                                                                        |
| Any other | None          |                       | `unsupported`, naming the level. The run fails before any account is read.                                             |

Validation returns the account's `uid` as the external reference, plus the
margin mode. Any account structure other than `okx_unified` is refused with
`unsupported` before any call.

Current test coverage, reviewed responses, real gaps, and pending exchange-app
checks are in [OKX verification](./verification.md).

## Endpoints

| Use                                                                 | Endpoint                                                                                                                                     | Signed | OKX limit                |
| ------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- | ------ | ------------------------ |
| Clock sync                                                          | `GET /api/v5/public/time`                                                                                                                    | No     | 10 / 2 s per IP          |
| Validate, and the margin mode read that starts every collection run | `GET /api/v5/account/config`                                                                                                                 | Yes    | 5 / 2 s                  |
| `trading`                                                           | `GET /api/v5/account/balance`, `GET /api/v5/account/positions`                                                                               | Yes    | 10 / 2 s each            |
| Futures contracts                                                   | `GET /api/v5/public/instruments?instType=SWAP` and `?instType=FUTURES`, fetched only for open positions and cached in process for one hour   | No     | 20 / 2 s per IP and type |
| `funding`                                                           | `GET /api/v5/asset/balances`                                                                                                                 | Yes    | 6 / s                    |
| `earn`                                                              | `GET /api/v5/finance/savings/balance` (6 / s), `GET /api/v5/finance/staking-defi/orders-active` (3 / s)                                      | Yes    | As listed, per user ID   |
| `dual_investment`                                                   | `GET /api/v5/finance/sfp/dcd/order-history?limit=100` and its next pages `?endId=<ordId>&limit=100`                                          | Yes    | 1 / s per user ID        |
| Combined earn/dual completeness                                     | `GET /api/v5/asset/asset-valuation?ccy=USDT`                                                                                                 | Yes    | 1 / s per user ID        |
| Prices                                                              | `GET /api/v5/market/tickers?instType=SPOT` (every spot pair in one call, fetched before the account calls so its time precedes every report) | No     | 20 / 2 s per IP          |

Signed requests carry four headers:

- `OK-ACCESS-KEY`;
- `OK-ACCESS-PASSPHRASE`;
- `OK-ACCESS-TIMESTAMP`, in ISO 8601 UTC with milliseconds;
- `OK-ACCESS-SIGN`, the Base64 HMAC-SHA256 of timestamp + method + path with its
  query.

OKX rejects a timestamp more than 30 seconds from its clock. So the timestamp is
corrected by the offset to OKX server time. The offset is measured before the
first signed call, and again after a `50102` or `50112` answer, which is then
retried once.

With all four accounts selected, a collection run makes eight signed calls, plus
one for each further page of dual investment history. It also makes one to four
public calls:

- the tickers, always;
- server time, before the first signed call and after a clock rejection;
- the two futures contract lists, when an open position needs a list that is not
  cached.

## Account report

`trading` reads `account/balance`. OKX returns `""` for a field that does not
apply to the account's level. One mapping serves all four margin modes, and an
empty field stays null, never zero. `totalEq` is required: without it the report
fails with `normalization`.

In the tables below, "yes" marks the fields OKX documents for that mode. A field
OKX leaves empty stays null in any mode.

| Stored fact or read projection        | Source                                                                       | Spot mode | Futures mode | Multi-currency, portfolio margin |
| ------------------------------------- | ---------------------------------------------------------------------------- | --------- | ------------ | -------------------------------- |
| `reported_equity_usd`                 | `totalEq` (USD)                                                              | Yes       | Yes          | Yes                              |
| `adjusted_equity_usd`                 | `adjEq` (USD, after collateral discounts)                                    | Yes       | `null`       | Yes                              |
| `available_equity_usd`                | `availEq`                                                                    | `null`    | `null`       | Yes                              |
| Initial margin ratio (projection)     | Derived: `imr ÷ adjEq`, 8 decimals. Null when `adjEq ≤ 0` or `imr` is empty. | Yes       | `null`       | Yes                              |
| Maintenance margin ratio (projection) | Derived: `mmr ÷ adjEq`, same rules                                           | Yes       | `null`       | Yes                              |
| `reported_margin_ratio`               | `mgnRatio` as reported (`adjEq ÷ mmr`, a multiple where higher is safer)     | Yes       | `null`       | Yes                              |
| `collected_at`                        | Local time when the response arrived                                         | Yes       | Yes          | Yes                              |
| `source_observed_at`                  | `null`                                                                       | `null`    | `null`       | `null`                           |

In futures mode the account level carries only `totalEq` and `isoEq`. OKX
assesses margin per coin (`details[].mgnRatio`, `imr`, `mmr`, `notionalLever`),
and only while a cross position is open. Plutus has no per-coin margin column.
This is an accepted display gap: the equity is still complete.

`funding`, `earn`, and `dual_investment` report no equity. Their value is
`calculated` from balances, and every other column is null.

Not stored: `upl`, `isoEq`, `ordFroz`, `borrowFroz`, `notionalUsd` and its
per-business variants, `delta`, `deltaLever`, `deltaNeutralStatus`, `uTime`, and
portfolio margin's `account/risk-state`.

Raw initial and maintenance requirements (`imr` / `mmr`) are stored in
`initial_margin_requirement_usd` and `maintenance_margin_requirement_usd`. The
read projection derives ratios from these facts and positive
`adjusted_equity_usd`; derived ratios are not persisted as reported ratios.
Missing or nonpositive denominators produce null.

## Balance

`trading` reads `account/balance` `details[]`:

| Plutus column           | Source                                                          | Spot mode | Futures mode | Multi-currency, portfolio margin |
| ----------------------- | --------------------------------------------------------------- | --------- | ------------ | -------------------------------- |
| `AssetSymbol`           | `ccy`                                                           | Yes       | Yes          | Yes                              |
| `equity` (coin units)   | `eq` as reported, may be negative                               | Yes       | Yes          | Yes                              |
| `available_to_trade`    | `availEq`                                                       | `null`    | Yes          | Yes                              |
| `available_to_withdraw` | `availBal`                                                      | Yes       | Yes          | Yes                              |
| `liability`             | `liab`, documented as positive and stored as its absolute value | Yes       | `null`       | Yes                              |
| `interest`              | `interest`                                                      | Yes       | `null`       | Yes                              |
| `unrealized_pnl`        | `upl`                                                           | `null`    | Yes          | Yes                              |
| `name`                  | Not mapped                                                      |           |              |                                  |

`funding` reads `asset/balances`: `AssetSymbol` from `ccy`, `equity` from `bal`,
and `available_to_withdraw` from `availBal`.

`earn` has one row per coin, summing two sources:

- Simple Earn `amt`;
- on-chain earn `investData[].amt`.

Dual principal is summed separately in `dual_investment` (see
[dual investment](#dual-investment)).

The other `funding` and `earn` columns are null. OKX gives no full coin names,
so `name` is never mapped.

Mapping rules:

- **Equity.** `eq` is taken as reported, never rebuilt. OKX does not document
  its composition, and it must not be replaced by `cashBal + upl`. The per-coin
  field remains distinct from the account-wide `totalEq`. It already includes
  isolated margin, unrealized PnL, and option value.
- **Spot mode borrowings.** `liab` already includes the accrued interest,
  according to OKX's help article on Spot mode borrowing. `interest` is shown
  beside it and never added again.
- **Futures mode borrowings.** They exist only on `MARGIN` position rows, which
  are skipped (see [Position](#position)), so no balance shows them.
- **Required amounts.** `eq`, `bal`, each `amt`, and each invested order's
  `notionalSz` are required. An empty one fails its account with
  `normalization`, naming the coin. Read as zero, it would drop the coin under
  the complete set rule. The other fields may be empty and then stay null.
- **Zero rows.** Rows whose numeric fields are all zero are dropped.
- **Simple Earn.** `amt` already includes `loanAmt` and `pendingAmt` and is used
  alone.
- **ETH and SOL staking** (`finance/staking-defi/eth/balance`, `/sol/balance`)
  are not collected. OKX reports those balances account-wide, and the BETH and
  OKSOL they produce are ordinary coins already held in `trading` or `funding`.
  Collecting them again would count them twice.
- **Products without a holdings endpoint** are not itemized: Simple Earn fixed,
  Shark Fin, Snowball, Seagull, Price Lock, Jumpstart, BTC staking, Stable
  Rewards, and OKUSD. Value that OKX counts in its earn valuation trips the
  [earn check](#earn-check).
- **Not mapped:**
  - `trading`: `cashBal`, `disEq`, `eqUsd`, `isoEq`, `frozenBal`, `ordFrozen`,
    `uplLiab`, `crossLiab`, `isoLiab`, `spotBal`, `spotUpl`, `spotInUseAmt`,
    `accAvgPx`, `openAvgPx`, `maxLoan`, `rewardBal`, `fixedBal`, `autoLendAmt`,
    `twap`, `stgyEq`, and futures mode's per-coin `mgnRatio`, `imr`, `mmr`,
    `notionalLever`;
  - earn balance columns remain null; these fields are retained as descriptive
    earn metadata, not additional equity;
  - dual investment: expected payout and settlement amounts; the annualized rate
    is descriptive only.

## Dual investment

The public API documents no separate structured-product cash balance endpoint. A
dual investment order takes its principal out of funding when it is bought
(funding bill type `160`). OKX pays the principal back to funding at settlement
or refund (`161`–`163`). While the order is open, the principal is in no other
account. `dual_investment` counts it in the coin invested
([dual investment decision](../../adr/0005-earn-and-dual-products.md)):
`notionalSz` of `notionalCcy`, added to that coin's dual balance and priced like
any other balance. The yield (`yieldSz`) and a conversion into the other coin
(`settleSz` of `settleCcy`) count only once OKX pays them into funding.

| `state`                                                                                                      | Counted                                                                     |
| ------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------- |
| `initial`, `pending_book`, `live`, `pending_settle`, `pending_redeem_booking`, `pending_redeem`, `redeeming` | Yes                                                                         |
| `settled`, `redeemed`, `rejected`                                                                            | No. The payout or refund is back in funding, or the order was never filled. |
| Any other                                                                                                    | Selected earn and dual accounts fail with `normalization`, naming the state |

The states come from all of OKX's dual investment endpoints. The order history's
own `state` filter accepts only `initial`, `live`, `pending_settle`, `settled`,
`pending_redeem`, `redeemed`, and `rejected`. So the history cannot be filtered
to the invested states, and OKX documents no retention bound. The adapter
therefore reads the whole history, newest first:

1. It requests 100 orders, then the 100 before the oldest `ordId` seen
   (`endId`), 1.1 s apart to respect the one-request-per-second limit.
2. It stops at the first page with fewer than 100 orders.
3. Every page must list strictly decreasing order ids, below the previous page's
   oldest. Otherwise selected earn and dual accounts fail with `normalization`,
   because a page out of order could end the walk before an invested order.
4. If ten full pages (1,000 orders) do not reach the end, selected earn and dual
   accounts fail with `unsupported` and go stale
   ([report policy](../../adr/0002-reports-and-compatible-history.md)). It never
   reads a partial history and drops an older order that is still invested.

**Settlement window.** An order settles at its product's expiry, and OKX pays
out to funding. OKX does not document how long the payout takes. A run inside
that window can go either way:

- The order has already left the counted states and the payout is not yet in
  funding. The value dips.
- The payout is in funding and the order still shows `pending_settle`. The value
  is counted twice.

The next run after both have updated is correct again. Telling the two apart
would need funding's bill history, which Plutus does not read
([capital flow decision](../../adr/0004-value-change-and-boundary-flows.md)).
Both are expected and are not reconciled across accounts. The same window can
fail one run's [earn check](#earn-check) if OKX's valuation still holds an order
the history already shows as settled. Selected earn and dual accounts then stay
stale for that run.

### Dual position fields

Dual account selection, principal valuation, and shared earn completeness follow
[domain rules](../../domain-rules.md#dual-investment-and-earn-completeness).
Platform-specific fields map as follows:

| Dual position field   | Source                                                                                                                                                                                                             |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Subscription identity | `ordId`; `productId` is separate                                                                                                                                                                                   |
| Principal             | `notionalSz` of `notionalCcy`                                                                                                                                                                                      |
| Pair and direction    | `baseCcy` / `quoteCcy`; investing base means `Sell high`, investing quote means `Buy low`                                                                                                                          |
| Target, expiry        | `strike`, scheduled `settleTime` milliseconds. The older documentation example has `expTime` instead, which is used when `settleTime` is empty. `settledTime` is the actual payout time, not the scheduled expiry. |
| Annualized rate       | `annualizedYield`, tagged `annualized_yield`                                                                                                                                                                       |

## Earn position descriptions

Rows from
[Savings balances](https://my.okx.com/docs-v5/en/#financial-product-simple-earn-flexible-get-saving-balance)
remain coin aggregates. `amt` is total holding Amount; `rate` is a minimum
lending preference kept in details, not an actual return. Earnings with
unspecified accumulation scope remain reported rewards in details.
[On-chain orders](https://my.okx.com/docs-v5/en/#financial-product-on-chain-earn-get-active-orders)
retain all nonzero invested currencies in one row, Estimated APY, purchase time,
and native stage. `earningType` distinguishes Estimated and Cumulative rewards;
unknown scopes remain reported rewards. Estimated redemption settlement time is
a detail, never Maturity. These facts describe existing balance equity and do
not change the shared completeness check. See
[ADR 0005](../../adr/0005-earn-and-dual-products.md).

## Earn check

OKX keeps adding earn products. One the adapter cannot read would otherwise
vanish from a successful `earn` report. So after itemizing, the adapter reads
`GET /api/v5/asset/asset-valuation?ccy=USDT`, OKX's own valuation split into
`trading`, `funding`, `earn`, and the deprecated `classic` bucket. It compares
`details.earn` with the combined ordinary earn and dual balances valued at the
batch's prices
([dual investment decision](../../adr/0005-earn-and-dual-products.md)). USDT
counts as USD
([valuation policy](../../adr/0003-valuation-and-asset-policy.md)), the quote
the balances are priced in.

## Position

`account/positions`, every futures contract type in one call.

| Plutus column                             | Source                                                                                             |
| ----------------------------------------- | -------------------------------------------------------------------------------------------------- |
| `position_key`                            | `<instId>:<posSide>`, such as `DOGE-USDT-SWAP:net`                                                 |
| `side`                                    | `posSide` `long` / `short` in long/short mode. In net mode (`posSide = net`), the sign of `pos`.   |
| `source_quantity`, `source_quantity_unit` | `abs(pos)` in contracts                                                                            |
| `entry_price`, `mark_price`               | `avgPx`, `markPx`                                                                                  |
| `unrealized_pnl`                          | `upl`, in the settlement coin                                                                      |
| `liquidation_price`                       | `liqPx`. Null when empty or `0` (portfolio margin cross positions report none).                    |
| `leverage`                                | `lever`. Null when empty or `0` (portfolio margin reports no leverage and fills `0`).              |
| `margin`                                  | Isolated `margin`; non-portfolio cross source `imr`, with verified currency; see provenance below. |

- **Zero rows.** Rows with `pos = 0` are dropped: OKX leaves them behind after
  an isolated-margin transfer.
- **`MARGIN` rows** are skipped in every margin mode. They are borrow-backed
  spot exposure whose value is already in the `trading` balances' `eq`. In
  futures mode they are also the only place OKX reports the borrowing (`liab`,
  `liabCcy`, `interest`), which Plutus therefore does not show.
- **`OPTION` and `EVENTS` rows** fail the `trading` report with `normalization`,
  naming the futures contract, because Plutus has no futures contract kind for
  them. This includes the options a Spot mode account may buy.
- **Spot mode** holds no swaps or futures, so its positions list is empty unless
  it holds options.

| Futures contract column                   | Source (`public/instruments`)                                                                                         |
| ----------------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| `symbol`                                  | `instId`                                                                                                              |
| `kind`                                    | `SWAP` → `linear_perpetual` / `inverse_perpetual`, `FUTURES` → `linear_delivery` / `inverse_delivery`, by `ctType`    |
| `base_asset_symbol`, `quote_asset_symbol` | The two halves of `uly` (`DOGE-USDT`, `BTC-USD`). Contract rows leave `baseCcy` and `quoteCcy` empty.                 |
| `settlement_asset_symbol`                 | `settleCcy`: `USDT` or `USDC` for linear swaps, `USD` for the `-USD_UM-` futures, the base coin for inverse contracts |
| `expiry_at`                               | `expTime`, futures only (`alias` is deprecated)                                                                       |
| `contract_size`                           | `ctVal × ctMult`: base coins per contract for linear contracts, USD per contract for inverse ones                     |

Rows are handled as follows:

- **Skipped futures contracts.** Pre-open rows without an `instId` or a contract
  type are skipped, and so are futures without `expTime`.
- **Unknown contracts.** A position whose contract is missing from a fresh list
  fails the `trading` report with `normalization`, naming the contract. It is
  never dropped.
- **USD notional.** For a linear contract position it is contracts × contract
  size × mark price. For an inverse one it is contracts × contract size.

## Prices

Each non-USD balance symbol is priced at the spot `last` of `<symbol>-USDT`,
else `<symbol>-USDC`, observed when the ticker response arrived.

- **USD equivalents.** USD, USDT, and USDC get no price row
  ([valuation policy](../../adr/0003-valuation-and-asset-policy.md)). Other
  stablecoins such as USDG are priced through their pairs.
- **No pair.** A symbol with neither pair gets no price and is valued as
  `no_price`.
- **Failed ticker call.** The batch has no prices: valuation falls back to the
  latest price within 24 hours, and the earn check is skipped.
- **Positions** carry their own `markPx`. Contracts get no separate price.

## Errors

| OKX answer                                                                                                                                                                                   | Class                                                  | Scope                                                                                     |
| -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------ | ----------------------------------------------------------------------------------------- |
| `50101`, `50103`–`50107`, `50111`, `50113`, `50114`, `50119` (key, signature, passphrase, missing header, wrong environment), `50035`, `50110`, `50121` (IP), `50100` (API frozen), HTTP 401 | `authentication`                                       | The whole batch; the connection becomes `invalid`                                         |
| `50030`, `50120`, `59500` (the key may not use the endpoint)                                                                                                                                 | `authorization`                                        | The whole batch; the connection becomes `invalid`                                         |
| `50011`, `50013`, `50040`, HTTP 429                                                                                                                                                          | `rate_limited`                                         | The account; the batch when every account failed                                          |
| `50001`, `50004`, `50026`, `50046`, `51767`, HTTP 5xx, timeouts, network errors, `50102` or `50112` after one resync                                                                         | `temporary`                                            | The account; the batch when every account failed                                          |
| Any other non-zero `code` or HTTP status                                                                                                                                                     | `temporary`, with OKX's code and message in the detail | The account                                                                               |
| An `acctLv` other than `1`–`4`, or an account structure other than `okx_unified`                                                                                                             | `unsupported`                                          | The whole batch. Not retried; the connection stays enabled while its accounts turn stale. |
| A dual investment history of 1,000 orders or more; an earn valuation above the itemized earn beyond tolerance                                                                                | `unsupported`                                          | Selected `earn` and `dual_investment`, which turn stale                                   |
| An account type other than `trading`, `funding`, `earn`, or `dual_investment`                                                                                                                | `unsupported`                                          | That account                                                                              |
| An empty `acctLv`, or `account/config` without rows                                                                                                                                          | `normalization`                                        | The whole batch                                                                           |
| Unparseable or non-decimal fields, an empty `totalEq`, `eq`, `bal`, or `amt`, unknown contracts, `OPTION` or `EVENTS` positions, `code` `0` without `data`                                   | `normalization`                                        | The account                                                                               |
| An undocumented dual investment state, an order history that is not newest first, an invested order without `notionalCcy` or `notionalSz`, a valuation without `details.earn`                | `normalization`                                        | Selected `earn` and `dual_investment` accounts                                            |

OKX answers many failures with HTTP 200 and a non-zero `code`, so both the
status and the code decide. `code` may be a string or a number. A compound code
such as `51008_1000` is classified by its main code. Error details name the
endpoint and OKX's code and message, cut to 200 characters, never a body or a
key.

### Position presentation provenance

`posSide=net` retains one-way mode; long/short retains hedged mode. `mgnMode`
retains the cross/isolated setting. Non-portfolio cross `imr` is a position
initial requirement; `ccy` is retained rather than universally labeling it USD.
Under portfolio margin, risk-unit `imr` can repeat across member positions and
is not retained as a position allocation. Isolated `margin` is allocated funds;
`ccy` retains its source currency for validation. Effective contract size
remains `ctVal × ctMult`, with base units for linear and quote units for
inverse.

The current
[REST guide](https://app.okx.com/docs-v5/en/#rest-api-trading-account-get-positions)
describes position `imr` as USD, but reviewed futures and Multi-currency inverse
responses match native settlement amounts. The
[official margin rules](https://www.okx.com/en-sg/help/futures-margin-calculation-rules)
also define crypto cross initial requirements in settlement coin. For example,
two USD 10 ETH contracts at mark 2,687.06 and leverage 3 report
`imr=0.0024810263509809`, matching ETH rather than USD. The old USD mapping
would divide this amount by mark a second time.

Non-PM inverse reads validate base=settle, USD-equivalent quote, positive size,
contract size, mark, and initial leverage. They compare the source amount to
both native and USD initial requirements within relative `1e-8` rounding
tolerance. A native-only match uses settle; a USD-only match uses USD and
explicit conversion. Ambiguous or unmatched amounts have unknown currency.
Linear USD-equivalent currencies follow the existing 1:1 policy; incompatible or
missing currency remains unknown. This validation applies to earlier typed-USD
reports with known non-PM regime and provenance without changing persisted
amounts or using the identity's latest mode. No generic calculated fallback
replaces OKX reported hedge or risk-unit treatment.
