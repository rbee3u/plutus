# OKX adapter

Package `internal/adapters/okx`. OKX has one account, the Unified account, so it has one Account Structure, `okx_unified`, with the Account Types `trading`, `funding`, `earn`, and `dual_investment`. The Margin Mode is OKX's account mode (the `acctLv` level). Plutus supports all four levels: Spot mode, Futures mode, Multi-currency margin, and Portfolio margin. The adapter observes the level at validation and at the start of every Collection Run. It is never chosen in Plutus ([ADR 0028](../adr/0028-separate-account-structure-from-margin-mode.md)).

All four modes have independent synthetic response coverage based on official contracts; 20 Spot, Futures, MCM, PM and shared responses recorded on 2026-10-01 are now owner-reviewed. Spot evidence includes enabled borrowing; its borrowing-disabled real shape is deferred because the owner found no separate disable control. PM and MCM use separate constructed shapes, including their empty and mode-specific fields. New recordings are reviewed independently of those tests ([ADR 0037](../adr/0037-separate-recorded-evidence-from-synthetic-tests.md)).

Every call is a read-only GET for account, balance, position, Earn holdings, or market data.

A sub-account is its own Identity with its own read-only key ([ADR 0015](../adr/0015-select-account-types-per-identity.md)). OKX answers `account/config` with the sub-account's own UID and level, so it validates like a main account. Each sub-account can use a different Margin Mode. Nothing is collected through a main-account key.

The adapter calls `https://openapi.okx.com`. OKX requires accounts registered on its US or Australian app to use `us.okx.com`, and European ones to use `eea.okx.com`. Those hosts are not configurable yet.

## Margin Modes

`account/config` reports the level as `acctLv`. Every validation and every Collection Run read it before any Account is collected. The User can switch the level in the OKX app between two runs. The next run then reports the new Margin Mode and maps what that level fills, with no failure.

| `acctLv` | Margin Mode | OKX name | What the trading account holds | Coverage |
| --- | --- | --- | --- | --- |
| `1` | `okx_spot` | Spot mode | Spot, spot borrowing once enabled, bought options. No swaps or futures. | synthetic; reviewed borrowing-enabled config/balance evidence (2026-10-01) |
| `2` | `okx_futures` | Futures mode | Spot, margin, swaps, futures, options. Margin is assessed per coin, and spot margin borrowings are `MARGIN` positions. | synthetic; reviewed isolated Hedge, cross and MARGIN evidence (2026-10-01) |
| `3` | `okx_mcm` | Multi-currency margin | Every product, cross-collateralized in USD | synthetic mode-specific response; reviewed cross/net evidence (2026-10-01) |
| `4` | `okx_pm` | Portfolio margin | As Multi-currency margin, margined by portfolio risk | synthetic mode-specific response; reviewed cross/net evidence (2026-10-01) |
| empty | none | | `normalization` | tested |
| any other | none | | `unsupported`, naming the level. The run fails before any Account is read. | tested with `5` |

Validation returns the account's `uid` as the External Ref, plus the Margin Mode. Any Account Structure other than `okx_unified` is refused with `unsupported` before any call.

## Endpoints

| Use | Endpoint | Signed | OKX limit |
| --- | --- | --- | --- |
| Clock sync | `GET /api/v5/public/time` | no | 10 / 2 s per IP |
| Validate, and the Margin Mode read that starts every Collection Run | `GET /api/v5/account/config` | yes | 5 / 2 s |
| `trading` | `GET /api/v5/account/balance`, `GET /api/v5/account/positions` | yes | 10 / 2 s each |
| Instruments | `GET /api/v5/public/instruments?instType=SWAP` and `?instType=FUTURES`, fetched only for open positions and cached in process for one hour | no | 20 / 2 s per IP and type |
| `funding` | `GET /api/v5/asset/balances` | yes | 6 / s |
| `earn` | `GET /api/v5/finance/savings/balance` (6 / s), `GET /api/v5/finance/staking-defi/orders-active` (3 / s) | yes | as listed, per User ID |
| `dual_investment` | `GET /api/v5/finance/sfp/dcd/order-history?limit=100` and its next pages `?endId=<ordId>&limit=100` | yes | 1 / s per User ID |
| Combined Earn/Dual completeness | `GET /api/v5/asset/asset-valuation?ccy=USDT` | yes | 1 / s per User ID |
| Prices | `GET /api/v5/market/tickers?instType=SPOT` (every spot pair in one call, fetched before the Account calls so its time precedes every report) | no | 20 / 2 s per IP |

