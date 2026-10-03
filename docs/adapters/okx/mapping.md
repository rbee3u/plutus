# OKX Adapter mapping

Package `internal/adapters/okx`. OKX has one account, the Unified account, so it
has one Account Structure, `okx_unified`, with the Account Types `trading`,
`funding`, `earn`, and `dual_investment`. The Margin Mode is OKX's account mode
(the `acctLv` level). Plutus supports all four levels: Spot mode, Futures mode,
Multi-currency margin, and Portfolio margin. The adapter observes the level at
validation and at the start of every Collection Run. It is never chosen in
Plutus ([Account model](../../adr/0001-ownership-and-accounts.md)).

Every call is a read-only GET for account, balance, position, Earn holdings, or
market data.

A sub-account is its own Identity with its own read-only key
([Account model](../../adr/0001-ownership-and-accounts.md)). OKX answers
`account/config` with the sub-account's own UID and level, so it validates like
a main account. Each sub-account can use a different Margin Mode. Nothing is
collected through a main-account key.

The adapter calls `https://openapi.okx.com`. OKX requires accounts registered on
its US or Australian app to use `us.okx.com`, and European ones to use
`eea.okx.com`. Those hosts are not configurable yet.

## Margin Modes

`account/config` reports the level as `acctLv`. Every validation and every
Collection Run read it before any Account is collected. The User can switch the
level in the OKX app between two runs. The next run then reports the new Margin
Mode and maps what that level fills, with no failure.

| `acctLv`  | Margin Mode   | OKX name              | What the trading account holds                                                                                         |
| --------- | ------------- | --------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| `1`       | `okx_spot`    | Spot mode             | Spot, spot borrowing once enabled, bought options. No swaps or futures.                                                |
| `2`       | `okx_futures` | Futures mode          | Spot, margin, swaps, futures, options. Margin is assessed per coin, and spot margin borrowings are `MARGIN` positions. |
| `3`       | `okx_mcm`     | Multi-currency margin | Every product, cross-collateralized in USD                                                                             |
| `4`       | `okx_pm`      | Portfolio margin      | As Multi-currency margin, margined by portfolio risk                                                                   |
| empty     | none          |                       | `normalization`                                                                                                        |
| any other | none          |                       | `unsupported`, naming the level. The run fails before any Account is read.                                             |

Validation returns the account's `uid` as the External Ref, plus the Margin
Mode. Any Account Structure other than `okx_unified` is refused with
`unsupported` before any call.

Current test coverage, reviewed responses, real gaps, and pending exchange-app
checks are in [OKX verification](./verification.md).

## Endpoints

| Use                                                                 | Endpoint                                                                                                                                     | Signed | OKX limit                |
| ------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- | ------ | ------------------------ |
| Clock sync                                                          | `GET /api/v5/public/time`                                                                                                                    | no     | 10 / 2 s per IP          |
| Validate, and the Margin Mode read that starts every Collection Run | `GET /api/v5/account/config`                                                                                                                 | yes    | 5 / 2 s                  |
| `trading`                                                           | `GET /api/v5/account/balance`, `GET /api/v5/account/positions`                                                                               | yes    | 10 / 2 s each            |
| Instruments                                                         | `GET /api/v5/public/instruments?instType=SWAP` and `?instType=FUTURES`, fetched only for open positions and cached in process for one hour   | no     | 20 / 2 s per IP and type |
| `funding`                                                           | `GET /api/v5/asset/balances`                                                                                                                 | yes    | 6 / s                    |
| `earn`                                                              | `GET /api/v5/finance/savings/balance` (6 / s), `GET /api/v5/finance/staking-defi/orders-active` (3 / s)                                      | yes    | as listed, per User ID   |
| `dual_investment`                                                   | `GET /api/v5/finance/sfp/dcd/order-history?limit=100` and its next pages `?endId=<ordId>&limit=100`                                          | yes    | 1 / s per User ID        |
| Combined Earn/Dual completeness                                     | `GET /api/v5/asset/asset-valuation?ccy=USDT`                                                                                                 | yes    | 1 / s per User ID        |
| Prices                                                              | `GET /api/v5/market/tickers?instType=SPOT` (every spot pair in one call, fetched before the Account calls so its time precedes every report) | no     | 20 / 2 s per IP          |

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

With all four Accounts selected, a Collection Run makes eight signed calls, plus
one for each further page of dual investment history. It also makes one to four
public calls:

