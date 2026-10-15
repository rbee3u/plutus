# Binance adapter mapping

Package `internal/platform/binance`. Binance has two account structures and no
margin mode. The user selects the structure on the identity following the
[account model](../../adr/0001-ownership-and-account-boundaries.md):

- **Classic trading** (`binance_classic`): Spot, funding, cross margin, isolated
  margin, USDⓈ-M futures, COIN-M futures, earn, and dual investment are separate
  account types.
- **Portfolio margin** (`binance_pm`): cross margin, USDⓈ-M futures, and COIN-M
  futures merge into one `portfolio_margin` account that Binance values itself.
  Spot, funding, earn, and dual investment stay separate.

Portfolio Margin Pro is for institutions only and is not offered. Every call is
a read-only account, balance, position, earn position, or market-data endpoint.

Current test coverage, reviewed responses, real gaps, and pending exchange-app
checks are in [Binance verification](./verification.md).

## Endpoints

`USER_DATA` endpoints are signed and need only the key's "Enable Reading"
permission. Weights are Binance's documented request weights per call.

| Use                                                                                               | Endpoint                                                                                                                                 | Security    | Weight       |
| ------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- | ----------- | ------------ |
| Clock sync                                                                                        | `GET api /api/v3/time`                                                                                                                   | `NONE`      | 1            |
| Account structure, both calls                                                                     | `GET api /sapi/v1/account/info`                                                                                                          | `USER_DATA` | 1            |
| Validate: UID                                                                                     | `GET api /api/v3/account?omitZeroBalances=true`                                                                                          | `USER_DATA` | 20           |
| Prices (all symbols, fetched right after the structure check so their time precedes every report) | `GET api /api/v3/ticker/price`                                                                                                           | `NONE`      | 4            |
| `spot`                                                                                            | `POST api /sapi/v3/asset/getUserAsset`                                                                                                   | `USER_DATA` | 5            |
| `funding`                                                                                         | `POST api /sapi/v1/asset/get-funding-asset`                                                                                              | `USER_DATA` | 1            |
| `earn`: Simple Earn, paged by 100                                                                 | `GET api /sapi/v1/simple-earn/flexible/position`, `/sapi/v1/simple-earn/locked/position`                                                 | `USER_DATA` | 150 per page |
| `dual_investment`: dual investment, one query per running state, paged by 100                     | `GET api /sapi/v1/dci/product/positions?status=…`                                                                                        | `USER_DATA` | 1 per page   |
| `earn`: completeness check                                                                        | `GET api /sapi/v1/asset/wallet/balance?quoteAsset=USDT`                                                                                  | `USER_DATA` | 60           |
| `portfolio_margin`                                                                                | `GET papi /papi/v1/account`, `/papi/v1/balance`, `/papi/v1/um/positionRisk`, `/papi/v1/cm/positionRisk`                                  | `USER_DATA` | 20, 20, 5, 1 |
| `cross_margin`                                                                                    | `GET api /sapi/v1/margin/account`                                                                                                        | `USER_DATA` | 10           |
| `isolated_margin`                                                                                 | `GET api /sapi/v1/margin/isolated/account`                                                                                               | `USER_DATA` | 10           |
| `um_futures`                                                                                      | `GET fapi /fapi/v1/multiAssetsMargin`, `/fapi/v3/account`, `/fapi/v3/positionRisk`; `/fapi/v1/symbolConfig` only when a position is open | `USER_DATA` | 30, 5, 5, 5  |
| `cm_futures`                                                                                      | `GET dapi /dapi/v1/account`, `/dapi/v1/positionRisk`                                                                                     | `USER_DATA` | 5, 1         |
| Futures contracts, fetched only for open positions and cached in process for one hour             | `GET fapi /fapi/v1/exchangeInfo`, `GET dapi /dapi/v1/exchangeInfo`                                                                       | `NONE`      | 1 each       |

`/sapi/v1/portfolio/account` is no longer called.