Signed requests carry four headers:

- `OK-ACCESS-KEY`;
- `OK-ACCESS-PASSPHRASE`;
- `OK-ACCESS-TIMESTAMP`, in ISO 8601 UTC with milliseconds;
- `OK-ACCESS-SIGN`, the Base64 HMAC-SHA256 of timestamp + method + path with its query.

OKX rejects a timestamp more than 30 seconds from its clock. So the timestamp is corrected by the offset to OKX server time. The offset is measured before the first signed call, and again after a `50102` or `50112` answer, which is then retried once.

With all four Accounts selected, a Collection Run makes eight signed calls, plus one for each further page of dual investment history. It also makes one to four public calls:

- the tickers, always;
- server time, before the first signed call and after a clock rejection;
- the two instrument lists, when an open position needs a list that is not cached.

## Account Report

`trading` reads `account/balance`. OKX returns `""` for a field that does not apply to the account's level. One mapping serves all four Margin Modes, and an empty field stays null, never zero. `totalEq` is required: without it the report fails with `normalization`.

In the tables below, "yes" marks the fields OKX documents for that mode. A field OKX leaves empty stays null in any mode.

| Plutus column | Source | Spot mode | Futures mode | Multi-currency, Portfolio margin |
| --- | --- | --- | --- | --- |
| `equity` | `totalEq` (USD) | yes | yes | yes |
| `adjusted_equity` | `adjEq` (USD, after collateral discounts) | yes | null | yes |
| `available_equity` | `availEq` | null | null | yes |
| `initial_margin_ratio` | derived: `imr ÷ adjEq`, 8 decimals. Null when `adjEq ≤ 0` or `imr` is empty. | yes | null | yes |
| `maintenance_margin_ratio` | derived: `mmr ÷ adjEq`, same rules | yes | null | yes |
| `unified_margin_ratio` | `mgnRatio` as reported (`adjEq ÷ mmr`, a multiple where higher is safer) | yes | null | yes |
| `observed_at` | local time when the response arrived | yes | yes | yes |

In Futures mode the account level carries only `totalEq` and `isoEq`. OKX assesses margin per coin (`details[].mgnRatio`, `imr`, `mmr`, `notionalLever`), and only while a cross position is open. Plutus has no per-coin margin column. This is an accepted display gap: the equity is still complete.

`funding`, `earn`, and `dual_investment` report no equity. Their value is `calculated` from Balances, and every other column is null.

Not stored: `upl`, `isoEq`, `ordFroz`, `borrowFroz`, `notionalUsd` and its per-business variants, `delta`, `deltaLever`, `deltaNeutralStatus`, `uTime`, and portfolio margin's `account/risk-state`.

## Balance

`trading` reads `account/balance` `details[]`:

| Plutus column | Source | Spot mode | Futures mode | Multi-currency, Portfolio margin |
| --- | --- | --- | --- | --- |
| `symbol` | `ccy` | yes | yes | yes |
| `equity` (coin units) | `eq` as reported, may be negative | yes | yes | yes |
| `available_to_trade` | `availEq` | null | yes | yes |
| `available_to_withdraw` | `availBal` | yes | yes | yes |
| `liability` | `liab`, documented as positive and stored as its absolute value | yes | null | yes |
| `interest` | `interest` | yes | null | yes |
| `unrealized_pnl` | `upl` | null | yes | yes |
| `name` | not mapped | | | |

`funding` reads `asset/balances`: `symbol` from `ccy`, `equity` from `bal`, and `available_to_withdraw` from `availBal`.

`earn` has one row per coin, summing two sources:

- Simple Earn `amt`;
- on-chain earn `investData[].amt`.

