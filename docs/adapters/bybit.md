# Bybit adapter

Package `internal/adapters/bybit`. Bybit has one Account Structure, `bybit_uta_2_0`: the global Unified Trading Account 2.0, including UTA 2.0 Pro (`unifiedMarginStatus` 5 and 6). Its Account Types are `unified`, `funding`, `earn`, and `dual_investment`. The account's Margin Mode is observed, never declared ([ADR 0028](../adr/0028-separate-account-structure-from-margin-mode.md)). It is Isolated Margin (`bybit_isolated`), Cross Margin (`bybit_cross`), or Portfolio Margin (`bybit_pm`), and it decides which `unified` figures a report carries.

Classic accounts, UTA 1.0, UTA 1.0 Pro, and regional services such as Bybit EU, KZ, and GE are not supported. Options are out of scope: an open option position fails the `unified` report. Every call is a read-only GET of an account, balance, position, Earn position, or market-data endpoint on `https://api.bybit.com`. The test account is a UTA 2.0 Pro in Portfolio Margin, so Isolated and Cross Margin are implemented from Bybit's documentation and have no live coverage yet ([ADR 0027](../adr/0027-allow-documented-adapter-scenarios-without-live-coverage.md)).

## Endpoints

| Use | Endpoint | Signed | Key permission | Bybit limit |
| --- | --- | --- | --- | --- |
| Clock sync | `GET /v5/market/time` | no | none | IP limit only |
| Account Structure and Margin Mode, at validation and at the start of every Collection Run | `GET /v5/account/info` | yes | not documented | 50 / s |
| Validate: UID | `GET /v5/user/query-api` | yes | any | 10 / s |
| Prices (every spot pair in one call, fetched right after the structure check so their time precedes every report) | `GET /v5/market/tickers?category=spot` | no | none | IP limit only |
| `unified` | `GET /v5/account/wallet-balance?accountType=UNIFIED` | yes | not documented | 50 / s |
| `unified`: positions, paged by 200 | `GET /v5/position/list` for linear `settleCoin=USDT`, linear `settleCoin=USDC`, `inverse`, and `option` | yes | not documented | 50 / s |
| `unified`: instruments, paged by 1000, fetched only for a category with an open position | `GET /v5/market/instruments-info?category=linear` and `category=inverse` | no | none | IP limit only |
| `funding` | `GET /v5/asset/transfer/query-account-coins-balance?accountType=FUND` | yes | not documented | 5 / s |
| `earn`: Flexible Savings | `GET /v5/earn/position?category=FlexibleSaving` | yes | Earn | not documented |
| `earn`: Fixed-Term Savings | `GET /v5/earn/fixed-term/position?category=FixedTermSaving` | yes | Earn | 10 / s |
| `dual_investment`: Dual Asset, paged by 20 | `GET /v5/earn/advance/position?category=DualAssets` | yes | Earn | 10 / s |
| `earn`: completeness check | `GET /v5/asset/asset-overview?valuationCurrency=USD` | yes | not documented | 50 / s |

Signed limits are per UID. Every call also counts against Bybit's IP limit of 600 requests per 5 seconds, and an IP over it is refused with HTTP 403 for at least ten minutes.

Bybit documents the Earn permission for the three Earn position reads and no permission for the other signed reads. The test key is read-only (`readOnly = 1`) and has, among others, the Contract, Spot, Wallet, Options, Derivatives, Exchange, and Earn permission groups. A narrower key has not been tried. Bybit refuses a read the key has no permission for with `10005`, an `authorization` failure that invalidates the Connection. So a key without the Earn permission, or without whatever permission `asset-overview` needs, invalidates the Connection whenever `earn` or `dual_investment` is selected. Such a key can still serve an Identity that selects neither `earn` nor `dual_investment`.

