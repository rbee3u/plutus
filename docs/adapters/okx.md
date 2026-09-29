# OKX adapter

Package `internal/adapters/okx`. OKX has one account, the Unified account, so it has one Account Structure, `okx_unified`, with the Account Types `trading`, `funding`, `earn`, and `dual_investment`. The Margin Mode is OKX's account mode (the `acctLv` level). Plutus supports all four levels: Spot mode, Futures mode, Multi-currency margin, and Portfolio margin. The adapter observes the level at validation and at the start of every Collection Run. It is never chosen in Plutus ([ADR 0028](../adr/0028-separate-account-structure-from-margin-mode.md)).

Coverage differs by mode:

- **Portfolio margin:** exercised against a real account.
- **Multi-currency margin:** uses the same fields as Portfolio margin. It is tested by replaying the Portfolio margin recording as level 3.
- **Spot mode and Futures mode:** implemented and tested from OKX's documentation only ([ADR 0027](../adr/0027-allow-documented-adapter-scenarios-without-live-coverage.md)).

Every call is a read-only GET for account, balance, position, Earn holdings, or market data.

A sub-account is its own Identity with its own read-only key ([ADR 0015](../adr/0015-select-account-types-per-identity.md)). OKX answers `account/config` with the sub-account's own UID and level, so it validates like a main account. Each sub-account can use a different Margin Mode. Nothing is collected through a main-account key.

The adapter calls `https://openapi.okx.com`. OKX requires accounts registered on its US or Australian app to use `us.okx.com`, and European ones to use `eea.okx.com`. Those hosts are not configurable yet.

## Margin Modes

`account/config` reports the level as `acctLv`. Every validation and every Collection Run read it before any Account is collected. The User can switch the level in the OKX app between two runs. The next run then reports the new Margin Mode and maps what that level fills, with no failure.

| `acctLv` | Margin Mode | OKX name | What the trading account holds | Coverage |
| --- | --- | --- | --- | --- |
| `1` | `okx_spot` | Spot mode | Spot, spot borrowing once enabled, bought options. No swaps or futures. | documented |
| `2` | `okx_futures` | Futures mode | Spot, margin, swaps, futures, options. Margin is assessed per coin, and spot margin borrowings are `MARGIN` positions. | documented |
| `3` | `okx_mcm` | Multi-currency margin | Every product, cross-collateralized in USD | recorded Portfolio margin set replayed as level 3 |
| `4` | `okx_pm` | Portfolio margin | As Multi-currency margin, margined by portfolio risk | recorded and live 2026-09-25 |
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

Every signed endpoint needs only the key's Read permission. On 2026-09-25 the test environment's read-only key reached all of them, including the dual investment order history and the asset valuation.

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

- **Equity.** `eq` is taken as reported, never rebuilt. OKX does not document its composition, and on the recorded account it differs from `cashBal + upl`. But the per-coin USD equities `eqUsd` add up to `totalEq` exactly, so `eq` is the figure consistent with the account equity. It already includes isolated margin, unrealized PnL, and option value.
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

OKX documents `code` as a string, but this endpoint answers with a number (`"code": 0`, observed 2026-09-25 and 2026-09-29). The recorded orders use uppercase states (`LIVE`, `SETTLED`); state matching ignores case. Both forms of `code` are accepted everywhere.

**Settlement window.** An order settles at its product's expiry, and OKX pays out to Funding. OKX does not document how long the payout takes. A run inside that window can go either way:

- The order has already left the counted states and the payout is not yet in Funding. The value dips.
- The payout is in Funding and the order still shows `pending_settle`. The value is counted twice.

The next run after both have updated is correct again. Telling the two apart would need Funding's bill history, which Plutus does not read ([ADR 0020](../adr/0020-record-capital-flows-manually-with-read-only-credentials.md)). Both are expected and are not reconciled across Accounts. The same window can fail one run's [Earn check](#earn-check) if OKX's valuation still holds an order the history already shows as settled. Selected Earn and Dual Accounts then stay Stale for that run.

## Earn check

OKX keeps adding Earn products. One the adapter cannot read would otherwise vanish from a successful `earn` report. So after itemizing, the adapter reads `GET /api/v5/asset/asset-valuation?ccy=USDT`, OKX's own valuation split into `trading`, `funding`, `earn`, and the deprecated `classic` bucket. It compares `details.earn` with the combined ordinary Earn and Dual Balances valued at the batch's prices ([ADR 0033](../adr/0033-separate-dual-investment-accounts-and-positions.md)). USDT counts as USD ([ADR 0017](../adr/0017-value-in-usd-and-treat-major-stablecoins-as-usd.md)), the quote the Balances are priced in.

- **Too high.** If OKX's figure exceeds the itemized value by more than max(1% of OKX's figure, 1 USD), selected Earn and Dual Accounts fail with `unsupported`, naming both amounts, and go Stale. The tolerance absorbs the difference between OKX's prices and the batch's.
- **Missing.** A missing or empty `details.earn` fails selected Earn and Dual Accounts with `normalization`.
- **Skipped.** The check is skipped when an Earn or Dual Balance has no price. That happens for a coin without a USDT or USDC pair, and for every coin but USD, USDT, and USDC when the ticker call failed. Valuation already marks such a Balance `no_price`.
- **One-sided.** An itemized value above OKX's never fails. So a product OKX leaves out of its `earn` bucket cannot trip the check. The 2026-09-25 recording had no Dual order; the 2026-09-29 recording and live comparison cover funded Dual principal. The live test logs that principal and the aggregate residual separately.

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

