# Bybit verification

Current evidence and pending exchange-app checks. See the
[adapter mapping](./mapping.md) for field semantics,
[evidence workflow](../evidence-workflow.md) for recording and promotion, and
[earn delivery report](../earn-presentation-delivery.md) for metadata coverage.

## Current evidence

**19 reviewed responses**, recorded and owner-reviewed on 2026-10-01. They cover
PM One-way, cross One-way/Hedge, and shared funding, Flexible Savings, BuyLow,
overview, and market interfaces. Configuration directly observes UTA 2.0 Pro
(`unifiedMarginStatus=6`) with `PORTFOLIO_MARGIN` and `REGULAR_MARGIN`.
Credential metadata directly observes read-only, master-account, and earn flags;
masked identities do not independently prove ownership.

Nonempty linear USDT/USDC and inverse perpetual/delivery contracts are present.
PM positions have `positionIdx=0`; cross includes HYPEUSDT sides 1/2 alongside
One-way contracts. Cross has populated leverage and margin and both blank and
populated liquidation fields. Negative equity, borrowing, and accrued interest
remain intact; the adapter uses reported per-coin equity.

## Important observations

- PM `riskLimitValue` is blank for linear positions and `200`/`3000` for inverse
  positions, unlike documented zero; the field is not mapped.
- Each PM and cross position response has a nonempty percent-escaped
  `nextPageCursor` despite few rows. No continuation was recorded and the
  samples do not prove complete position enumeration.
- Cross retains `spotHedgingStatus=ON` with zero hedging quantities; the flag
  alone does not establish effective cross spot hedging. SOLPERP metadata uses
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
- Nonempty Fixed-Term Savings is owner-deferred because minimum principal
  exceeds preparation funds. SellHigh is deferred under principal/product
  requirements.
- Unobserved transient, refusal, and pagination-boundary shapes remain gaps.
  Captured cursors are not evidence of following additional pages.
- Exchange-app comparison and complete asset-total reconciliation remain
  pending.

Deferred preparation resumes only on a later owner request; there is no current
obligation to change account settings or fund another product.

## Interface references

Each interface links its contract, independent synthetic inputs, and reviewed
recordings. Coverage notes below qualify those samples; exact UTC capture times
and trimming details remain in each recording's `recordedAt` and `notes`. Link
labels name file variants; `sample` denotes an unsuffixed file. Sampling
settings do not limit an interface's applicability.

### [`v5: GET /v5/account/info`](https://bybit-exchange.github.io/docs/v5/account/account-info)

- Synthetic:
  [bad_signature](../../../internal/platform/bybit/testdata/v5_account_info.bad_signature.json),
  [cross](../../../internal/platform/bybit/testdata/v5_account_info.cross.json),
  [invalid_key](../../../internal/platform/bybit/testdata/v5_account_info.invalid_key.json),
  [ip_rate_limited](../../../internal/platform/bybit/testdata/v5_account_info.ip_rate_limited.json),
  [isolated](../../../internal/platform/bybit/testdata/v5_account_info.isolated.json),
  [pm](../../../internal/platform/bybit/testdata/v5_account_info.pm.json),
  [region_restricted](../../../internal/platform/bybit/testdata/v5_account_info.region_restricted.json),
  [stale_timestamp](../../../internal/platform/bybit/testdata/v5_account_info.stale_timestamp.json),
  [uta_1_0_pro](../../../internal/platform/bybit/testdata/v5_account_info.uta_1_0_pro.json).
- Recorded:
  [PM](../../../internal/platform/bybit/evidence/v5_account_info.pm.json),
  [Cross](../../../internal/platform/bybit/evidence/v5_account_info.cross.json);
  directly observed UTA 2.0 Pro and the corresponding margin mode; both report
  Spot Hedging ON

### [`v5: GET /v5/account/wallet-balance`](https://bybit-exchange.github.io/docs/v5/account/wallet-balance)

- Synthetic:
  [cross](../../../internal/platform/bybit/testdata/v5_account_wallet_balance.cross.json),
  [isolated](../../../internal/platform/bybit/testdata/v5_account_wallet_balance.isolated.json),
  [pm](../../../internal/platform/bybit/testdata/v5_account_wallet_balance.pm.json),
  [rate_limited](../../../internal/platform/bybit/testdata/v5_account_wallet_balance.rate_limited.json).
- Recorded:
  [PM](../../../internal/platform/bybit/evidence/v5_account_wallet_balance.pm.json),
  [Cross](../../../internal/platform/bybit/evidence/v5_account_wallet_balance.cross.json);
  nonzero HYPE borrowing/interest and negative equity; PM SOL spotHedgingQty
  nonzero, cross quantities zero

