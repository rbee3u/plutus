# Bybit verification

This document owns current coverage, evidence review observations, remaining
real gaps, and pending exchange-app checks. See the
[Adapter mapping](./mapping.md) for field semantics and the
[evidence workflow](../evidence-workflow.md) for recording and promotion rules.
Synthetic tests, recorded responses, and exchange-app validation are distinct.

## Current evidence

**19 reviewed responses**, recorded and owner-reviewed on 2026-10-01. They cover
PM One-way, Cross One-way/Hedge, and shared Funding, Flexible Saving, BuyLow,
overview, and market interfaces. Configuration directly observes UTA 2.0 Pro
(`unifiedMarginStatus=6`) with `PORTFOLIO_MARGIN` and `REGULAR_MARGIN`.
Credential metadata directly observes read-only, master-account, and Earn flags;
masked identities do not independently prove ownership.

Nonempty linear USDT/USDC and inverse perpetual/delivery contracts are present.
PM positions have `positionIdx=0`; Cross includes HYPEUSDT sides 1/2 alongside
One-way contracts. Cross has populated leverage and margin and both blank and
populated liquidation fields. Negative equity, borrowing, and accrued interest
remain intact; the Adapter uses reported per-coin equity.

## Important observations

- PM `riskLimitValue` is blank for linear positions and `200`/`3000` for inverse
  positions, unlike documented zero; the field is not mapped.
- Each PM and Cross position response has a nonempty percent-escaped
  `nextPageCursor` despite few rows. No continuation was recorded and the
  samples do not prove complete position enumeration.
- Cross retains `spotHedgingStatus=ON` with zero hedging quantities; the flag
  alone does not establish effective Cross spot hedging. SOLPERP metadata uses
  display symbol SOLUSDC.
- A zero-principal HYPE Savings row remains because historical PnL is nonzero,
  not because it represents an active subscription.

## Sanitation and replacements

Funding `memberId` initially escaped redaction. The sanitizer was fixed with an
independent synthetic regression; the pre-promotion response masks that field
and discloses the repair while preserving business values and `recordedAt`.