Signed requests use HMAC-SHA256 hex signatures for ordinary secrets or Ed25519
Base64 signatures for an unencrypted PKCS#8 private key PEM
([Credential decision](../../adr/0008-authentication-and-credentials.md)).
Ed25519 signs the same encoded parameters and percent-encodes the Base64
signature. Both use `recvWindow = 10000` and a timestamp corrected by the
measured offset to Binance server time. The offset is measured before the first
signed call and again after a `-1021` answer, which is then retried once.
Binance's
[Spot API key documentation](https://github.com/binance/binance-spot-api-docs/blob/master/testnet/general-info.md#how-can-i-use-ed25519-api-keys)
specifies the Ed25519 signing flow. Its application to all the adapter's SAPI,
PAPI, FAPI, and DAPI calls has not been live verified.

## Account structure

Validation and every collection run start with `/sapi/v1/account/info`
([account model](../../adr/0001-ownership-and-account-boundaries.md)). The user
can switch between the two structures inside Binance at any time.

| `/sapi/v1/account/info`                       | Result                                                                                                                                            |
| --------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| `isPortfolioMarginRetailEnabled: true`        | Portfolio margin                                                                                                                                  |
| `isPortfolioMarginRetailEnabled: false`       | Classic trading                                                                                                                                   |
| Field missing                                 | `normalization`                                                                                                                                   |
| A structure other than the identity's         | `unsupported`, naming both structures, before any account is read. The user creates a new identity for the new structure and archives the old one |
| Classic trading with `isMarginEnabled: false` | `cross_margin` and `isolated_margin` succeed with empty reports without calling their endpoints                                                   |
| Classic trading with `isFutureEnabled: false` | `um_futures` and `cm_futures` succeed with empty reports without calling their endpoints                                                          |

An absent `isMarginEnabled` or `isFutureEnabled` counts as opened, so the wallet
is read. A structure the adapter does not know, such as another platform's,
fails with `unsupported` before any call. So does an account type the structure
does not hold, which fails only its own account. `ExternalRef` is the account
UID from `/api/v3/account` in both structures.

## Account report

Only `portfolio_margin` reports a USD equity. Every other account keeps
`reported_equity_usd` null and is valued from its balances.

| Stored fact or read projection        | `portfolio_margin`                                                                | `um_futures`, Multi-Assets Mode                                                     | `cross_margin`                                                                                                                                                      |
| ------------------------------------- | --------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `reported_equity_usd`                 | `actualEquity` (USD, no collateral haircut)                                       | `null`                                                                              | `null`                                                                                                                                                              |
| `adjusted_equity_usd`                 | `accountEquity` (USD, after collateral rates)                                     | `totalMarginBalance`                                                                | `null`                                                                                                                                                              |
| `available_equity_usd`                | `totalAvailableBalance`                                                           | `availableBalance`                                                                  | `null`                                                                                                                                                              |
| Initial margin ratio (projection)     | `accountInitialMargin ÷ accountEquity`, 8 decimals; null when `accountEquity ≤ 0` | `totalInitialMargin ÷ totalMarginBalance`; null when `totalMarginBalance ≤ 0`       | `null`                                                                                                                                                              |
| Maintenance margin ratio (projection) | `accountMaintMargin ÷ accountEquity`, same rule                                   | `totalMaintMargin ÷ totalMarginBalance`, the margin ratio of Binance's futures page | `null`                                                                                                                                                              |
| `reported_margin_ratio`               | `uniMMR` as reported                                                              | `null`                                                                              | `marginLevel` as reported: total asset value ÷ (borrowed + interest), higher is safer. `999` means nothing is borrowed and is kept as reported, as the app shows it |
| `collected_at`                        | Local time once the account's responses arrived                                   | Same                                                                                | Same                                                                                                                                                                |
| `source_observed_at`                  | `null`                                                                            | `null`                                                                              | `null`                                                                                                                                                              |

- `spot`, `funding`, `earn`, `isolated_margin`, and `cm_futures` report no
  account figures. Isolated margin's `marginLevel` belongs to one pair, not the
  account. COIN-M margins each coin on its own.
- In Single-Asset Mode (`multiAssetsMargin: false`) every USDⓈ-M margin asset
  stands alone. `/fapi/v3/account` totals then cover USDT only, so none is
  stored.
- A cross margin account with `created: false` succeeds with an empty report.

Not stored: PM `totalMarginOpenLoss`, `virtualMaxWithdrawAmount`,
`accountStatus`, `updateTime`; cross margin `totalAssetOfBtc`,
`totalLiabilityOfBtc`, `totalNetAssetOfBtc`, `tradeEnabled`; USDⓈ-M
`totalWalletBalance`, `totalUnrealizedProfit`, `maxWithdrawAmount`.

Raw initial and maintenance requirements (`accountInitialMargin` /
`accountMaintMargin` for portfolio margin and `totalInitialMargin` /
`totalMaintMargin` for USDⓈ-M Multi-Assets Mode) are stored in
`initial_margin_requirement_usd` and `maintenance_margin_requirement_usd`. The
read projection derives ratios from these facts and positive
`adjusted_equity_usd`; derived ratios are not persisted as reported ratios.
Missing or nonpositive denominators produce null.

## Balance

Binance reports no full coin names on these endpoints, so `name` stays empty. A
row with a nonzero amount but no `asset` fails its account with `normalization`
and is never dropped. Rows whose amounts are all zero are omitted, because
Binance keeps coins that were once held.

| Plutus column           | `portfolio_margin` (`/papi/v1/balance`)                                                                               | `spot`, `funding`                      | `cross_margin`, `isolated_margin`                                   | `um_futures`, `cm_futures` (`assets`)                                 | `earn`                                |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------- | -------------------------------------- | ------------------------------------------------------------------- | --------------------------------------------------------------------- | ------------------------------------- |
| `AssetSymbol`           | `asset`                                                                                                               | `asset`                                | `asset`                                                             | `asset`                                                               | `asset`, dual investment `investCoin` |
| `equity` (coin units)   | `totalWalletBalance + umUnrealizedPNL + cmUnrealizedPNL − crossMarginBorrowed − crossMarginInterest`, may be negative | `free + locked + freeze + withdrawing` | `netAsset` (`free + locked − borrowed − interest`), may be negative | `marginBalance` (`walletBalance + unrealizedProfit`), may be negative | See [Earn](#earn)                     |
| `liability`             | `crossMarginBorrowed`                                                                                                 | `null`                                 | `borrowed`                                                          | `null`                                                                | `null`                                |
| `interest`              | `crossMarginInterest`                                                                                                 | `null`                                 | `interest`                                                          | `null`                                                                | `null`                                |
| `unrealized_pnl`        | `umUnrealizedPNL + cmUnrealizedPNL`                                                                                   | `null`                                 | `null`                                                              | `unrealizedProfit`                                                    | `null`                                |
| `available_to_trade`    | See position presentation provenance                                                                                  | `free`                                 | Not mapped                                                          | `availableBalance`                                                    | `null`                                |
| `available_to_withdraw` | See position presentation provenance                                                                                  | `free`                                 | Not mapped                                                          | `maxWithdrawAmount`                                                   | `null`                                |

- PM `totalWalletBalance` already equals
  `crossMarginFree + crossMarginLocked + umWalletBalance + cmWalletBalance`.
  `negativeBalance` is the negative part already inside it and is not subtracted
  again.
- Isolated margin holds every pair apart. The account sums the base and quote
  rows of all pairs per coin, so USDT held in BTCUSDT and ETHUSDT becomes one
  USDT balance.
- `getUserAsset` does not list Simple Earn flexible receipts (`LD*` assets such
  as `LDUSDT`), while `/api/v3/account` does. Using it keeps flexible positions
  only in `earn`, so selecting `spot` and `earn` never counts them twice. Real
  coins that start with `LD`, such as `LDO`, are kept.

## Earn

The `earn` account has one balance per coin: Simple Earn flexible `totalAmount`
plus locked `amount`. Dual principal is held in the separate `dual_investment`
account, summing `subscriptionAmount` in `investCoin` until the subscription
ends. Expected yield is excluded.

| `purchaseStatus`                                                 | Read                                                                             |
| ---------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| `PENDING`, `PURCHASE_SUCCESS`, `SETTLING`, `REFUNDING` (running) | Queried one by one with `status=<state>`, every page, and counted                |
| `SETTLED`, `PURCHASE_FAIL`, `REFUND_SUCCESS` (finished)          | Never queried: the principal or payout is back in Spot, or was never taken       |
| An undocumented status, or any status other than the one queried | The selected earn and dual accounts fail with `normalization`, naming the status |

Querying each running state keeps the read bounded by the running positions,
however long the account's history is. A position that moves between running
states while they are read, such as from `PURCHASE_SUCCESS` to `SETTLING`, is
counted once by `id`.

**Completeness check.** Binance keeps adding earn products, and one the adapter
does not read would otherwise vanish from a successful report. So the adapter
also reads Binance's own USDT valuation of its `Earn` wallet from
`/sapi/v1/asset/wallet/balance` and compares it with the combined ordinary earn
and dual balances, valued with the batch's last prices. If Binance's value
exceeds the itemized value by more than max(1%, 1 USD), the selected earn and
dual accounts fail with `unsupported`, naming both amounts, and go stale
([report policy](../../adr/0002-reports-and-compatible-history.md)). A missing
`Earn` wallet is a `normalization` failure. A balance without a price skips the
check, because valuation already marks it `no_price`.

Any other product Binance values in its earn wallet trips the check. Liquid
staking tokens such as WBETH and BNSOL are ordinary spot coins.

**Settlement window.** Dual investment settles at 08:00 UTC on the settlement
date. Binance pays out to Spot within six hours, by 14:00 UTC, and the position
is `SETTLING` in between. A collection run in that window can count the
principal in `dual_investment` and the payout in `spot` at once, a double count,
or neither, a dip, depending on which side has updated. The next run after the
payout is correct again. Both are expected and are not reconciled across
accounts.

### Earn position descriptions

Flexible rows preserve product aggregates, `totalAmount`, source APR and tier
rules, and explicitly cumulative rewards. Locked rows preserve each position
reference, principal `amount`, subscription time, source APY, and native status.
`rewardsEndDate` supplies Maturity; `deliverDate` is a separate delivery detail.
This date mapping is an inference from the
[API fields](https://developers.binance.com/en/docs/catalog/investment-and-services-simple-earn/api/rest-api/flexible-locked)
and the locked-product help's
[Interest end date](https://www.binance.com/en/support/faq/detail/af607a50de614350ae57be3e6f616699).
The API APY label and Help APR explanation are retained without compounding
conversion. Unqualified `rewardAmt` stays in details as reported rewards. These
descriptions never add value to the balance totals or change shared
completeness.

## Dual investment account and positions

Dual account selection, principal valuation, and shared earn completeness follow
[domain rules](../../domain-rules.md#dual-investment-and-earn-completeness).
Platform-specific fields map as follows:

| Dual position field   | Source                                                                                            |
| --------------------- | ------------------------------------------------------------------------------------------------- |
| Subscription identity | `id` (position ID); `orderId` retained separately                                                 |
| Principal             | `subscriptionAmount` of `investCoin`                                                              |
| Pair and direction    | PUT: `exercisedCoin` / `investCoin`, `Buy low`; CALL: `investCoin` / `exercisedCoin`, `Sell high` |
| Target, expiry        | `strikePrice`, `settleDate` milliseconds                                                          |
| Annualized rate       | `apr`, tagged APR                                                                                 |

The single-interface recorder can explicitly read
`GET /sapi/v1/dci/product/accounts` (`USER_DATA`, weight 1). Its
`totalAmountInUSDT` is diagnostic only: the valuation formula is undocumented
and never overrides principal or creates a runtime dependency.

## Position

Positions come from `/papi/v1/um/positionRisk` and `/papi/v1/cm/positionRisk` in
portfolio margin, and from `/fapi/v3/positionRisk` and `/dapi/v1/positionRisk`
in classic trading.

| Plutus column                                 | Source                                                                                                                                                                                                   |
| --------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `position_key`                                | `um:<symbol>:<positionSide>` or `cm:<symbol>:<positionSide>`                                                                                                                                             |
| `side`                                        | `positionSide` `LONG` / `SHORT`; for one-way mode `BOTH`, the sign of `positionAmt`. Zero positions are omitted                                                                                          |
| `source_quantity`, `source_quantity_unit`     | `\|positionAmt\|`; USDⓈ-M in `base`, COIN-M in `contracts`                                                                                                                                               |
| `entry_price`, `mark_price`, `unrealized_pnl` | `entryPrice`, `markPrice`, `unRealizedProfit`                                                                                                                                                            |
| `leverage`                                    | `leverage`; `/fapi/v3/positionRisk` has none, so classic USDⓈ-M reads `leverage` from `/fapi/v1/symbolConfig`, once per run and only with an open position. A symbol missing there keeps a null leverage |
| `liquidation_price`                           | `liquidationPrice`; null when `0`                                                                                                                                                                        |
| `margin`                                      | See position presentation provenance                                                                                                                                                                     |

| Futures contract column                                              | Source (`exchangeInfo`)                                                                                                                                                |
| -------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `kind`                                                               | USDⓈ-M: `PERPETUAL`, `TRADIFI_PERPETUAL`, `PERPETUAL_DELIVERING` → `linear_perpetual`; `*_QUARTER`, `*_MONTH` → `linear_delivery`. COIN-M: the `inverse_*` equivalents |
| `base_asset_symbol`, `quote_asset_symbol`, `settlement_asset_symbol` | `baseAsset`, `quoteAsset`, `marginAsset` (Binance's name for the settlement coin)                                                                                      |
| `expiry_at`                                                          | `deliveryDate`, delivery contracts only                                                                                                                                |
| `contract_size`                                                      | COIN-M `contractSize` (USD per contract); null for USDⓈ-M, whose quantity is already in base units                                                                     |

A position whose symbol is missing from a fresh `exchangeInfo` fails its account
with `normalization`, naming the symbol. It is never dropped. Contract symbols
may use non-Latin scripts (`牛来USDT`).

## Prices

For every non-USD balance symbol, the price is the spot last price of
`<symbol>USDT`, else `<symbol>USDC`, observed when the ticker response arrived.
Binance keeps delisted pairs in the ticker at price `0`; such a pair is ignored,
never used as a zero price. USD, USDT, and USDC get no price row
([valuation policy](../../adr/0003-valuation-and-asset-policy.md)). Other
stablecoins such as FDUSD are priced through their pairs. A symbol with neither
pair gets no price and is valued as `no_price`. A failed ticker call leaves the
batch without prices. Valuation then falls back to the latest price within 24
hours, and the earn check runs only when every earn coin is a USD equivalent.
Last price differs from the index price Binance uses for `actualEquity` by a few
hundredths of a percent. Index or mark prices are a later refinement.

## Errors

| Binance answer                                                                                                            | Class                                                  | Scope                                             |
| ------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------ | ------------------------------------------------- |
| `-2015` (key, IP whitelist, or permission), `-2014` (malformed key), `-2008` (unknown key), `-1022` (signature), HTTP 401 | `authentication`                                       | The whole batch; the connection becomes `invalid` |
| HTTP 429, 418, 403 (WAF), `-1003`                                                                                         | `rate_limited`                                         | The account; the batch when every account failed  |
| HTTP 5xx, timeouts, network errors, `-1001`, `-1021` after one resync                                                     | `temporary`                                            | The account; the batch when every account failed  |
| Any other 4xx                                                                                                             | `temporary`, detail carries Binance's code and message | The account                                       |
| A different account structure                                                                                             | `unsupported`                                          | The whole call, before any account is read        |
| Earn valued above its itemized balances                                                                                   | `unsupported`, naming both amounts                     | Selected `earn` and `dual_investment` accounts    |
| No `isPortfolioMarginRetailEnabled`                                                                                       | `normalization`                                        | The whole call                                    |
| Unparseable or non-decimal fields, an amount without a coin, an undocumented dual investment status, unknown contracts    | `normalization`                                        | The account                                       |

Error details name the endpoint and Binance's code and message, never a body or
a key.

## Known gaps

- `/sapi/v1/asset/wallet/balance` also lists `Options`, `Trading Bots`, and
  `Copy Trading` wallets. No account type collects them, and the earn check
  covers only the earn wallet, so value held there is not reported and nothing
  flags it.

### Position presentation provenance

`positionSide=BOTH` retains one-way mode; LONG/SHORT retains hedged mode.
Classic USDⓈ-M reads the already requested symbol config for `marginType`; cross
uses `positionInitialMargin`, while isolated uses `isolatedWallet`, excluding
PnL. Validate the `marginAsset` currency before settlement display. COIN-M joins
an unambiguous account position by exact symbol, positionSide, and quantity for
its initial requirement; optional `isolatedWallet` is used only when returned.
Missing or contradictory joins leave the source requirement unknown. Portfolio
margin risk responses supply no per-position allocation, but do retain futures
initial leverage. Supported retail PM futures are cross. No new requests are
added.

The read model prefers verified source initial amounts. For cross with no usable
source, including retail PM, it calculates the documented futures initial
requirement: linear absolute base size times mark divided by leverage; inverse
quote face size divided by mark and leverage in base settle. Specifications and
currencies must be compatible, with positive mark and initial leverage. The API
marks this display as calculated; the stored margin is not filled or changed.
This does not allocate shared collateral or PM risk. Old PM reports can use
their retained position facts; classic rows without a known margin setting stay
unknown. See
[Binance's PM overview](https://www.binance.com/en/support/faq/detail/174861644e7049aaa64845f1db2e4376).

COIN-M account and position-risk reads are not atomic. A mismatched quantity or
ambiguous symbol/side join leaves the account-derived requirement unknown. An
actually returned `isolatedWallet` may come from the risk row or the exact,
quantity-compatible account position; no isolated amount is synthesized.