Signed requests carry `X-BAPI-API-KEY`, `X-BAPI-TIMESTAMP`, `X-BAPI-RECV-WINDOW` (5000 ms), and `X-BAPI-SIGN`, the hex HMAC-SHA256 of timestamp + key + receive window + query string. The timestamp is corrected by the offset to Bybit server time, taken at the midpoint of a `/v5/market/time` round trip. The offset is measured before the first signed call and again after a `10002` or `-1` answer, which is then retried once. With all four Accounts selected and no position open, a Collection Run makes eleven signed calls and one public one. Every further page, every category with an open position, and every clock sync adds one call.

## Account Structure and Margin Mode

Validation and every Collection Run start with `/v5/account/info` ([ADR 0025](../adr/0025-re-check-the-account-structure-in-every-collection-run.md)). The User can change the Margin Mode in the Bybit app at any time. The adapter returns it with the validation result and with every batch.

| `/v5/account/info` | Result |
| --- | --- |
| `unifiedMarginStatus` 5 or 6, `marginMode: ISOLATED_MARGIN` | `bybit_isolated` |
| `unifiedMarginStatus` 5 or 6, `marginMode: REGULAR_MARGIN` | `bybit_cross` |
| `unifiedMarginStatus` 5 or 6, `marginMode: PORTFOLIO_MARGIN` | `bybit_pm` |
| `unifiedMarginStatus` 1 (classic), 3 (UTA 1.0), 4 (UTA 1.0 Pro), or an undocumented status | `unsupported` before any Account is read, naming Bybit's status and the Identity's structure. The User creates a new Identity for the new structure and archives the old one |
| any other `marginMode` | `unsupported` before any Account is read, naming Bybit's value |
| `unifiedMarginStatus` or `marginMode` missing | `normalization` |

A structure the adapter does not know, such as another Platform's, fails with `unsupported` before any call. So does an Account Type the structure does not hold, which fails only its own Account. `ExternalRef` is `userID` from `/v5/user/query-api`, the UID of the account the key belongs to.

## Account Report

| Plutus column | `unified`, Cross and Portfolio Margin | `unified`, Isolated Margin | `funding`, `earn` |
| --- | --- | --- | --- |
| `equity` | `totalEquity` (USD, before collateral haircuts) | null, so the value is `calculated` from Balances | null, so the value is `calculated` from Balances |
| `adjusted_equity` | null | null | null |
| `available_equity` | `totalAvailableBalance` (USD, after haircuts and initial margin) | null | null |
| `initial_margin_ratio` | `accountIMRate` as reported | null | null |
| `maintenance_margin_ratio` | `accountMMRate` as reported | null | null |
| `unified_margin_ratio` | null | null | null |
| `observed_at` | local time when the Account's responses arrived | same | same |

- Bybit documents every account-wide field as not applicable to Isolated Margin, so the adapter ignores them there, whatever Bybit returns.
- In Cross and Portfolio Margin, an empty field stays null, never zero. A field that is present but not a decimal fails the report with `normalization`.
- `accountIMRate` and `accountMMRate` are Bybit's own ratios. They divide the initial or maintenance margin by the margin balance in Cross Margin, and by the collateral-adjusted equity in Portfolio Margin.
- `totalMarginBalance` is the wallet balance plus perpetual and futures unrealized PnL, not a haircut-adjusted equity, so `adjusted_equity` stays null.
- Not stored: `totalWalletBalance`, `totalMarginBalance`, `totalPerpUPL`, `totalInitialMargin`, `totalMaintenanceMargin`, the `…ByMp` duplicates, and the deprecated `accountLTV`.

## Balance

