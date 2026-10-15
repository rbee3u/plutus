# Bybit adapter mapping

Package `internal/platform/bybit`. Bybit has one account structure,
`bybit_uta_2_0`: the global Unified Trading Account 2.0, including UTA 2.0 Pro
(`unifiedMarginStatus` 5 and 6). Its account types are `unified`, `funding`,
`earn`, and `dual_investment`. The account's margin mode is observed, never
declared ([account model](../../adr/0001-ownership-and-account-boundaries.md)).
It is isolated margin (`bybit_isolated`), cross margin (`bybit_cross`), or
portfolio margin (`bybit_pm`), and it decides which `unified` figures a report
carries.

Classic accounts, UTA 1.0, UTA 1.0 Pro, and regional services such as Bybit EU,
KZ, and GE are not supported. Options are out of scope: an open option position
fails the `unified` report. Every call is a read-only GET of an account,
balance, position, earn position, or market-data endpoint on
`https://api.bybit.com`.

Current test coverage, reviewed responses, real gaps, and pending exchange-app
checks are in [Bybit verification](./verification.md).

## Endpoints

| Use                                                                                                               | Endpoint                                                                                                | Signed | Key permission | Bybit limit    |
| ----------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------- | ------ | -------------- | -------------- |
| Clock sync                                                                                                        | `GET /v5/market/time`                                                                                   | No     | None           | IP limit only  |
| Account structure and margin mode, at validation and at the start of every collection run                         | `GET /v5/account/info`                                                                                  | Yes    | Not documented | 50 / s         |
| Validate: UID                                                                                                     | `GET /v5/user/query-api`                                                                                | Yes    | Any            | 10 / s         |
| Prices (every spot pair in one call, fetched right after the structure check so their time precedes every report) | `GET /v5/market/tickers?category=spot`                                                                  | No     | None           | IP limit only  |
| `unified`                                                                                                         | `GET /v5/account/wallet-balance?accountType=UNIFIED`                                                    | Yes    | Not documented | 50 / s         |
| `unified`: positions, paged by 200                                                                                | `GET /v5/position/list` for linear `settleCoin=USDT`, linear `settleCoin=USDC`, `inverse`, and `option` | Yes    | Not documented | 50 / s         |
| `unified`: futures contracts, paged by 1000, fetched only for a category with an open position                    | `GET /v5/market/instruments-info?category=linear` and `category=inverse`                                | No     | None           | IP limit only  |
| `funding`                                                                                                         | `GET /v5/asset/transfer/query-account-coins-balance?accountType=FUND`                                   | Yes    | Not documented | 5 / s          |
| `earn`: Flexible Savings                                                                                          | `GET /v5/earn/position?category=FlexibleSaving`                                                         | Yes    | Earn           | Not documented |
| `earn`: Fixed-Term Savings                                                                                        | `GET /v5/earn/fixed-term/position?category=FixedTermSaving`                                             | Yes    | Earn           | 10 / s         |
| `dual_investment`: dual asset, paged by 20                                                                        | `GET /v5/earn/advance/position?category=DualAssets`                                                     | Yes    | Earn           | 10 / s         |
| `earn`: completeness check                                                                                        | `GET /v5/asset/asset-overview?valuationCurrency=USD`                                                    | Yes    | Not documented | 50 / s         |

Signed limits are per UID. Every call also counts against Bybit's IP limit of
600 requests per 5 seconds, and an IP over it is refused with HTTP 403 for at
least ten minutes.

Bybit documents the earn permission for the three earn position reads and no
permission for the other signed reads. The test key is read-only
(`readOnly = 1`) and has, among others, the contract, Spot, wallet, Options,
Derivatives, Exchange, and earn permission groups. A narrower key has not been
tried. Bybit refuses a read the key has no permission for with `10005`, an
`authorization` failure that invalidates the connection. So a key without the
earn permission, or without whatever permission `asset-overview` needs,
invalidates the connection whenever `earn` or `dual_investment` is selected.
Such a key can still serve an identity that selects neither `earn` nor
`dual_investment`.