### [`v5: GET /v5/asset/asset-overview`](https://bybit-exchange.github.io/docs/v5/asset/balance/asset-overview)

- Synthetic:
  [dual_only](../../../internal/platform/bybit/testdata/v5_asset_asset_overview.dual_only.json),
  [fixed_saving](../../../internal/platform/bybit/testdata/v5_asset_asset_overview.fixed_saving.json),
  [sample](../../../internal/platform/bybit/testdata/v5_asset_asset_overview.json).
- Recorded:
  [sample](../../../internal/platform/bybit/evidence/v5_asset_asset_overview.json);
  funding, unified and earn with EasyEarn/DualAsset entries; independent
  interface totals

### [`v5: GET /v5/asset/transfer/query-account-coins-balance`](https://bybit-exchange.github.io/docs/v5/asset/balance/all-balance)

- Synthetic:
  [sample](../../../internal/platform/bybit/testdata/v5_asset_transfer_query_account_coins_balance.json),
  [rate_limited](../../../internal/platform/bybit/testdata/v5_asset_transfer_query_account_coins_balance.rate_limited.json).
- Recorded:
  [sample](../../../internal/platform/bybit/evidence/v5_asset_transfer_query_account_coins_balance.json);
  nonzero USDT/USDC funding; memberId masked, including the disclosed local
  redaction repair

### [`v5: GET /v5/earn/advance/position`](https://bybit-exchange.github.io/docs/v5/finance/advanced-earn/dual-asset/position)

- Synthetic:
  [buy_low](../../../internal/platform/bybit/testdata/v5_earn_advance_position.buy_low.json),
  [empty](../../../internal/platform/bybit/testdata/v5_earn_advance_position.empty.json),
  [sell_high](../../../internal/platform/bybit/testdata/v5_earn_advance_position.sell_high.json).
- Recorded:
  [sample](../../../internal/platform/bybit/evidence/v5_earn_advance_position.json);
  Active HYPE BuyLow, USDT principal 20; SellHigh deferred

### [`v5: GET /v5/earn/fixed-term/position`](https://bybit-exchange.github.io/docs/v5/finance/earn/fixed-saving/position)

- Synthetic:
  [empty](../../../internal/platform/bybit/testdata/v5_earn_fixed_term_position.empty.json),
  [sample](../../../internal/platform/bybit/testdata/v5_earn_fixed_term_position.json).
- Recorded: Missing nonempty evidence; owner cannot currently meet minimum
  principal. Empty diagnostic was not promoted.

### [`v5: GET /v5/earn/position`](https://bybit-exchange.github.io/docs/v5/finance/earn/easy-onchain/position)

- Synthetic:
  [sample](../../../internal/platform/bybit/testdata/v5_earn_position.json).
- Recorded:
  [sample](../../../internal/platform/bybit/evidence/v5_earn_position.json);
  USDT flexible principal 20; zero-principal HYPE row with historical PnL
  retained

### [`v5: GET /v5/market/instruments-info`](https://bybit-exchange.github.io/docs/v5/market/instrument)

- Synthetic:
  [inverse](../../../internal/platform/bybit/testdata/v5_market_instruments_info.inverse.json),
  [linear](../../../internal/platform/bybit/testdata/v5_market_instruments_info.linear.json).
- Recorded:
  [linear](../../../internal/platform/bybit/evidence/v5_market_instruments_info.linear.json),
  [inverse](../../../internal/platform/bybit/evidence/v5_market_instruments_info.inverse.json);
  kept 7/890 and 4/26 rows, retaining every held contract; linear replaced for
  the HYPEUSDT coverage gap

### [`v5: GET /v5/market/tickers`](https://bybit-exchange.github.io/docs/v5/market/tickers)

- Synthetic:
  [sample](../../../internal/platform/bybit/testdata/v5_market_tickers.json).
- Recorded:
  [sample](../../../internal/platform/bybit/evidence/v5_market_tickers.json);
  6/530 whole rows, including relevant ETH/SOL/HYPE/XRP price pairs

### [`v5: GET /v5/market/time`](https://bybit-exchange.github.io/docs/v5/market/time)

- Synthetic:
  [sample](../../../internal/platform/bybit/testdata/v5_market_time.json).
- Recorded:
  [sample](../../../internal/platform/bybit/evidence/v5_market_time.json);
  explicit public clock request, native time fields retained

### [`v5: GET /v5/position/list`](https://bybit-exchange.github.io/docs/v5/position)