Dual principal is summed separately in `dual_investment` (see [Dual investment](#dual-investment)).

The other `funding` and `earn` columns are null. OKX gives no full coin names, so `name` is never mapped.

Mapping rules:

- **Equity.** `eq` is taken as reported, never rebuilt. OKX does not document its composition, and it must not be replaced by `cashBal + upl`. The per-coin field remains distinct from the account-wide `totalEq`. It already includes isolated margin, unrealized PnL, and option value.
- **Spot mode borrowings.** `liab` already includes the accrued interest, according to OKX's help article on Spot mode borrowing. `interest` is shown beside it and never added again.
- **Futures mode borrowings.** They exist only on `MARGIN` position rows, which are skipped (see [Position](#position)), so no Balance shows them.
- **Required amounts.** `eq`, `bal`, each `amt`, and each invested order's `notionalSz` are required. An empty one fails its Account with `normalization`, naming the coin. Read as zero, it would drop the coin under the Complete Set rule. The other fields may be empty and then stay null.
- **Zero rows.** Rows whose numeric fields are all zero are dropped.
- **Simple Earn.** `amt` already includes `loanAmt` and `pendingAmt` and is used alone.
- **ETH and SOL staking** (`finance/staking-defi/eth/balance`, `/sol/balance`) are not collected. OKX reports those balances account-wide, and the BETH and OKSOL they produce are ordinary coins already held in `trading` or `funding`. Collecting them again would count them twice.
- **Products without a holdings endpoint** are not itemized: Simple Earn Fixed, Shark Fin, Snowball, Seagull, Price Lock, Jumpstart, BTC staking, Stable Rewards, and OKUSD. Value that OKX counts in its Earn valuation trips the [Earn check](#earn-check).
- **Not mapped:**
  - `trading`: `cashBal`, `disEq`, `eqUsd`, `isoEq`, `frozenBal`, `ordFrozen`, `uplLiab`, `crossLiab`, `isoLiab`, `spotBal`, `spotUpl`, `spotInUseAmt`, `accAvgPx`, `openAvgPx`, `maxLoan`, `rewardBal`, `fixedBal`, `autoLendAmt`, `twap`, `stgyEq`, and Futures mode's per-coin `mgnRatio`, `imr`, `mmr`, `notionalLever`;
  - Earn: `earnings`, `rate`, `loanAmt`, `pendingAmt`;
  - dual investment: expected payout and settlement amounts; the annualized rate is descriptive only.

## Dual investment

A dual investment order takes its principal out of Funding when it is bought (Funding bill type `160`). OKX pays the principal back to Funding at settlement or refund (`161`–`163`). While the order is open, the principal is in no other Account. `dual_investment` counts it in the coin invested ([ADR 0033](../adr/0033-separate-dual-investment-accounts-and-positions.md)): `notionalSz` of `notionalCcy`, added to that coin's Dual Balance and priced like any other Balance. The yield (`yieldSz`) and a conversion into the other coin (`settleSz` of `settleCcy`) count only once OKX pays them into Funding.

| `state` | Counted |
| --- | --- |
| `initial`, `pending_book`, `live`, `pending_settle`, `pending_redeem_booking`, `pending_redeem`, `redeeming` | yes |
| `settled`, `redeemed`, `rejected` | no. The payout or refund is back in Funding, or the order was never filled. |
| any other | selected Earn and Dual Accounts fail with `normalization`, naming the state |

The states come from all of OKX's dual investment endpoints. The order history's own `state` filter accepts only `initial`, `live`, `pending_settle`, `settled`, `pending_redeem`, `redeemed`, and `rejected`. So the history cannot be filtered to the invested states, and OKX documents no retention bound. The adapter therefore reads the whole history, newest first:

1. It requests 100 orders, then the 100 before the oldest `ordId` seen (`endId`), 1.1 s apart to respect the one-request-per-second limit.
2. It stops at the first page with fewer than 100 orders.
3. Every page must list strictly decreasing order ids, below the previous page's oldest. Otherwise selected Earn and Dual Accounts fail with `normalization`, because a page out of order could end the walk before an invested order.
4. If ten full pages (1,000 orders) do not reach the end, selected Earn and Dual Accounts fail with `unsupported` and go Stale ([ADR 0019](../adr/0019-carry-a-failed-account-as-stale-and-treat-success-as-complete.md)). It never reads a partial history and drops an older order that is still invested.

**Settlement window.** An order settles at its product's expiry, and OKX pays out to Funding. OKX does not document how long the payout takes. A run inside that window can go either way:

- The order has already left the counted states and the payout is not yet in Funding. The value dips.
- The payout is in Funding and the order still shows `pending_settle`. The value is counted twice.

The next run after both have updated is correct again. Telling the two apart would need Funding's bill history, which Plutus does not read ([ADR 0020](../adr/0020-record-capital-flows-manually-with-read-only-credentials.md)). Both are expected and are not reconciled across Accounts. The same window can fail one run's [Earn check](#earn-check) if OKX's valuation still holds an order the history already shows as settled. Selected Earn and Dual Accounts then stay Stale for that run.

## Earn check

OKX keeps adding Earn products. One the adapter cannot read would otherwise vanish from a successful `earn` report. So after itemizing, the adapter reads `GET /api/v5/asset/asset-valuation?ccy=USDT`, OKX's own valuation split into `trading`, `funding`, `earn`, and the deprecated `classic` bucket. It compares `details.earn` with the combined ordinary Earn and Dual Balances valued at the batch's prices ([ADR 0033](../adr/0033-separate-dual-investment-accounts-and-positions.md)). USDT counts as USD ([ADR 0017](../adr/0017-value-in-usd-and-treat-major-stablecoins-as-usd.md)), the quote the Balances are priced in.

## Dual Investment Account and positions

`dual_investment` is a separately selected product Account, not an independently transferable cash wallet. Its Balances aggregate outstanding principal by invested currency; available and withdrawable values remain null. Each subscription is stored separately and never adds a second value to its Balance. The Positions page separates Futures and Dual Investment.

Selecting either `earn` or `dual_investment` reads both product groups once for the combined Earn completeness check. Only selected reports leave the Adapter. Required auxiliary failures and unexplained aggregate shortfalls fail both selected dependent Accounts; unrelated Accounts retain their normal failure isolation. Neither product group is read when neither is selected. Missing optional terms are null with display issues; missing identity/principal or contradictory fields fail normalization. See [ADR 0033](../adr/0033-separate-dual-investment-accounts-and-positions.md).

| Dual Position field | Source |
| --- | --- |
| Subscription identity | `ordId`; `productId` is separate |
| Principal | `notionalSz` of `notionalCcy` |
| Pair and direction | `baseCcy` / `quoteCcy`; investing base means Sell High, investing quote means Buy Low |
| Target, expiry | `strike`, scheduled `settleTime` milliseconds. The older documentation example has `expTime` instead, which is used when `settleTime` is empty. `settledTime` is the actual payout time, not the scheduled expiry. |
| Annualized rate | `annualizedYield`, tagged Annualized Yield |

The public API documents no separate structured-product cash balance endpoint. Funding bill types 160–163 describe subscription and payouts; bills are not read by this Adapter. Capture acceptance combines ordinary Earn and Dual against `details.earn`, and logs Dual principal separately against the residual after ordinary Earn.

## Position

`account/positions`, every instrument type in one call.

| Plutus column | Source |
| --- | --- |
| `platform_position_ref` | `<instId>:<posSide>`, such as `DOGE-USDT-SWAP:net` |
| `side` | `posSide` `long` / `short` in long/short mode. In net mode (`posSide = net`), the sign of `pos`. |
| `quantity`, `quantity_unit` | `|pos|` in `contracts` |
| `entry_price`, `mark_price` | `avgPx`, `markPx` |
| `unrealized_pnl` | `upl`, in the settlement coin |
| `liquidation_price` | `liqPx`. Null when empty or `0` (portfolio margin cross positions report none). |
| `leverage` | `lever`. Null when empty or `0` (portfolio margin reports no leverage and fills `0`). |
| `margin` | `margin`, isolated positions only. Null for cross. |

- **Zero rows.** Rows with `pos = 0` are dropped: OKX leaves them behind after an isolated-margin transfer.
- **`MARGIN` rows** are skipped in every Margin Mode. They are borrow-backed spot exposure whose value is already in the `trading` Balances' `eq`. In Futures mode they are also the only place OKX reports the borrowing (`liab`, `liabCcy`, `interest`), which Plutus therefore does not show.
- **`OPTION` and `EVENTS` rows** fail the `trading` report with `normalization`, naming the instrument, because Plutus has no Instrument kind for them. This includes the options a Spot mode account may buy.
- **Spot mode** holds no swaps or futures, so its positions list is empty unless it holds options.

| Instrument column | Source (`public/instruments`) |
| --- | --- |
| `symbol` | `instId` |
| `kind` | `SWAP` → `linear_perpetual` / `inverse_perpetual`, `FUTURES` → `linear_delivery` / `inverse_delivery`, by `ctType` |
| `base_symbol`, `quote_symbol` | the two halves of `uly` (`DOGE-USDT`, `BTC-USD`). Contract rows leave `baseCcy` and `quoteCcy` empty. |
| `settlement_symbol` | `settleCcy`: `USDT` or `USDC` for linear swaps, `USD` for the `-USD_UM-` futures, the base coin for inverse contracts |
| `expiry_at` | `expTime`, futures only (`alias` is deprecated) |
| `contract_size` | `ctVal × ctMult`: base coins per contract for linear contracts, USD per contract for inverse ones |

Rows are handled as follows:

- **Skipped instruments.** Pre-open rows without an `instId` or a contract type are skipped, and so are futures without `expTime`.
- **Unknown contracts.** A position whose contract is missing from a fresh list fails the `trading` report with `normalization`, naming the contract. It is never dropped.
- **USD notional.** For a linear contract position it is contracts × contract size × mark price. For an inverse one it is contracts × contract size.

## Prices

Each non-USD Balance symbol is priced at the spot `last` of `<symbol>-USDT`, else `<symbol>-USDC`, observed when the ticker response arrived.

- **USD equivalents.** USD, USDT, and USDC get no price row ([ADR 0017](../adr/0017-value-in-usd-and-treat-major-stablecoins-as-usd.md)). Other stablecoins such as USDG are priced through their pairs.
- **No pair.** A symbol with neither pair gets no price and is valued as `no_price`.
- **Failed ticker call.** The batch has no prices: valuation falls back to the latest price within 24 hours, and the Earn check is skipped.
- **Positions** carry their own `markPx`. Contracts get no separate price.

## Errors

| OKX answer | Class | Scope |
| --- | --- | --- |
| `50101`, `50103`–`50107`, `50111`, `50113`, `50114`, `50119` (key, signature, passphrase, missing header, wrong environment), `50035`, `50110`, `50121` (IP), `50100` (API frozen), HTTP 401 | `authentication` | the whole batch; the Connection becomes `invalid` |
| `50030`, `50120`, `59500` (the key may not use the endpoint) | `authorization` | the whole batch; the Connection becomes `invalid` |
| `50011`, `50013`, `50040`, HTTP 429 | `rate_limited` | the Account; the batch when every Account failed |
| `50001`, `50004`, `50026`, `50046`, `51767`, HTTP 5xx, timeouts, network errors, `50102` or `50112` after one resync | `temporary` | the Account; the batch when every Account failed |
| any other non-zero `code` or HTTP status | `temporary`, with OKX's code and message in the detail | the Account |
| an `acctLv` other than `1`–`4`, or an Account Structure other than `okx_unified` | `unsupported` | the whole batch. Not retried; the Connection stays enabled while its Accounts turn stale. |
| a dual investment history of 1,000 orders or more; an Earn valuation above the itemized Earn beyond tolerance | `unsupported` | selected `earn` and `dual_investment`, which turn stale |
| an Account Type other than `trading`, `funding`, `earn`, or `dual_investment` | `unsupported` | that Account |
| an empty `acctLv`, or `account/config` without rows | `normalization` | the whole batch |
| unparseable or non-decimal fields, an empty `totalEq`, `eq`, `bal`, or `amt`, unknown contracts, `OPTION` or `EVENTS` positions, `code` `0` without `data` | `normalization` | the Account |
| an undocumented dual investment state, an order history that is not newest first, an invested order without `notionalCcy` or `notionalSz`, a valuation without `details.earn` | `normalization` | selected `earn` and `dual_investment` Accounts |

OKX answers many failures with HTTP 200 and a non-zero `code`, so both the status and the code decide. `code` may be a string or a number. A compound code such as `51008_1000` is classified by its main code. Error details name the endpoint and OKX's code and message, cut to 200 characters, never a body or a key.

## Fixtures and evidence

The [two-layer library](./fixtures.md) separates reviewed real `evidence/` from independently constructed `synthetic` inputs in `testdata/`. [The Okx inventory](./okx-fixtures.md) links official contracts, test cases and missing real evidence. Twenty Spot, Futures, MCM, PM and shared responses recorded on 2026-10-01 are frozen after owner review, covering nonzero linear/inverse perpetual/delivery positions, borrowing, Funding/BETH, Savings, both Dual directions and On-chain Earn. Futures evidence includes same-contract isolated Hedge sides, cross inverse positions and an isolated spot-margin borrowing row. Spot evidence includes nonzero HYPE borrowing with distinct cash and equity. Nonzero accrued interest remains a real gap. Borrowing-disabled Spot evidence is explicitly deferred; this account uses its observed enabled state as the baseline, without imposing that default on other accounts. Tests verify mapping, signatures, paging, failure classification and completeness without exchange access. Recording selects one explicit interface with `--only`, optional `--variant`, business `--query`/`--form`, and local `--keep-symbol` hints; prepared rounds may run an agreed sequential list, stopping at the first failure. Review critical shapes/document differences and owner-review before promoting to evidence; no full replay is required and synthetic tests are not regenerated.

## Comparison checklist (test environment)

Run on the test environment against the OKX app on the same account, within a few minutes of a collection.

**Every Margin Mode:**

1. **Identity**: validation stores the UID shown under the app's profile, and the Identity shows the Margin Mode that matches the app's account mode.
2. **Trading equity**: `Equity` equals the trading account's total equity in USD.
3. **Trading balances**: each coin's equity matches the app's per-coin equity in the trading account, including unrealized PnL.
4. **Positions**: every open swap and futures position appears with the same direction, size in contracts, entry price, and mark price.
5. **Funding**: coin amounts match the Funding account.
6. **Earn and Dual Investment**: ordinary Earn contains Simple Earn and on-chain earn. Dual Investment contains each outstanding subscription at its invested principal, with a separate Position showing target, expiry, rate, and state. Their combined value matches the platform Earn aggregate. BETH and OKSOL appear under `trading` or `funding`, not `earn`.
7. **Account values**: the calculated USD values of `funding` and `earn` are within about 0.1% of the app's USD valuation of each account.

**Per Margin Mode:**

8. **Portfolio margin and Multi-currency margin**: `AdjEq` equals the adjusted equity and `UniMMR` the margin ratio the app shows, possibly as a percentage. In Portfolio margin, cross positions show no leverage and no liquidation price.
9. **Spot mode**: with spot borrowing on, `AdjEq` and `UniMMR` match the app. Each borrowed coin shows its borrowing, interest included, and its interest alone. Without borrowing, the margin figures may stay empty.
10. **Futures mode**: account-level margin figures stay empty. Spot margin borrowings appear neither as Balances' liabilities nor as positions, but the coin equities still add up to the total equity.

**Changes and failures:**

11. **Mode switch**: after switching the account mode in the app, the next cycle reports the new Margin Mode with no failed Account, and the audit log records the change.
12. **Dual investment across a settlement**: if an order settles during acceptance, note whether its principal dips or counts twice between the settlement and the Funding payout. Confirm the next cycle is correct.
13. **Earn check**: holding an Earn product Plutus does not itemize, such as Shark Fin, turns `earn` stale with an `unsupported` detail naming OKX's and Plutus's amounts, provided OKX counts the product in its Earn valuation.
14. **Failures**: removing the server IP from the key's whitelist sets the Connection `invalid` with a `50110` detail at the next cycle. Restoring it and validating again resumes collection.
15. **Other modes**: once sub-accounts in Spot mode, Futures mode, and Multi-currency margin have their own read-only keys, enter each as an Identity and repeat 1–10.