| Plutus column | `unified` (`wallet-balance` `coin[]`) | `funding` (`query-account-coins-balance`) | `earn` |
| --- | --- | --- | --- |
| `symbol` | `coin` | `coin` | `coin`; Dual Asset `investCoin` |
| `equity` (coin units) | `equity` as reported, may be negative | `walletBalance` | principal, see [Earn](#earn), one row per coin |
| `available_to_trade` | null | null | null |
| `available_to_withdraw` | null | `transferBalance` | null |
| `liability` | `borrowAmount`, stored as its absolute value | null | null |
| `interest` | `accruedInterest` | null | null |
| `unrealized_pnl` | `unrealisedPnl` | null | null |
| `name` | not mapped: Bybit gives no full coin name | not mapped | not mapped |

- `equity` is Bybit's per-coin equity: the wallet balance minus spot borrowing, plus unrealized PnL and option value. It is taken as reported and never rebuilt, so borrowing and PnL are not added or subtracted again. `walletBalance` includes borrowed coins and is not used.
- `equity`, `walletBalance`, and each Earn `amount` are required. An empty one fails its Account with `normalization`, naming the coin: read as zero, it would drop the coin under the Complete Set rule. The other fields may be empty and then stay null.
- A row without a coin, or a coin listed twice, fails its Account with `normalization`.
- `unified` rows whose mapped figures are all zero and `funding` rows with a zero `walletBalance` are dropped.
- Not mapped: `usdValue`, `walletBalance`, `locked`, `spotBorrow`, `spotHedgingQty`, `totalOrderIM`, `totalPositionIM`, `totalPositionMM`, `cumRealisedPnl`, `bonus`, `marginCollateral`, `collateralSwitch`, `colRes`, and the deprecated `free` and `availableToWithdraw`; Funding `bonus`.

## Earn

The `earn` Account has one Balance per coin: the summed principal of Flexible and Fixed-Term Savings ([ADR 0033](../adr/0033-separate-dual-investment-accounts-and-positions.md)).

| Product | Counted |
| --- | --- |
| Flexible Savings | `amount` of every row. Bybit also lists fully redeemed positions, and a coin whose total is zero is dropped. `claimableYield` is not counted: on the recorded account Bybit's own Earn value matched the summed `amount`, not `amount` plus yield |
| Fixed-Term Savings | `amount` of every row. Bybit lists only running positions, `Active` and `EarlyRedemptionProcessing`. `effectiveAmount` takes effect a day later and is not used |
| Dual Asset (separate `dual_investment` Account) | `amount` in `investCoin`: the quote coin of a `BuyLow` position, the base coin of a `SellHigh` one. Every page is read |

| Dual Asset `status` | Counted |
| --- | --- |
| `Active`, `Redeeming` | yes, whatever the settlement will pay, including in the other coin. `expectReturnAmount` is not counted before Bybit pays it |
| any other | the selected Earn and Dual Accounts fail with `normalization`, naming the status |

Neither savings endpoint pages, so a `nextPageCursor` there fails the Account instead of cutting the list short. A row whose `category` differs from the one requested also fails it. The fixed-term read leaves out FundPool and FundPoolPremium. On-chain Earn, BYUSDT, Discount Buy, Double Win, Smart Leverage, Liquidity Mining, and any later product are not read either. The completeness check catches value held in them.

**Completeness check.** The adapter also reads Bybit's own USD valuation of Earn, the `totalEquity` of the `Earn` entry of `/v5/asset/asset-overview`. It compares that figure with the combined ordinary Earn and Dual Balances valued at the batch's spot prices, with USDT and USDC at 1 USD. If Bybit's value exceeds the itemized value by more than max(1%, 1 USD), the selected Earn and Dual Accounts fail with `unsupported`, naming both amounts, and go Stale ([ADR 0019](../adr/0019-carry-a-failed-account-as-stale-and-treat-success-as-complete.md)). A list without an `Earn` entry counts as zero. A Balance without a price skips the check, because valuation already marks it `no_price`. An `Earn` entry valued in another currency, or with a `totalEquity` that is not a decimal, fails with `normalization`. The check catches value Plutus does not read, not value it counts twice.

**Settlement window.** Dual Asset settles at 08:00 UTC on the settlement date, and Bybit pays out to Funding (`toAccountType`, currently always `FUND`) within 5 to 30 minutes. A Collection Run in that window can count the principal in `dual_investment` and the payout in `funding` at once, a double count, or neither, a dip, depending on which side has updated. The next run after the payout is correct again. Both are expected and are not reconciled across Accounts. When Bybit's Earn valuation still includes a position that has already left the list, the completeness check fails and selected Earn and Dual Accounts are carried Stale for that run instead of dipping.

## Dual Investment Account and positions

`dual_investment` is a separately selected product Account, not an independently transferable cash wallet. Its Balances aggregate outstanding principal by invested currency; available and withdrawable values remain null. Each subscription is stored separately and never adds a second value to its Balance. The Positions page separates Futures and Dual Investment.

Selecting either `earn` or `dual_investment` reads both product groups once for the combined Earn completeness check. Only selected reports leave the Adapter. Required auxiliary failures and unexplained aggregate shortfalls fail both selected dependent Accounts; unrelated Accounts retain their normal failure isolation. Neither product group is read when neither is selected. Missing optional terms are null with display issues; missing identity/principal or contradictory fields fail normalization. See [ADR 0033](../adr/0033-separate-dual-investment-accounts-and-positions.md).

| Dual Position field | Source |
| --- | --- |
| Subscription identity | `positionId`; `productId` and `orderId` are separate |
| Principal | `amount` of `investCoin` |
| Pair and direction | `baseCoin` / `quoteCoin`; `BuyLow` or `SellHigh` |
| Target, expiry | `targetPrice`, `settlementTime` milliseconds |
| Annualized rate | `apyE8 / 100000000`, tagged APY |

`accountType` identifies the subscription source (`FUND` or `UNIFIED`), and `toAccountType` the settlement destination (`FUND`). Neither establishes an unused Dual wallet balance. The overview exposes generic categories but does not define a reliable Dual category name, so runtime completeness compares the combined groups. Pending orders before a position exists and failed-order refunds are not added to principal without evidence of where their funds reside.

## Position

`/v5/position/list` is read to its last page for linear USDT, linear USDC, inverse, and option positions before any position is kept.

| Plutus column | Source |
| --- | --- |
| `platform_position_ref` | `<symbol>:<positionIdx>`, such as `DOGEUSDT:0`. `positionIdx` 1 and 2 are the buy and sell sides of hedge mode |
| `side` | `side` `Buy` → long, `Sell` → short |
| `quantity`, `quantity_unit` | `size`, always positive: in the base coin (`base`) for linear contracts, in USD (`quote`) for inverse contracts |
| `entry_price`, `mark_price` | `avgPrice`, `markPrice` |
| `unrealized_pnl` | `unrealisedPnl`, in the settlement coin |
| `liquidation_price` | Isolated Margin: `liqPrice`, the position's own. Cross Margin: `liqPrice`, Bybit's estimate for the whole account. Portfolio Margin: null, as Bybit provides none. Empty or `0` is null |
| `leverage` | `leverage`; null in Portfolio Margin, and when empty or `0` |
| `margin` | `positionIM`; null in Portfolio Margin, and when empty |

- Rows with `size = 0` are dropped.
- A nonzero `option` row fails the `unified` report with `normalization`, naming the contract, because Plutus has no Instrument kind for it.
- A position whose contract is missing from the instrument list fails the `unified` report with `normalization`, naming the contract. It is never dropped. A position reference listed twice fails the report too.

| Instrument column | Source (`/v5/market/instruments-info`) |
| --- | --- |
| `symbol` | `symbol` |
| `kind` | `contractType`: `LinearPerpetual` → `linear_perpetual`, `InversePerpetual` → `inverse_perpetual`, `LinearFutures` → `linear_delivery`, `InverseFutures` → `inverse_delivery` |
| `base_symbol`, `quote_symbol`, `settlement_symbol` | `baseCoin`, `quoteCoin`, `settleCoin` |
| `expiry_at` | `deliveryTime`, delivery contracts only |
| `contract_size` | not mapped: Bybit sizes positions in coins or USD, not in contracts |

An unknown contract type, a missing coin, or a delivery contract without `deliveryTime` fails the `unified` report with `normalization`.

## Prices

For every non-USD Balance symbol, the spot `lastPrice` of `<symbol>USDT`, else `<symbol>USDC`, observed when the ticker response arrived. USD, USDT, and USDC get no price row ([ADR 0017](../adr/0017-value-in-usd-and-treat-major-stablecoins-as-usd.md)). A symbol with neither pair gets no price and is valued as `no_price`. A failed ticker call leaves the batch without prices, and valuation falls back to the latest price within 24 hours. The Earn completeness check uses the same prices. Positions carry their own `markPrice`; contracts get no separate price.

## Errors

| Bybit answer | Class | Scope |
| --- | --- | --- |
| `10003` (unknown key), `10004` (wrong signature), `10007` (authentication failed), `33004`, `-2015` (expired key), HTTP 401 | `authentication` | the whole batch; the Connection becomes `invalid` |
| `10005` (permission denied), `10010` (IP not bound to the key) | `authorization` | the whole batch; the Connection becomes `invalid` |
| `10006`, `10018`, HTTP 429 | `rate_limited` | the Account; the batch when every Account failed |
| `10016`, HTTP 403 and 5xx, timeouts, network errors, a cut-off response, `10002` or `-1` after one resync, any other non-zero `retCode` | `temporary` | the Account; the batch when every Account failed |
| `10009` (region not served), `10028`, `100028` (endpoint for another account type), `180004` (not the global site), `180024` (not for sub-accounts) | `unsupported` | the Account |
| a structure other than UTA 2.0, or an unknown Margin Mode | `unsupported` | the whole batch; not retried, and the Connection stays enabled while its Accounts turn Stale |
| Bybit values Earn above the itemized Balances by more than max(1%, 1 USD) | `unsupported` | selected `earn` and `dual_investment` Accounts |
| unparseable or non-decimal fields; an empty `equity`, `walletBalance`, `amount`, or `size`; a row without a coin; an undocumented Dual Asset status; an open option position; an unknown contract; a repeated page cursor or a list longer than 1000 pages; `retCode 0` without a `result` | `normalization` | the Account |

Bybit answers most failures with HTTP 200 and a non-zero `retCode`, so both the status and the code decide. HTTP 403 means an IP over Bybit's request budget or a U.S. IP. A failure of `/v5/account/info` fails the whole batch whatever its class, because no Account is read before the structure check. Error details name the endpoint, the HTTP status, and Bybit's code, never Bybit's message, which can echo request parameters, and never a body or a key.

## Fixtures

Replay tests use two fixture sets, told apart by their headers:

- **Recorded** (`testdata/*.json`): the test account's answers, a UTA 2.0 Pro in Portfolio Margin, with `scenario: "recorded"`, `accountStructure: "bybit_uta_2_0"`, `marginMode: "bybit_pm"`, and a per-file `recordedAt` time. The baseline was recorded on 2026-09-25 and holds every read of validation and of a Collection Run with all four Accounts: one linear USDT perpetual position, followed by an empty second position page that Bybit's cursor leads to; Flexible Savings; empty Fixed-Term Savings; and the asset overview. Only the Dual Asset position answer was refreshed on 2026-09-29; it contains one active BuyLow subscription funded in USDT. The replay therefore combines answers recorded on different dates and is a field-mapping test, not a contemporaneous valuation snapshot. The three refusals carry their scenario name instead of `recorded`.
- **Documented** (`testdata/documented/<scenario>/*.json`, [ADR 0027](../adr/0027-allow-documented-adapter-scenarios-without-live-coverage.md)): built from Bybit's documentation examples, with `scenario: "documented"`, no `recordedAt`, the example's `documentation` URL, and the `marginMode` it describes. `trimmed` names every change from the example.
  - `bybit_isolated`: an Isolated Margin account with the documented wallet and an inverse position that reports its own liquidation price.
  - `bybit_cross`: a Cross Margin account with the documented wallet and inverse position, both verbatim.
  - `uta_1_0_pro`: the documented `/v5/account/info` example, a UTA 1.0 Pro account, which is refused.
  - `dual_investment`: the documented Dual Asset position with a matching Earn valuation.
  - `fixed_saving`: the documented Fixed-Term position plus one in early redemption, with a matching Earn valuation.
  - `region_restricted`: code `10009` on `/v5/account/info`, from the documented code and message.
  - `ip_rate_limited`: HTTP 403 on `/v5/account/info`, from the documented status and error text.
  - `rate_limited`: code `10006` on the `unified` and `funding` reads, from the documented rate-limit answer.

The replay lays at most one documented directory over the recorded set. It checks the header provenance of every file and the method of every request. Like Bybit, it answers a signed request with a wrong key, signature, receive window, or timestamp with the matching refusal, and the test fails unless it provoked that refusal on purpose. A documented scenario proves the mapping of the documented shape, not live access. The recorded refusals, replayed on the first call of a run and on an Account read, check the error classes, the code-only details, and the clock resynchronization. Error codes that neither a recording nor a documented answer shows are checked in a table. Paging, malformed answers, and unknown states are tested with synthetic answers written in the tests.

`plutus adapter record --platform bybit` reads `PLUTUS_GOTEST_BYBIT_APIKEY` / `_SECRET` from the environment or `config/gotest.env`. It calls every endpoint that validation and a Collection Run with all four Accounts use, plus both instrument lists, and writes `tmp/fixtures/bybit/<endpoint>.json`, with query values in the file name (`v5_account_wallet_balance.UNIFIED.json`). The file name takes each value as the adapter passed it, so a page cursor appears as Bybit returned it, already percent-encoded (`v5_position_list.linear.DOGEUSDT%2C1790352000006%2C0.200.USDT.json`). The header's `endpoint` is the exact request, which escapes the cursor once more (`%252C`). The header carries `accountStructure: "bybit_uta_2_0"` and the detected `marginMode`. Amounts are kept as recorded. The recorder:

For a focused refresh, use `--only v5_earn_advance_position.DualAssets.20` (repeat `--only` for more files). The recorder reads clock, structure, and key checks first, then only the requested account or product endpoint; it writes only that fixture. This supports first pages. Later pages, market-data lists, and refusal fixtures still use a full recording.

- replaces the values of identifying fields (`id`, `note`, `apiKey`, `secret`, `ips`, `userID`, `inviterID`, `affiliateID`, `parentUid`, `rsaPublicKey`, `kycRegion`, `memberId`, `positionId`, `orderId`, `smpGroup`) with `100001`, `"redacted"`, or an empty list or object;
- replaces every other value equal to one of those identifiers, when 8 characters or longer, in every file, every occurrence of the key or secret inside a string, and every IPv4 address with `203.0.113.1`. It writes nothing if the key, the secret, or the account UID survives;
- names in `redacted` the fields it replaced, plus `identifier echoes`, `API key and secret echoes`, and `IP addresses` when it replaced those;
- trims `market/tickers` and both instrument lists to the rows whose symbol is a held contract, a recorded coin's USDT or USDC pair, or starts with BTC or ETH. `trimmed` is set only on a list it shortened;
- records three refusals of `/v5/account/info`, the first call of every validation and Collection Run: a key Bybit does not know (`invalid_key`), a wrong signature (`bad_signature`), and a timestamp a minute old (`stale_timestamp`), as `v5_account_info.<scenario>.json`. It fails if Bybit accepts one. Bybit's `10004` message repeats the signed string, key included, so the key is replaced there. A refusal without a JSON body is kept as a JSON string. Rate-limit, region, and server errors are not provoked; the documented scenarios above stand in for them.

After owner review, a targeted refresh replaces only the selected endpoint fixture in `internal/adapters/bybit/testdata/`. A full-set refresh also removes files the recorder no longer writes. A replay test runs the recorder over the committed set and fails unless it writes exactly the committed files, each for the request its header names.

## Coverage

`make test-live` validates the key and checks that the Margin Mode is one of the three. It collects every Account and checks each report against its Margin Mode: no account-wide figures in Isolated Margin, equity in Cross and Portfolio Margin, and no leverage, margin, or liquidation price on Portfolio Margin positions. It reads the Dual Asset list. It compares every Account with its asset-overview entry within max(1%, 1 USD), and fails on a populated entry that no Account Type collects, such as copy trading or trading bots.

- **Live, 2026-09-25**: UTA 2.0 Pro in Portfolio Margin, with `unified` Balances and a linear USDT perpetual position, `funding` Balances, and Flexible Savings. Fixed-Term Savings and Dual Asset answered empty. Each Account matched Bybit's asset overview within 0.1%, so the Earn completeness check passed.
- **Recorded and live, 2026-09-29**: one active BuyLow Dual Asset position, with invested principal, pair, direction, target, maturity, and APY. The recorder redacted its position and order identifiers. Only this endpoint's fixture was refreshed. The live smoke passed with one Dual Balance and one Dual Asset position; the combined Earn valuation and the Unified and Funding overview comparisons passed.
- **Documented only**: Isolated Margin, Cross Margin, an inverse position, a UTA 1.0 Pro refusal, and nonempty Fixed-Term Savings. A documented Dual Asset scenario remains for comparison with the recorded shape.
- **Refusals**: an unknown key (`10003`), a wrong signature (`10004`), and a stale timestamp (`10002`) are recorded, each as an HTTP 200 answer. A region Bybit does not serve, an IP over the request budget, and a rate-limited UID are documented only.
- **Synthetic only**: an unknown Margin Mode, classic and undocumented account statuses, delivery contracts, hedge-mode references, an open option position, Dual Asset paging and undocumented states, and a failing completeness check.
- **Not covered**: UTA 2.0 without Pro (`unifiedMarginStatus` 5), sub-accounts, regional services, and a key with fewer permission groups.

## Comparison checklist (test environment)

Run these on the test environment against the Bybit app on the same account, within a few minutes of a collection. Items 5 and 6 need an Isolated or Cross Margin account. Item 8 still needs app comparison for the recorded Dual Asset position; Fixed-Term has only documented nonempty coverage.

1. **Identity and structure**: validation stores the UID shown under the app's profile. The Identity's Margin Mode matches the margin mode shown for the Unified Trading Account.
2. **Margin Mode change**: after switching the margin mode in the app, the next Collection Run succeeds and the Identity shows the new Margin Mode.
3. **Portfolio Margin equity**: `Equity` equals the Unified Trading Account's total equity in USD, `available_equity` its available balance, and the two margin ratios the IM and MM rates the app shows, possibly as percentages.
4. **Unified balances**: each coin's equity matches the app's per-coin equity, including unrealized PnL and net of spot borrowing. `liability` matches the borrowed amount.
5. **Cross Margin**: the account figures appear as in item 3. Positions show a leverage, a margin, and Bybit's estimated liquidation price.
6. **Isolated Margin**: the account figures are empty, and the value is calculated from the Balances. Each position shows its own liquidation price, leverage, and margin.
7. **Positions**: every open linear and inverse position appears with the same side, size (linear in coins, inverse in USD), entry price, and mark price. Portfolio Margin positions show no leverage, margin, or liquidation price.
8. **Funding and Earn**: Funding coin amounts and transferable amounts match the Funding account. Flexible and Fixed-Term principal per coin matches the Earn page, including an early redemption in progress. Each open Dual Asset position appears at its invested amount in the invested coin until Bybit pays it out. The Earn Account succeeds. Holding an Earn product Plutus does not read, such as On-chain Earn, makes it Stale with both USD amounts.
9. **Account values**: the calculated USD values of Funding and Earn are within about 0.1% of the app's asset overview.
10. **Failures**: removing the server IP from the key's whitelist sets the Connection `invalid` with a `10010` detail at the next cycle. Restoring it and validating again resumes collection.

On 2026-09-29, the Dual Asset position endpoint returned one active BuyLow subscription for the test account. Its recorded fixture now covers a nonempty Dual list; the other recorded endpoint answers retain their earlier dates. The live smoke checked the current combined Earn/Dual valuation against Bybit's Earn overview within tolerance. Direct comparison with the Bybit app remains to be done.