Signed requests carry `X-BAPI-API-KEY`, `X-BAPI-TIMESTAMP`, `X-BAPI-RECV-WINDOW`
(5000 ms), and `X-BAPI-SIGN`, the hex HMAC-SHA256 of timestamp + key + receive
window + query string. The timestamp is corrected by the offset to Bybit server
time, taken at the midpoint of a `/v5/market/time` round trip. The offset is
measured before the first signed call and again after a `10002` or `-1` answer,
which is then retried once. With all four accounts selected and no position
open, a collection run makes eleven signed calls and one public one. Every
further page, every category with an open position, and every clock sync adds
one call.

## Account structure and margin mode

Validation and every collection run start with `/v5/account/info`
([account model](../../adr/0001-ownership-and-account-boundaries.md)). The user
can change the margin mode in the Bybit app at any time. The adapter returns it
with the validation result and with every batch.

| `/v5/account/info`                                                                         | Result                                                                                                                                                                       |
| ------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `unifiedMarginStatus` 5 or 6, `marginMode: ISOLATED_MARGIN`                                | `bybit_isolated`                                                                                                                                                             |
| `unifiedMarginStatus` 5 or 6, `marginMode: REGULAR_MARGIN`                                 | `bybit_cross`                                                                                                                                                                |
| `unifiedMarginStatus` 5 or 6, `marginMode: PORTFOLIO_MARGIN`                               | `bybit_pm`                                                                                                                                                                   |
| `unifiedMarginStatus` 1 (classic), 3 (UTA 1.0), 4 (UTA 1.0 Pro), or an undocumented status | `unsupported` before any account is read, naming Bybit's status and the identity's structure. The user creates a new identity for the new structure and archives the old one |
| Any other `marginMode`                                                                     | `unsupported` before any account is read, naming Bybit's value                                                                                                               |
| `unifiedMarginStatus` or `marginMode` missing                                              | `normalization`                                                                                                                                                              |

A structure the adapter does not know, such as another platform's, fails with
`unsupported` before any call. So does an account type the structure does not
hold, which fails only its own account. `ExternalRef` is `userID` from
`/v5/user/query-api`, the UID of the account the key belongs to.

## Account report

| Plutus column                       | `unified`, cross and portfolio margin                            | `unified`, isolated margin                         | `funding`, `earn`                                  |
| ----------------------------------- | ---------------------------------------------------------------- | -------------------------------------------------- | -------------------------------------------------- |
| `reported_equity_usd`               | `totalEquity` (USD, before collateral haircuts)                  | `null`, so the value is `calculated` from balances | `null`, so the value is `calculated` from balances |
| `adjusted_equity_usd`               | `null`                                                           | `null`                                             | `null`                                             |
| `available_equity_usd`              | `totalAvailableBalance` (USD, after haircuts and initial margin) | `null`                                             | `null`                                             |
| `reported_initial_margin_ratio`     | `accountIMRate` as reported                                      | `null`                                             | `null`                                             |
| `reported_maintenance_margin_ratio` | `accountMMRate` as reported                                      | `null`                                             | `null`                                             |
| `reported_margin_ratio`             | `null`                                                           | `null`                                             | `null`                                             |
| `collected_at`                      | Local time when the account's responses arrived                  | Same                                               | Same                                               |
| `source_observed_at`                | `null`                                                           | `null`                                             | `null`                                             |

- Bybit documents every account-wide field as not applicable to isolated margin,
  so the adapter ignores them there, whatever Bybit returns.
- In cross and portfolio margin, an empty field stays null, never zero. A field
  that is present but not a decimal fails the report with `normalization`.
- `accountIMRate` and `accountMMRate` are Bybit's own ratios. They divide the
  initial or maintenance margin by the margin balance in cross margin, and by
  the collateral-adjusted equity in portfolio margin.
- `totalMarginBalance` is the wallet balance plus perpetual and futures
  unrealized PnL, not a haircut-adjusted equity, so `adjusted_equity_usd` stays
  null.
- Not stored: `totalWalletBalance`, `totalMarginBalance`, `totalPerpUPL`,
  `totalInitialMargin`, `totalMaintenanceMargin`, the `…ByMp` duplicates, and
  the deprecated `accountLTV`.

## Balance