The public API documents no separate structured-product cash balance endpoint. Funding bill types 160–163 describe subscription and payouts; bills are not read by this Adapter. The live comparison combines ordinary Earn and Dual against `details.earn`, and logs Dual principal separately against the residual after ordinary Earn.

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
- **`OPTION` and `EVENTS` rows** fail the `trading` report with `normalization`, naming the instrument, because Plutus has no Instrument kind for them. This includes the options a Spot mode account may buy. The recorded account cannot trade options (`opAuth = 0`).
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

## Completeness check (live test)

The runtime [Earn check](#earn-check) covers the combined Earn and Dual holdings. The live smoke test compares every bucket of `asset/asset-valuation?ccy=USDT`:

- the collected `trading` equity;
- the priced Funding Balances and combined Earn/Dual Balances.

It fails when one differs by more than 1% (at least 1 USD), or when another bucket, such as `classic`, holds value. That is how a trading or funding bucket OKX adds later shows up before the Complete Set rule would drop it silently.

The recorded baseline run on 2026-09-25 matched every bucket within 0.03%:

| Bucket | Difference |
| --- | --- |
| `trading` | −0.026% |
| `funding` | +0.001% |
| `earn` | −0.005% |
| `classic` | empty |

## Fixtures

Replay tests serve two kinds of fixture through `httptest`. Both are checked for the key, passphrase, signature, and clock-corrected timestamp of every signed request.

**Recorded** (`testdata/*.json`, [ADR 0023](../adr/0023-test-platform-adapters-in-three-layers.md)). `plutus adapter record --platform okx` reads `PLUTUS_GOTEST_OKX_APIKEY` / `_SECRET` / `_PASSPHRASE` from the environment or `config/gotest.env`. It calls each endpoint above once, and every page of dual investment history the adapter would read. It writes `tmp/fixtures/okx/<endpoint>.json`, with query values in the file name: `api_v5_public_instruments.SWAP.json`, or `api_v5_finance_sfp_dcd_order_history.100.json` for the first dual investment page and `.<endId>.100.json` for the next ones. Each header carries:

For a focused refresh, use `--only api_v5_finance_sfp_dcd_order_history.100` (repeat `--only` for more files). Only the requested fixture is written; the recorder still reads config and clock for provenance and signing. A later Dual Investment page also reads its preceding pages to reach the requested cursor. Market-data and refusal fixtures still use a full recording.

- `accountStructure` `okx_unified`;
- the `marginMode` read from the account's level;
- `scenario` `recorded`, or the error name for the error answers below;
- the recording time.

Amounts are kept as recorded, so identities such as Σ `eqUsd` = `totalEq` still hold in the tests. The recorder:

- **Redacts `account/config`.** It replaces `uid` and `mainUid` with `100000000000000001` (a sub-account's `mainUid` with `100000000000000002`), empties `ip`, and sets `label` to `plutus-fixture`. It replaces any IPv4 address with `203.0.113.1`, and writes nothing if the key, secret, passphrase, or a real UID survives.
- **Trims `market/tickers`** to the `-USDT` and `-USDC` pairs of recorded coins, dual investment coins included, plus `BTC-USDT`, `ETH-USDT`, and the first USDC-only coin.
- **Trims the instrument lists.** `SWAP` keeps held contracts plus `BTC-USDT-SWAP` (linear) and `BTC-USD-SWAP` (inverse). `FUTURES` keeps held contracts plus the first linear and the first inverse BTC contract.
- **Records four error answers** on `account/config`, all HTTP 401. Rate-limit and server errors are not provoked; tests simulate them.
  - an unknown key: `invalid_key`, `50119`;
  - a wrong signature: `bad_signature`, `50113`;
  - a wrong passphrase: `wrong_passphrase`, `50105`;
  - a timestamp five minutes old: `stale_timestamp`, `50102`.

Position, trade, order, and quote ids (`posId`, `tradeId`, `ordId`, `quoteId`) are kept as recorded. After review, the files move to `internal/adapters/okx/testdata/`.

The baseline set comes from a Portfolio margin main account recorded on 2026-09-25: 12 successful answers and four error answers. Only the dual investment order-history fixture was refreshed on 2026-09-29. The fixtures cover:

- `trading`: USDT and HYPE, and one cross `DOGE-USDT-SWAP` position in net mode;
- `funding`: USDT, USDC, ENS, and NIGHT;
- `earn`: Simple Earn in USDT and USDC and no on-chain earn in the baseline recording;
- `dual_investment`: one page with two `LIVE` HYPE/USDT subscriptions, one invested in each currency, and one earlier `SETTLED` BTC/USDT subscription excluded from Balances and Dual Positions;
- an asset valuation whose `earn` bucket the Earn check compares.

Tests read amounts and prices from these files rather than repeating them. They also take the futures sample from the recording, since it changes as contracts expire. One test checks that every recorded success answers a request the adapter sends. Another replays the recorded valuation without the Earn Balances it values, so the Earn check must fail on the recorded bucket.

The refreshed order history and the baseline valuation were recorded at different times. Their joint replay checks order mapping and report behavior; the read-only live test checks Earn completeness against a contemporaneous valuation.

**Documented** (`testdata/documented/<scenario>/`, [ADR 0027](../adr/0027-allow-documented-adapter-scenarios-without-live-coverage.md)). These files cover paths the test account cannot exercise. Each header has:

- `scenario` `documented` and no `recordedAt`;
- a `documentation` link to the OKX page the body comes from;
- a `trimmed` note naming every change made to OKX's example.

A test overlays one scenario on the recorded set, replacing the files for the same endpoints. Tests refuse a recorded header without a recording time, and a documented header without a documentation link or with a recording time.

| Scenario | Endpoints | Covers |
| --- | --- | --- |
| `okx_spot` | `account/config` (`acctLv 1`), `account/balance`, `account/positions` | 0.8 BTC and a 100 USDT borrowing with 0.0125 USDT interest. It checks the Spot mode margin figures, per-coin `liab` and `interest`, the absent `availEq` and `upl`, and no positions. |
| `okx_futures` | `account/config` (`acctLv 2`), `account/balance`, `account/positions` | Only `totalEq` at account level, per-coin `availEq` and `upl`, no liabilities. An isolated `MARGIN` row is skipped, and a cross `BTC-USDT-SWAP` long is mapped. |
| `dual_investment` | `finance/sfp/dcd/order-history` | `live` 1.5 BTC, `pending_redeem` 2000 USDT, and `redeeming` 3 ETH are counted. The documentation's `settled` order is not. |

Inline responses in the tests cover:

- Margin Mode detection, including `acctLv` `3`, `5`, and empty;
- every dual investment state in documented lowercase, uppercase, and mixed case; unknown states still fail selected Earn and Dual Accounts;
- paging, ordering, and the 1,000-order limit;
- the pause between pages;
- the Earn check's tolerance;
- a numeric `code`.

## Coverage

On 2026-09-25 the live smoke test ran on the test account:

- **Margin Mode.** It read `okx_pm` from the account and confirmed that another Platform's Account Structure is refused.
- **Accounts.** It collected all three Accounts: `trading` had two coins, one position, and reported equity; `funding` had four coins; `earn` had two coins.
- **Dual investment.** The history answered with an empty list and a numeric `code`.
- **Earn check.** The asset valuation answered, and the Earn check passed.
- **Assertions.** The test checks structure and success, never numbers.

On 2026-09-29 the split-account live test passed with one outstanding Dual subscription. Its normalized principal was 12.00 USD; the platform Earn bucket minus ordinary Earn was 12.33 USD, leaving a 0.33 USD residual difference. The combined Earn/Dual value differed by −0.011%, Funding by +0.002%, and Trading by −0.037%; Classic was empty. This verifies a real order identifier and nonempty principal mapping. Since the Dual amount is below 1% of total Earn, the independent residual is also logged rather than relying only on aggregate tolerance. No raw payload was recorded during this run.

Later on 2026-09-29, the refreshed dual order-history fixture captured two `LIVE` subscriptions and one `SETTLED` order. Both active orders expose `settleTime` and omit `expTime`; the recorded replay verifies their distinct invested currencies, directions, principal, target, annualized rate, and scheduled expiry. A subsequent read-only live test normalized two subscriptions, excluded the settled order, passed the combined Earn check, and matched OKX's Earn, Funding, and Trading valuation buckets within the live test's tolerance. This covers the nonempty order-history shape but not pagination or a settlement during collection.

Not covered live:

- Spot mode, Futures mode, and Multi-currency margin;
- multiple pages of nonempty dual investment history;
- a settlement inside a Collection Run;
- options;
- the US, Australian, and European hosts.

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