One accepted linear metadata response was replaced to add HYPEUSDT:
`2026-10-01T05:05:23.048246Z` (six of 890 rows) became
`2026-10-01T05:31:35.116165Z` (seven of 890 rows). The new whole response keeps
all previously retained symbols; no rows were spliced from the old capture. All
accepted responses are frozen under the
[replacement rules](../evidence-workflow.md#trim-save-and-review).

## Remaining gaps

- Isolated configuration, wallet, and positions are owner-deferred because the
  mode is rarely used. Its implementation and independent synthetic coverage
  remain; no real observation is claimed.
- Nonempty Fixed-Term Saving is owner-deferred because minimum principal exceeds
  preparation funds. SellHigh is deferred under principal/product requirements.
- Unobserved transient, refusal, and pagination-boundary shapes remain gaps.
  Captured cursors are not evidence of following additional pages.
- Exchange-app comparison and complete asset-total reconciliation remain
  pending.

Deferred preparation resumes only on a later owner request; there is no current
obligation to change account settings or fund another product.

## Interface references

The following independent synthetic inputs test deliberate request, field,
mapping, pagination, and failure scenarios. Recorded links identify reviewed
responses and their observed limits. Actual per-file UTC timestamps and trimming
notes are retained in each recorded envelope. Generic interfaces are not limited
to the structure used to sample them.

### `v5: GET /v5/account/info`

[Contract](https://bybit-exchange.github.io/docs/v5/account/account-info)

Synthetic inputs:

- [v5_account_info.bad_signature.json](../../../internal/adapters/bybit/testdata/v5_account_info.bad_signature.json)
- [v5_account_info.cross.json](../../../internal/adapters/bybit/testdata/v5_account_info.cross.json)
- [v5_account_info.invalid_key.json](../../../internal/adapters/bybit/testdata/v5_account_info.invalid_key.json)
- [v5_account_info.ip_rate_limited.json](../../../internal/adapters/bybit/testdata/v5_account_info.ip_rate_limited.json)
- [v5_account_info.isolated.json](../../../internal/adapters/bybit/testdata/v5_account_info.isolated.json)
- [v5_account_info.pm.json](../../../internal/adapters/bybit/testdata/v5_account_info.pm.json)
- [v5_account_info.region_restricted.json](../../../internal/adapters/bybit/testdata/v5_account_info.region_restricted.json)
- [v5_account_info.stale_timestamp.json](../../../internal/adapters/bybit/testdata/v5_account_info.stale_timestamp.json)
- [v5_account_info.uta_1_0_pro.json](../../../internal/adapters/bybit/testdata/v5_account_info.uta_1_0_pro.json)

Recorded evidence:

[PM](../../../internal/adapters/bybit/evidence/v5_account_info.pm.json),
[Cross](../../../internal/adapters/bybit/evidence/v5_account_info.cross.json);
directly observed UTA 2.0 Pro and the corresponding Margin Mode; both report
Spot Hedging ON

### `v5: GET /v5/account/wallet-balance`

[Contract](https://bybit-exchange.github.io/docs/v5/account/wallet-balance)

Synthetic inputs:

- [v5_account_wallet_balance.cross.json](../../../internal/adapters/bybit/testdata/v5_account_wallet_balance.cross.json)
- [v5_account_wallet_balance.isolated.json](../../../internal/adapters/bybit/testdata/v5_account_wallet_balance.isolated.json)
- [v5_account_wallet_balance.pm.json](../../../internal/adapters/bybit/testdata/v5_account_wallet_balance.pm.json)
- [v5_account_wallet_balance.rate_limited.json](../../../internal/adapters/bybit/testdata/v5_account_wallet_balance.rate_limited.json)

Recorded evidence:

[PM](../../../internal/adapters/bybit/evidence/v5_account_wallet_balance.pm.json),
[Cross](../../../internal/adapters/bybit/evidence/v5_account_wallet_balance.cross.json);
nonzero HYPE borrowing/interest and negative equity; PM SOL spotHedgingQty
nonzero, Cross quantities zero

### `v5: GET /v5/asset/asset-overview`

[Contract](https://bybit-exchange.github.io/docs/v5/asset/balance/asset-overview)

Synthetic inputs:

- [v5_asset_asset_overview.dual_only.json](../../../internal/adapters/bybit/testdata/v5_asset_asset_overview.dual_only.json)
- [v5_asset_asset_overview.fixed_saving.json](../../../internal/adapters/bybit/testdata/v5_asset_asset_overview.fixed_saving.json)
- [v5_asset_asset_overview.json](../../../internal/adapters/bybit/testdata/v5_asset_asset_overview.json)

Recorded evidence:

[v5_asset_asset_overview.json](../../../internal/adapters/bybit/evidence/v5_asset_asset_overview.json);
Funding, Unified and Earn with EasyEarn/DualAsset entries; independent interface
totals

### `v5: GET /v5/asset/transfer/query-account-coins-balance`

[Contract](https://bybit-exchange.github.io/docs/v5/asset/balance/all-balance)

Synthetic inputs:

- [v5_asset_transfer_query_account_coins_balance.json](../../../internal/adapters/bybit/testdata/v5_asset_transfer_query_account_coins_balance.json)
- [v5_asset_transfer_query_account_coins_balance.rate_limited.json](../../../internal/adapters/bybit/testdata/v5_asset_transfer_query_account_coins_balance.rate_limited.json)

Recorded evidence:

[v5_asset_transfer_query_account_coins_balance.json](../../../internal/adapters/bybit/evidence/v5_asset_transfer_query_account_coins_balance.json);
nonzero USDT/USDC Funding; memberId masked, including the disclosed local
redaction repair

### `v5: GET /v5/earn/advance/position`

[Contract](https://bybit-exchange.github.io/docs/v5/finance/advanced-earn/dual-asset/position)

Synthetic inputs:

- [v5_earn_advance_position.buy_low.json](../../../internal/adapters/bybit/testdata/v5_earn_advance_position.buy_low.json)
- [v5_earn_advance_position.empty.json](../../../internal/adapters/bybit/testdata/v5_earn_advance_position.empty.json)
- [v5_earn_advance_position.sell_high.json](../../../internal/adapters/bybit/testdata/v5_earn_advance_position.sell_high.json)

Recorded evidence:

[v5_earn_advance_position.json](../../../internal/adapters/bybit/evidence/v5_earn_advance_position.json);
Active HYPE BuyLow, USDT principal 20; SellHigh deferred

### `v5: GET /v5/earn/fixed-term/position`

[Contract](https://bybit-exchange.github.io/docs/v5/finance/earn/fixed-saving/position)

Synthetic inputs:

- [v5_earn_fixed_term_position.empty.json](../../../internal/adapters/bybit/testdata/v5_earn_fixed_term_position.empty.json)
- [v5_earn_fixed_term_position.json](../../../internal/adapters/bybit/testdata/v5_earn_fixed_term_position.json)

Recorded evidence:

Missing nonempty evidence; owner cannot currently meet minimum principal. Empty
diagnostic remains in tmp

### `v5: GET /v5/earn/position`

[Contract](https://bybit-exchange.github.io/docs/v5/finance/earn/easy-onchain/position)

Synthetic inputs:

- [v5_earn_position.json](../../../internal/adapters/bybit/testdata/v5_earn_position.json)

Recorded evidence:

[v5_earn_position.json](../../../internal/adapters/bybit/evidence/v5_earn_position.json);
USDT Flexible principal 20; zero-principal HYPE row with historical PnL retained

### `v5: GET /v5/market/instruments-info`

[Contract](https://bybit-exchange.github.io/docs/v5/market/instrument)

Synthetic inputs:

- [v5_market_instruments_info.inverse.json](../../../internal/adapters/bybit/testdata/v5_market_instruments_info.inverse.json)
- [v5_market_instruments_info.linear.json](../../../internal/adapters/bybit/testdata/v5_market_instruments_info.linear.json)

Recorded evidence:

[linear](../../../internal/adapters/bybit/evidence/v5_market_instruments_info.linear.json),
[inverse](../../../internal/adapters/bybit/evidence/v5_market_instruments_info.inverse.json);
kept 7/890 and 4/26 rows, retaining every held contract; linear replaced for the
HYPEUSDT coverage gap

### `v5: GET /v5/market/tickers`

[Contract](https://bybit-exchange.github.io/docs/v5/market/tickers)

Synthetic inputs:

- [v5_market_tickers.json](../../../internal/adapters/bybit/testdata/v5_market_tickers.json)

Recorded evidence:

[v5_market_tickers.json](../../../internal/adapters/bybit/evidence/v5_market_tickers.json);
6/530 whole rows, including relevant ETH/SOL/HYPE/XRP price pairs

### `v5: GET /v5/market/time`

[Contract](https://bybit-exchange.github.io/docs/v5/market/time)

Synthetic inputs:

- [v5_market_time.json](../../../internal/adapters/bybit/testdata/v5_market_time.json)

Recorded evidence:

[v5_market_time.json](../../../internal/adapters/bybit/evidence/v5_market_time.json);
explicit public clock request, native time fields retained

### `v5: GET /v5/position/list`

[Contract](https://bybit-exchange.github.io/docs/v5/position)

Synthetic inputs:

- [v5_position_list.inverse_cross.json](../../../internal/adapters/bybit/testdata/v5_position_list.inverse_cross.json)
- [v5_position_list.inverse_isolated.json](../../../internal/adapters/bybit/testdata/v5_position_list.inverse_isolated.json)
- [v5_position_list.inverse_pm.json](../../../internal/adapters/bybit/testdata/v5_position_list.inverse_pm.json)
- [v5_position_list.linear_usdc_cross.json](../../../internal/adapters/bybit/testdata/v5_position_list.linear_usdc_cross.json)
- [v5_position_list.linear_usdc_isolated.json](../../../internal/adapters/bybit/testdata/v5_position_list.linear_usdc_isolated.json)
- [v5_position_list.linear_usdc_pm.json](../../../internal/adapters/bybit/testdata/v5_position_list.linear_usdc_pm.json)
- [v5_position_list.linear_usdt_cross.json](../../../internal/adapters/bybit/testdata/v5_position_list.linear_usdt_cross.json)
- [v5_position_list.linear_usdt_isolated.json](../../../internal/adapters/bybit/testdata/v5_position_list.linear_usdt_isolated.json)
- [v5_position_list.linear_usdt_pm.json](../../../internal/adapters/bybit/testdata/v5_position_list.linear_usdt_pm.json)
- [v5_position_list.option.json](../../../internal/adapters/bybit/testdata/v5_position_list.option.json)

Recorded evidence:

PM
[USDT](../../../internal/adapters/bybit/evidence/v5_position_list.linear_usdt_pm.json),
[USDC](../../../internal/adapters/bybit/evidence/v5_position_list.linear_usdc_pm.json),
[inverse](../../../internal/adapters/bybit/evidence/v5_position_list.inverse_pm.json);
Cross
[USDT](../../../internal/adapters/bybit/evidence/v5_position_list.linear_usdt_cross.json),
[USDC](../../../internal/adapters/bybit/evidence/v5_position_list.linear_usdc_cross.json),
[inverse](../../../internal/adapters/bybit/evidence/v5_position_list.inverse_cross.json);
four contract kinds, coexisting HYPEUSDT Hedge sides and One-way rows; literal
cursors preserved

### `v5: GET /v5/user/query-api`

[Contract](https://bybit-exchange.github.io/docs/v5/user/apikey-info)

Synthetic inputs:

- [v5_user_query_api.json](../../../internal/adapters/bybit/testdata/v5_user_query_api.json)

Recorded evidence:

[v5_user_query_api.json](../../../internal/adapters/bybit/evidence/v5_user_query_api.json);
readOnly=1, isMaster=true and Earn permission; identity/key/IP/label masked

## Exchange-app validation checklist

Status: pending. This is an acceptance procedure, not a record of completed
checks. Recorded evidence and synthetic tests do not establish a reconciled
Account total. Human operators execute it on the remote test environment; agent
sessions remain limited to the isolated development instance.

Run these on the test environment against the Bybit app on the same account,
within a few minutes of a collection. Items 5 and 6 need an Isolated or Cross
Margin account. Item 8 includes Dual Asset app comparison; available evidence
and preparation deferrals are listed above.

1. **Identity and structure**: validation stores the UID shown under the app's
   profile. The Identity's Margin Mode matches the margin mode shown for the
   Unified Trading Account.
2. **Margin Mode change**: after switching the margin mode in the app, the next
   Collection Run succeeds and the Identity shows the new Margin Mode.
3. **Portfolio Margin equity**: `Equity` equals the Unified Trading Account's
   total equity in USD, `available_equity` its available balance, and the two
   margin ratios the IM and MM rates the app shows, possibly as percentages.
4. **Unified balances**: each coin's equity matches the app's per-coin equity,
   including unrealized PnL and net of spot borrowing. `liability` matches the
   borrowed amount.
5. **Cross Margin**: the account figures appear as in item 3. Positions show
   leverage and margin, and Bybit's estimated liquidation price when provided;
   empty liquidation fields remain null.
6. **Isolated Margin**: the account figures are empty, and the value is
   calculated from the Balances. Each position shows its own liquidation price,
   leverage, and margin.
7. **Positions**: every open linear and inverse position appears with the same
   side, size (linear in coins, inverse in USD), entry price, and mark price.
   Portfolio Margin positions show no leverage, margin, or liquidation price.
8. **Funding and Earn**: Funding coin amounts and transferable amounts match the
   Funding account. Flexible and Fixed-Term principal per coin matches the Earn
   page, including an early redemption in progress. Each open Dual Asset
   position appears at its invested amount in the invested coin until Bybit pays
   it out. The Earn Account succeeds. Holding an Earn product Plutus does not
   read, such as On-chain Earn, makes it Stale with both USD amounts.
9. **Account values**: the calculated USD values of Funding and Earn are within
   about 0.1% of the app's asset overview.
10. **Failures**: removing the server IP from the key's whitelist sets the
    Connection `invalid` with a `10010` detail at the next cycle. Restoring it
    and validating again resumes collection.