| Plutus column           | `unified` (`wallet-balance` `coin[]`)        | `funding` (`query-account-coins-balance`) | `earn`                                         |
| ----------------------- | -------------------------------------------- | ----------------------------------------- | ---------------------------------------------- |
| `AssetSymbol`           | `coin`                                       | `coin`                                    | `coin`; dual asset `investCoin`                |
| `equity` (coin units)   | `equity` as reported, may be negative        | `walletBalance`                           | Principal, see [Earn](#earn), one row per coin |
| `available_to_trade`    | `null`                                       | `null`                                    | `null`                                         |
| `available_to_withdraw` | `null`                                       | `transferBalance`                         | `null`                                         |
| `liability`             | `borrowAmount`, stored as its absolute value | `null`                                    | `null`                                         |
| `interest`              | `accruedInterest`                            | `null`                                    | `null`                                         |
| `unrealized_pnl`        | `unrealisedPnl`                              | `null`                                    | `null`                                         |
| `name`                  | Not mapped: Bybit gives no full coin name    | Not mapped                                | Not mapped                                     |

- `equity` is Bybit's per-coin equity: the wallet balance minus spot borrowing,
  plus unrealized PnL and option value. It is taken as reported and never
  rebuilt, so borrowing and PnL are not added or subtracted again.
  `walletBalance` includes borrowed coins and is not used.
- `equity`, `walletBalance`, and each earn `amount` are required. An empty one
  fails its account with `normalization`, naming the coin: read as zero, it
  would drop the coin under the complete set rule. The other fields may be empty
  and then stay null.
- A row without a coin, or a coin listed twice, fails its account with
  `normalization`.
- `unified` rows whose mapped figures are all zero and `funding` rows with a
  zero `walletBalance` are dropped.
- Not mapped: `usdValue`, `walletBalance`, `locked`, `spotBorrow`,
  `spotHedgingQty`, `totalOrderIM`, `totalPositionIM`, `totalPositionMM`,
  `cumRealisedPnl`, `bonus`, `marginCollateral`, `collateralSwitch`, `colRes`,
  and the deprecated `free` and `availableToWithdraw`; funding `bonus`.

## Earn

The `earn` account has one balance per coin: the summed principal of flexible
and Fixed-Term Savings
([dual investment decision](../../adr/0005-earn-and-dual-products.md)).

| Product                                         | Counted                                                                                                                                                                                                                     |
| ----------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Flexible Savings                                | `amount` of every row. Bybit also lists fully redeemed positions, and a coin whose total is zero is dropped. `claimableYield` is not counted: the principal is the documented `amount`; claimable yield is a separate field |
| Fixed-Term Savings                              | `amount` of every row. Bybit lists only running positions, `Active` and `EarlyRedemptionProcessing`. `effectiveAmount` takes effect a day later and is not used                                                             |
| Dual asset (separate `dual_investment` account) | `amount` in `investCoin`: the quote coin of a `BuyLow` position, the base coin of a `SellHigh` one. Every page is read                                                                                                      |

| Dual asset `status`   | Counted                                                                                                                      |
| --------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| `Active`, `Redeeming` | Yes, whatever the settlement will pay, including in the other coin. `expectReturnAmount` is not counted before Bybit pays it |
| Any other             | The selected earn and dual accounts fail with `normalization`, naming the status                                             |

Neither savings endpoint pages, so a `nextPageCursor` there fails the account
instead of cutting the list short. A row whose `category` differs from the one
requested also fails it. The fixed-term read leaves out FundPool and
FundPoolPremium. On-chain earn, BYUSDT, Discount Buy, Double Win, Smart
Leverage, Liquidity Mining, and any later product are not read either. The
completeness check catches value held in them.

**Completeness check.** The adapter also reads Bybit's own USD valuation of
earn, the `totalEquity` of the `Earn` entry of `/v5/asset/asset-overview`. It
compares that figure with the combined ordinary earn and dual balances valued at
the batch's spot prices, with USDT and USDC at 1 USD. If Bybit's value exceeds
the itemized value by more than max(1%, 1 USD), the selected earn and dual
accounts fail with `unsupported`, naming both amounts, and go stale
([report policy](../../adr/0002-reports-and-compatible-history.md)). A list
without an `Earn` entry counts as zero. A balance without a price skips the
check, because valuation already marks it `no_price`. An `Earn` entry valued in
another currency, or with a `totalEquity` that is not a decimal, fails with
`normalization`. The check catches value Plutus does not read, not value it
counts twice.

**Settlement window.** Dual asset settles at 08:00 UTC on the settlement date,
and Bybit pays out to funding (`toAccountType`, currently always `FUND`) within
5 to 30 minutes. A collection run in that window can count the principal in
`dual_investment` and the payout in `funding` at once, a double count, or
neither, a dip, depending on which side has updated. The next run after the
payout is correct again. Both are expected and are not reconciled across
accounts. When Bybit's earn valuation still includes a position that has already
left the list, the completeness check fails and selected earn and dual accounts
are carried stale for that run instead of dipping.

### Earn position descriptions

Flexible Savings rows remain product aggregates: `amount` is principal and
`claimableYield` is Pending yield. Other reported earnings keep their own scope
in details. Fixed-Term rows retain position IDs, APY by reward currency,
subscription and settlement times, estimated rewards, and native status.

Only a collection run selecting earn with nonzero Flexible Savings makes one
additional unsigned
[`GET /v5/earn/product?category=FlexibleSaving`](https://bybit-exchange.github.io/docs/v5/finance/earn/easy-onchain/product-info)
read. Product and coin must match exactly once to supply Estimated APR; failures
and missing matches add metadata issues without discarding holdings or affecting
valuation. Catalogue availability is never a holding status. Fixed-Term and
catalogue recorded evidence gaps remain documented in the
[earn delivery report](../earn-presentation-delivery.md).

## Dual investment account and positions

Dual account selection, principal valuation, and shared earn completeness follow
[domain rules](../../domain-rules.md#dual-investment-and-earn-completeness).
Platform-specific fields map as follows:

| Dual position field   | Source                                               |
| --------------------- | ---------------------------------------------------- |
| Subscription identity | `positionId`; `productId` and `orderId` are separate |
| Principal             | `amount` of `investCoin`                             |
| Pair and direction    | `baseCoin` / `quoteCoin`; `BuyLow` or `SellHigh`     |
| Target, expiry        | `targetPrice`, `settlementTime` milliseconds         |
| Annualized rate       | `apyE8 / 100000000`, tagged APY                      |

`accountType` identifies the subscription source (`FUND` or `UNIFIED`), and
`toAccountType` the settlement destination (`FUND`). Neither establishes an
unused dual wallet balance. The overview exposes generic categories but does not
define a reliable dual category name, so runtime completeness compares the
combined groups. Pending orders before a position exists and failed-order
refunds are not added to principal without evidence of where their funds reside.

## Position

`/v5/position/list` is read to its last page for linear USDT, linear USDC,
inverse, and option positions before any position is kept.

| Plutus column                             | Source                                                                                                                                                                                  |
| ----------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `position_key`                            | `<symbol>:<positionIdx>`, such as `DOGEUSDT:0`. `positionIdx` 1 and 2 are the buy and sell sides of hedge mode                                                                          |
| `side`                                    | `side` `Buy` → long, `Sell` → short                                                                                                                                                     |
| `source_quantity`, `source_quantity_unit` | `size`, always positive: in the base coin (`base`) for linear contracts, in USD (`quote`) for inverse contracts                                                                         |
| `entry_price`, `mark_price`               | `avgPrice`, `markPrice`                                                                                                                                                                 |
| `unrealized_pnl`                          | `unrealisedPnl`, in the settlement coin                                                                                                                                                 |
| `liquidation_price`                       | Isolated margin: `liqPrice`, the position's own. Cross margin: `liqPrice`, Bybit's estimate for the whole account. Portfolio margin: null, as Bybit provides none. Empty or `0` is null |
| `leverage`                                | `leverage`; null in portfolio margin, and when empty or `0`                                                                                                                             |
| `margin`                                  | `positionIM`; null in portfolio margin, and when empty                                                                                                                                  |

- Rows with `size = 0` are dropped.
- A nonzero `option` row fails the `unified` report with `normalization`, naming
  the contract, because Plutus has no futures contract kind for it.
- A position whose contract is missing from the futures contract list fails the
  `unified` report with `normalization`, naming the contract. It is never
  dropped. A position reference listed twice fails the report too.

| Futures contract column                                              | Source (`/v5/market/instruments-info`)                                                                                                                                       |
| -------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `symbol`                                                             | `symbol`                                                                                                                                                                     |
| `kind`                                                               | `contractType`: `LinearPerpetual` → `linear_perpetual`, `InversePerpetual` → `inverse_perpetual`, `LinearFutures` → `linear_delivery`, `InverseFutures` → `inverse_delivery` |
| `base_asset_symbol`, `quote_asset_symbol`, `settlement_asset_symbol` | `baseCoin`, `quoteCoin`, `settleCoin`                                                                                                                                        |
| `expiry_at`                                                          | `deliveryTime`, delivery contracts only                                                                                                                                      |
| `contract_size`                                                      | Not mapped: Bybit sizes positions in coins or USD, not in contracts                                                                                                          |

An unknown contract type, a missing coin, or a delivery contract without
`deliveryTime` fails the `unified` report with `normalization`.

## Prices

For every non-USD balance symbol, the spot `lastPrice` of `<symbol>USDT`, else
`<symbol>USDC`, observed when the ticker response arrived. USD, USDT, and USDC
get no price row
([valuation policy](../../adr/0003-valuation-and-asset-policy.md)). A symbol
with neither pair gets no price and is valued as `no_price`. A failed ticker
call leaves the batch without prices, and valuation falls back to the latest
price within 24 hours. The earn completeness check uses the same prices.
Positions carry their own `markPrice`; contracts get no separate price.

## Errors

| Bybit answer                                                                                                                                                                                                                                                                               | Class            | Scope                                                                                        |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------------- | -------------------------------------------------------------------------------------------- |
| `10003` (unknown key), `10004` (wrong signature), `10007` (authentication failed), `33004`, `-2015` (expired key), HTTP 401                                                                                                                                                                | `authentication` | The whole batch; the connection becomes `invalid`                                            |
| `10005` (permission denied), `10010` (IP not bound to the key)                                                                                                                                                                                                                             | `authorization`  | The whole batch; the connection becomes `invalid`                                            |
| `10006`, `10018`, HTTP 429                                                                                                                                                                                                                                                                 | `rate_limited`   | The account; the batch when every account failed                                             |
| `10016`, HTTP 403 and 5xx, timeouts, network errors, a cut-off response, `10002` or `-1` after one resync, any other non-zero `retCode`                                                                                                                                                    | `temporary`      | The account; the batch when every account failed                                             |
| `10009` (region not served), `10028`, `100028` (endpoint for another account type), `180004` (not the global site), `180024` (not for sub-accounts)                                                                                                                                        | `unsupported`    | The account                                                                                  |
| A structure other than UTA 2.0, or an unknown margin mode                                                                                                                                                                                                                                  | `unsupported`    | The whole batch; not retried, and the connection stays enabled while its accounts turn stale |
| Bybit values earn above the itemized balances by more than max(1%, 1 USD)                                                                                                                                                                                                                  | `unsupported`    | Selected `earn` and `dual_investment` accounts                                               |
| Unparseable or non-decimal fields; an empty `equity`, `walletBalance`, `amount`, or `size`; a row without a coin; an undocumented dual asset status; an open option position; an unknown contract; a repeated page cursor or a list longer than 1000 pages; `retCode 0` without a `result` | `normalization`  | The account                                                                                  |

Bybit answers most failures with HTTP 200 and a non-zero `retCode`, so both the
status and the code decide. HTTP 403 means an IP over Bybit's request budget or
a U.S. IP. A failure of `/v5/account/info` fails the whole batch whatever its
class, because no account is read before the structure check. Error details name
the endpoint, the HTTP status, and Bybit's code, never Bybit's message, which
can echo request parameters, and never a body or a key.

### Position presentation provenance

`positionIdx=0` retains one-way mode; 1/2 retain hedged long/short mode and must
agree with Buy/Sell. The report retains the observed UTA margin mode.
`positionIM` remains a source initial requirement in settlement currency. It
serves the cross display, but is not an allocated isolated margin amount.
Deprecated `positionBalance` is not used. Isolated allocation and portfolio
position margin remain unknown; legacy positionIM is never relabeled allocation.