- Synthetic:
  [inverse_cross](../../../internal/platform/bybit/testdata/v5_position_list.inverse_cross.json),
  [inverse_isolated](../../../internal/platform/bybit/testdata/v5_position_list.inverse_isolated.json),
  [inverse_pm](../../../internal/platform/bybit/testdata/v5_position_list.inverse_pm.json),
  [linear_usdc_cross](../../../internal/platform/bybit/testdata/v5_position_list.linear_usdc_cross.json),
  [linear_usdc_isolated](../../../internal/platform/bybit/testdata/v5_position_list.linear_usdc_isolated.json),
  [linear_usdc_pm](../../../internal/platform/bybit/testdata/v5_position_list.linear_usdc_pm.json),
  [linear_usdt_cross](../../../internal/platform/bybit/testdata/v5_position_list.linear_usdt_cross.json),
  [linear_usdt_isolated](../../../internal/platform/bybit/testdata/v5_position_list.linear_usdt_isolated.json),
  [linear_usdt_pm](../../../internal/platform/bybit/testdata/v5_position_list.linear_usdt_pm.json),
  [option](../../../internal/platform/bybit/testdata/v5_position_list.option.json).
- Recorded: PM
  [USDT](../../../internal/platform/bybit/evidence/v5_position_list.linear_usdt_pm.json),
  [USDC](../../../internal/platform/bybit/evidence/v5_position_list.linear_usdc_pm.json),
  [inverse](../../../internal/platform/bybit/evidence/v5_position_list.inverse_pm.json);
  cross
  [USDT](../../../internal/platform/bybit/evidence/v5_position_list.linear_usdt_cross.json),
  [USDC](../../../internal/platform/bybit/evidence/v5_position_list.linear_usdc_cross.json),
  [inverse](../../../internal/platform/bybit/evidence/v5_position_list.inverse_cross.json);
  four contract kinds, coexisting HYPEUSDT Hedge sides and One-way rows; literal
  cursors preserved

### [`v5: GET /v5/user/query-api`](https://bybit-exchange.github.io/docs/v5/user/apikey-info)

- Synthetic:
  [sample](../../../internal/platform/bybit/testdata/v5_user_query_api.json).
- Recorded:
  [sample](../../../internal/platform/bybit/evidence/v5_user_query_api.json);
  readOnly=1, isMaster=true and earn permission; identity/key/IP/label masked

## Exchange-app validation checklist

Status: pending. This is an acceptance procedure, not a record of completed
checks. Recorded evidence and synthetic tests do not establish a reconciled
account total. Human operators execute it on the remote test environment; agent
sessions remain limited to the isolated development instance.

Run these on the test environment against the Bybit app on the same account,
within a few minutes of a collection. Items 5 and 6 need an isolated or cross
margin account. Item 8 includes dual asset app comparison; available evidence
and preparation deferrals are listed above.

1. **Identity and structure**: validation stores the UID shown under the app's
   profile. The identity's margin mode matches the margin mode shown for the
   unified trading account.
2. **Margin mode change**: after switching the margin mode in the app, the next
   collection run succeeds and the identity shows the new margin mode.
3. **Portfolio margin equity**: `Equity` equals the unified trading account's
   total equity in USD, `available_equity` its available balance, and the two
   margin ratios the IM and MM rates the app shows, possibly as percentages.
4. **Unified balances**: each coin's equity matches the app's per-coin equity,
   including unrealized PnL and net of spot borrowing. `liability` matches the
   borrowed amount.
5. **Cross margin**: the account figures appear as in item 3. Positions show
   leverage and margin, and Bybit's estimated liquidation price when provided;
   empty liquidation fields remain null.
6. **Isolated margin**: the account figures are empty, and the value is
   calculated from the balances. Each position shows its own liquidation price,
   leverage, and an unknown allocated margin when only `positionIM` is returned.
   Its hover identifies the retained initial requirement.
7. **Positions**: every open linear and inverse position appears with the same
   side, size (linear in coins, inverse in USD), entry price, and mark price.
   Portfolio margin positions show no leverage, margin, or liquidation price.
8. **Funding and earn**: funding coin amounts and transferable amounts match the
   funding account. Flexible and fixed-term principal per coin matches the earn
   page, including an early redemption in progress. Each open dual asset
   position appears at its invested amount in the invested coin until Bybit pays
   it out. The earn account succeeds. Holding an earn product Plutus does not
   read, such as on-chain earn, makes it stale with both USD amounts.
9. **Account values**: the calculated USD values of funding and earn are within
   about 0.1% of the app's asset overview.
10. **Failures**: removing the server IP from the key's whitelist sets the
    connection `invalid` with a `10010` detail at the next cycle. Restoring it
    and validating again resumes collection.