- the tickers, always;
- server time, before the first signed call and after a clock rejection;
- the two instrument lists, when an open position needs a list that is not
  cached.

## Account Report

`trading` reads `account/balance`. OKX returns `""` for a field that does not
apply to the account's level. One mapping serves all four Margin Modes, and an
empty field stays null, never zero. `totalEq` is required: without it the report
fails with `normalization`.

In the tables below, "yes" marks the fields OKX documents for that mode. A field
OKX leaves empty stays null in any mode.

| Plutus column              | Source                                                                       | Spot mode | Futures mode | Multi-currency, Portfolio margin |
| -------------------------- | ---------------------------------------------------------------------------- | --------- | ------------ | -------------------------------- |
| `equity`                   | `totalEq` (USD)                                                              | yes       | yes          | yes                              |
| `adjusted_equity`          | `adjEq` (USD, after collateral discounts)                                    | yes       | null         | yes                              |
| `available_equity`         | `availEq`                                                                    | null      | null         | yes                              |
| `initial_margin_ratio`     | derived: `imr ÷ adjEq`, 8 decimals. Null when `adjEq ≤ 0` or `imr` is empty. | yes       | null         | yes                              |
| `maintenance_margin_ratio` | derived: `mmr ÷ adjEq`, same rules                                           | yes       | null         | yes                              |
| `unified_margin_ratio`     | `mgnRatio` as reported (`adjEq ÷ mmr`, a multiple where higher is safer)     | yes       | null         | yes                              |
| `observed_at`              | local time when the response arrived                                         | yes       | yes          | yes                              |

In Futures mode the account level carries only `totalEq` and `isoEq`. OKX
assesses margin per coin (`details[].mgnRatio`, `imr`, `mmr`, `notionalLever`),
and only while a cross position is open. Plutus has no per-coin margin column.
This is an accepted display gap: the equity is still complete.

`funding`, `earn`, and `dual_investment` report no equity. Their value is
`calculated` from Balances, and every other column is null.

Not stored: `upl`, `isoEq`, `ordFroz`, `borrowFroz`, `notionalUsd` and its
per-business variants, `delta`, `deltaLever`, `deltaNeutralStatus`, `uTime`, and
portfolio margin's `account/risk-state`.

## Balance

`trading` reads `account/balance` `details[]`:

| Plutus column           | Source                                                          | Spot mode | Futures mode | Multi-currency, Portfolio margin |
| ----------------------- | --------------------------------------------------------------- | --------- | ------------ | -------------------------------- |
| `symbol`                | `ccy`                                                           | yes       | yes          | yes                              |
| `equity` (coin units)   | `eq` as reported, may be negative                               | yes       | yes          | yes                              |
| `available_to_trade`    | `availEq`                                                       | null      | yes          | yes                              |
| `available_to_withdraw` | `availBal`                                                      | yes       | yes          | yes                              |
| `liability`             | `liab`, documented as positive and stored as its absolute value | yes       | null         | yes                              |
| `interest`              | `interest`                                                      | yes       | null         | yes                              |
| `unrealized_pnl`        | `upl`                                                           | null      | yes          | yes                              |
| `name`                  | not mapped                                                      |           |              |                                  |

`funding` reads `asset/balances`: `symbol` from `ccy`, `equity` from `bal`, and
`available_to_withdraw` from `availBal`.

`earn` has one row per coin, summing two sources:

- Simple Earn `amt`;
- on-chain earn `investData[].amt`.

Dual principal is summed separately in `dual_investment` (see
[Dual investment](#dual-investment)).

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
  are skipped (see [Position](#position)), so no Balance shows them.
- **Required amounts.** `eq`, `bal`, each `amt`, and each invested order's
  `notionalSz` are required. An empty one fails its Account with
  `normalization`, naming the coin. Read as zero, it would drop the coin under
  the Complete Set rule. The other fields may be empty and then stay null.
- **Zero rows.** Rows whose numeric fields are all zero are dropped.
- **Simple Earn.** `amt` already includes `loanAmt` and `pendingAmt` and is used
  alone.
- **ETH and SOL staking** (`finance/staking-defi/eth/balance`, `/sol/balance`)
  are not collected. OKX reports those balances account-wide, and the BETH and
  OKSOL they produce are ordinary coins already held in `trading` or `funding`.
  Collecting them again would count them twice.
- **Products without a holdings endpoint** are not itemized: Simple Earn Fixed,
  Shark Fin, Snowball, Seagull, Price Lock, Jumpstart, BTC staking, Stable
  Rewards, and OKUSD. Value that OKX counts in its Earn valuation trips the
  [Earn check](#earn-check).
- **Not mapped:**
  - `trading`: `cashBal`, `disEq`, `eqUsd`, `isoEq`, `frozenBal`, `ordFrozen`,
    `uplLiab`, `crossLiab`, `isoLiab`, `spotBal`, `spotUpl`, `spotInUseAmt`,
    `accAvgPx`, `openAvgPx`, `maxLoan`, `rewardBal`, `fixedBal`, `autoLendAmt`,
    `twap`, `stgyEq`, and Futures mode's per-coin `mgnRatio`, `imr`, `mmr`,
    `notionalLever`;
  - Earn: `earnings`, `rate`, `loanAmt`, `pendingAmt`;
  - dual investment: expected payout and settlement amounts; the annualized rate
    is descriptive only.

## Dual Investment

The public API documents no separate structured-product cash balance endpoint. A
Dual Investment order takes its principal out of Funding when it is bought
(Funding bill type `160`). OKX pays the principal back to Funding at settlement
or refund (`161`–`163`). While the order is open, the principal is in no other
Account. `dual_investment` counts it in the coin invested
([Dual Investment decision](../../adr/0005-dual-investment.md)): `notionalSz` of
`notionalCcy`, added to that coin's Dual Balance and priced like any other
Balance. The yield (`yieldSz`) and a conversion into the other coin (`settleSz`
of `settleCcy`) count only once OKX pays them into Funding.

| `state`                                                                                                      | Counted                                                                     |
| ------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------- |
| `initial`, `pending_book`, `live`, `pending_settle`, `pending_redeem_booking`, `pending_redeem`, `redeeming` | yes                                                                         |
| `settled`, `redeemed`, `rejected`                                                                            | no. The payout or refund is back in Funding, or the order was never filled. |
| any other                                                                                                    | selected Earn and Dual Accounts fail with `normalization`, naming the state |

The states come from all of OKX's dual investment endpoints. The order history's
own `state` filter accepts only `initial`, `live`, `pending_settle`, `settled`,
`pending_redeem`, `redeemed`, and `rejected`. So the history cannot be filtered
to the invested states, and OKX documents no retention bound. The adapter
therefore reads the whole history, newest first:

1. It requests 100 orders, then the 100 before the oldest `ordId` seen
   (`endId`), 1.1 s apart to respect the one-request-per-second limit.
2. It stops at the first page with fewer than 100 orders.
3. Every page must list strictly decreasing order ids, below the previous page's
   oldest. Otherwise selected Earn and Dual Accounts fail with `normalization`,
   because a page out of order could end the walk before an invested order.
4. If ten full pages (1,000 orders) do not reach the end, selected Earn and Dual
   Accounts fail with `unsupported` and go Stale
   ([Report policy](../../adr/0002-account-reports.md)). It never reads a
   partial history and drops an older order that is still invested.

**Settlement window.** An order settles at its product's expiry, and OKX pays
out to Funding. OKX does not document how long the payout takes. A run inside
that window can go either way:

- The order has already left the counted states and the payout is not yet in
  Funding. The value dips.
- The payout is in Funding and the order still shows `pending_settle`. The value
  is counted twice.

The next run after both have updated is correct again. Telling the two apart
would need Funding's bill history, which Plutus does not read
([Capital Flow decision](../../adr/0004-capital-and-periods.md)). Both are
expected and are not reconciled across Accounts. The same window can fail one
run's [Earn check](#earn-check) if OKX's valuation still holds an order the
history already shows as settled. Selected Earn and Dual Accounts then stay
Stale for that run.

### Dual Position fields

Dual Account selection, principal valuation, and shared Earn completeness follow
[domain rules](../../../CONTEXT.md#dual-investment-and-earn-completeness).
Platform-specific fields map as follows:

| Dual Position field   | Source                                                                                                                                                                                                             |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Subscription identity | `ordId`; `productId` is separate                                                                                                                                                                                   |
| Principal             | `notionalSz` of `notionalCcy`                                                                                                                                                                                      |
| Pair and direction    | `baseCcy` / `quoteCcy`; investing base means Sell High, investing quote means Buy Low                                                                                                                              |
| Target, expiry        | `strike`, scheduled `settleTime` milliseconds. The older documentation example has `expTime` instead, which is used when `settleTime` is empty. `settledTime` is the actual payout time, not the scheduled expiry. |
| Annualized rate       | `annualizedYield`, tagged Annualized Yield                                                                                                                                                                         |

## Earn check

OKX keeps adding Earn products. One the adapter cannot read would otherwise
vanish from a successful `earn` report. So after itemizing, the adapter reads
`GET /api/v5/asset/asset-valuation?ccy=USDT`, OKX's own valuation split into
`trading`, `funding`, `earn`, and the deprecated `classic` bucket. It compares
`details.earn` with the combined ordinary Earn and Dual Balances valued at the
batch's prices ([Dual Investment decision](../../adr/0005-dual-investment.md)).
USDT counts as USD
([Valuation policy](../../adr/0003-valuation-and-catalog.md)), the quote the
Balances are priced in.

## Position

`account/positions`, every instrument type in one call.

| Plutus column               | Source                                                                                             |
| --------------------------- | -------------------------------------------------------------------------------------------------- |
| `platform_position_ref`     | `<instId>:<posSide>`, such as `DOGE-USDT-SWAP:net`                                                 |
| `side`                      | `posSide` `long` / `short` in long/short mode. In net mode (`posSide = net`), the sign of `pos`.   |
| `quantity`, `quantity_unit` | `abs(pos)` in contracts                                                                            |
| `entry_price`, `mark_price` | `avgPx`, `markPx`                                                                                  |
| `unrealized_pnl`            | `upl`, in the settlement coin                                                                      |
| `liquidation_price`         | `liqPx`. Null when empty or `0` (portfolio margin cross positions report none).                    |
| `leverage`                  | `lever`. Null when empty or `0` (portfolio margin reports no leverage and fills `0`).              |
| `margin`                    | Isolated `margin`; non-Portfolio Cross source `imr`, with verified currency; see provenance below. |

- **Zero rows.** Rows with `pos = 0` are dropped: OKX leaves them behind after
  an isolated-margin transfer.
- **`MARGIN` rows** are skipped in every Margin Mode. They are borrow-backed
  spot exposure whose value is already in the `trading` Balances' `eq`. In
  Futures mode they are also the only place OKX reports the borrowing (`liab`,
  `liabCcy`, `interest`), which Plutus therefore does not show.
- **`OPTION` and `EVENTS` rows** fail the `trading` report with `normalization`,
  naming the instrument, because Plutus has no Instrument kind for them. This
  includes the options a Spot mode account may buy.
- **Spot mode** holds no swaps or futures, so its positions list is empty unless
  it holds options.

| Instrument column             | Source (`public/instruments`)                                                                                         |
| ----------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| `symbol`                      | `instId`                                                                                                              |
| `kind`                        | `SWAP` → `linear_perpetual` / `inverse_perpetual`, `FUTURES` → `linear_delivery` / `inverse_delivery`, by `ctType`    |
| `base_symbol`, `quote_symbol` | the two halves of `uly` (`DOGE-USDT`, `BTC-USD`). Contract rows leave `baseCcy` and `quoteCcy` empty.                 |
| `settlement_symbol`           | `settleCcy`: `USDT` or `USDC` for linear swaps, `USD` for the `-USD_UM-` futures, the base coin for inverse contracts |
| `expiry_at`                   | `expTime`, futures only (`alias` is deprecated)                                                                       |
| `contract_size`               | `ctVal × ctMult`: base coins per contract for linear contracts, USD per contract for inverse ones                     |

Rows are handled as follows:

- **Skipped instruments.** Pre-open rows without an `instId` or a contract type
  are skipped, and so are futures without `expTime`.
- **Unknown contracts.** A position whose contract is missing from a fresh list
  fails the `trading` report with `normalization`, naming the contract. It is
  never dropped.
- **USD notional.** For a linear contract position it is contracts × contract
  size × mark price. For an inverse one it is contracts × contract size.

## Prices

Each non-USD Balance symbol is priced at the spot `last` of `<symbol>-USDT`,
else `<symbol>-USDC`, observed when the ticker response arrived.

- **USD equivalents.** USD, USDT, and USDC get no price row
  ([Valuation policy](../../adr/0003-valuation-and-catalog.md)). Other
  stablecoins such as USDG are priced through their pairs.
- **No pair.** A symbol with neither pair gets no price and is valued as
  `no_price`.
- **Failed ticker call.** The batch has no prices: valuation falls back to the
  latest price within 24 hours, and the Earn check is skipped.
- **Positions** carry their own `markPx`. Contracts get no separate price.

## Errors

| OKX answer                                                                                                                                                                                   | Class                                                  | Scope                                                                                     |
| -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------ | ----------------------------------------------------------------------------------------- |
| `50101`, `50103`–`50107`, `50111`, `50113`, `50114`, `50119` (key, signature, passphrase, missing header, wrong environment), `50035`, `50110`, `50121` (IP), `50100` (API frozen), HTTP 401 | `authentication`                                       | the whole batch; the Connection becomes `invalid`                                         |
| `50030`, `50120`, `59500` (the key may not use the endpoint)                                                                                                                                 | `authorization`                                        | the whole batch; the Connection becomes `invalid`                                         |
| `50011`, `50013`, `50040`, HTTP 429                                                                                                                                                          | `rate_limited`                                         | the Account; the batch when every Account failed                                          |
| `50001`, `50004`, `50026`, `50046`, `51767`, HTTP 5xx, timeouts, network errors, `50102` or `50112` after one resync                                                                         | `temporary`                                            | the Account; the batch when every Account failed                                          |
| any other non-zero `code` or HTTP status                                                                                                                                                     | `temporary`, with OKX's code and message in the detail | the Account                                                                               |
| an `acctLv` other than `1`–`4`, or an Account Structure other than `okx_unified`                                                                                                             | `unsupported`                                          | the whole batch. Not retried; the Connection stays enabled while its Accounts turn stale. |
| a dual investment history of 1,000 orders or more; an Earn valuation above the itemized Earn beyond tolerance                                                                                | `unsupported`                                          | selected `earn` and `dual_investment`, which turn stale                                   |
| an Account Type other than `trading`, `funding`, `earn`, or `dual_investment`                                                                                                                | `unsupported`                                          | that Account                                                                              |
| an empty `acctLv`, or `account/config` without rows                                                                                                                                          | `normalization`                                        | the whole batch                                                                           |
| unparseable or non-decimal fields, an empty `totalEq`, `eq`, `bal`, or `amt`, unknown contracts, `OPTION` or `EVENTS` positions, `code` `0` without `data`                                   | `normalization`                                        | the Account                                                                               |
| an undocumented dual investment state, an order history that is not newest first, an invested order without `notionalCcy` or `notionalSz`, a valuation without `details.earn`                | `normalization`                                        | selected `earn` and `dual_investment` Accounts                                            |

OKX answers many failures with HTTP 200 and a non-zero `code`, so both the
status and the code decide. `code` may be a string or a number. A compound code
such as `51008_1000` is classified by its main code. Error details name the
endpoint and OKX's code and message, cut to 200 characters, never a body or a
key.

### Position presentation provenance

`posSide=net` retains one-way mode; long/short retains hedged mode. `mgnMode`
retains the cross/isolated setting. Non-Portfolio Cross `imr` is a position
initial requirement; `ccy` is retained rather than universally labeling it USD.
Under Portfolio Margin, risk-unit `imr` can repeat across member positions and
is not retained as a position allocation. Isolated `margin` is allocated funds;
`ccy` retains its source currency for validation. Effective Contract Size
remains `ctVal × ctMult`, with Base units for linear and Quote units for
inverse.

The current
[REST guide](https://app.okx.com/docs-v5/en/#rest-api-trading-account-get-positions)
describes position `imr` as USD, but reviewed Futures and Multi-currency inverse
responses match native settlement amounts. The
[official margin rules](https://www.okx.com/en-sg/help/futures-margin-calculation-rules)
also define crypto Cross initial requirements in settlement coin. For example,
two USD 10 ETH contracts at Mark 2,687.06 and leverage 3 report
`imr=0.0024810263509809`, matching ETH rather than USD. The old USD mapping
would divide this amount by Mark a second time.

Non-PM inverse reads validate Base=Settle, USD-equivalent Quote, positive size,
Contract Size, Mark, and initial leverage. They compare the source amount to
both native and USD initial requirements within relative `1e-8` rounding
tolerance. A native-only match uses Settle; a USD-only match uses USD and
explicit conversion. Ambiguous or unmatched amounts have unknown currency.
Linear USD-equivalent currencies follow the existing 1:1 policy; incompatible or
missing currency remains unknown. This validation applies to earlier typed-USD
Reports with known non-PM regime and provenance without changing persisted
amounts or using the Identity's latest mode. No generic calculated fallback
replaces OKX reported hedge or risk-unit treatment.
