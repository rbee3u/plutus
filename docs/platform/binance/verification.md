# Binance verification

Current evidence and pending exchange-app checks. See the
[Adapter mapping](./mapping.md) for field semantics,
[evidence workflow](../evidence-workflow.md) for recording and promotion, and
[Earn delivery report](../earn-presentation-delivery.md) for metadata coverage.

## Current evidence

**34 reviewed responses**, recorded on 2026-09-30 UTC and owner-reviewed on
2026-10-01 Asia/Shanghai. They cover Portfolio Margin One-way/Hedge, shared
wallets and products, Classic Single-Asset One-way/Hedge, and Classic
Multi-Assets. These are separate interface observations, not an atomic Account
snapshot or a reconciled total.

The initial account flags directly confirm PM and Classic respectively; the
Classic mode response confirms Single-Asset. Later Classic Hedge structure and
Single-Asset settings are owner-declared, with earlier flag/mode captures as
separate observations. Same-contract isolated LONG/SHORT sides are directly
observed. Multi-Assets is directly confirmed by `multiAssetsMargin=true`; its
Classic structure is owner-declared and cross settings are consistent with zero
isolated fields, not separately verified through symbolConfig. PM Hedge
structure is owner-declared; nonempty PAPI LONG/SHORT rows directly establish
Hedge positions.

## Important observations

- PM/shared samples include borrowing and interest, UM/CM perpetuals, flexible
  and locked Savings, and both Dual directions. Classic One-way includes funded
  isolated pairs sharing USDT and cross/isolated perpetual/delivery positions.
  Multi-Assets includes USDT/ETH collateral and same-contract Hedge rows.
- Dual `orderId` is integer in recorded responses; compact synthetic examples
  use strings. Runtime accepts either. Real Cross Margin has
  `totalCollateralValueInUSDT`, unlike the uppercase
  `TotalCollateralValueInUSDT` in documentation and synthetic examples; the
  field is not mapped.
- UM V3 positionRisk omits marginType and leverage; symbolConfig supplies them.
  Multi-Assets account totals are USD measures, not sums of raw per-asset
  quantities. Observed asset lists extend beyond the abbreviated documented
  list. Zero wallet balance can coexist with positive available balance.
- PM Hedge preserves signed SHORT quantities and notionals, string leverage,
  reported liquidation prices including zero, and SHORT-before-LONG ordering.
  The observed order is not an API guarantee.

## Sanitation and replacements

UID masking and whole-row trimming are disclosed in evidence notes. Original
aggregates, retained business values, precision, and timestamps remain intact.
Market lists retain representative contracts; configuration responses were not
pruned by a new selection rule. Multi-Assets and PM Hedge responses retain all
returned business rows.

No accepted evidence was replaced. Four corrected Classic candidates closed
funded-isolated-pair and isolated-delivery preparation gaps before promotion;
the earlier candidates were never accepted evidence. Future accepted
replacements follow the
[shared workflow](../evidence-workflow.md#trim-save-and-review).

## Remaining gaps

- PM delivery-specific observations, Classic CM cross Hedge positions, and
  unobserved product/settings combinations remain unverified.
- Frozen representative UM metadata lacks the exact ETHUSDT and ETHUSDT_261225
  rows. Observed positions do not imply complete runtime metadata coverage;
  exact CM instruments are present.
- Difficult transient, pagination, and refusal states remain synthetic or
  documented coverage. Ed25519 signed live access remains unverified.
- The optional `/sapi/v1/dci/product/accounts` diagnostic was not requested.
  Metadata alone does not prove nonempty positions.
- Exchange-app comparison and complete asset-total reconciliation remain
  pending.

Unobserved settings require separately agreed preparation; do not rerecord
unrelated accepted interfaces to manufacture a test scene.

## Interface references

Each interface links its contract, independent synthetic inputs, and reviewed
recordings. Coverage notes below qualify those samples; exact UTC capture times
and trimming details remain in each recording's `recordedAt` and `notes`. Link
labels name file variants; `sample` denotes an unsuffixed file. Sampling
settings do not limit an interface's applicability.

### [`cm: GET /dapi/v1/account`](https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-coin-m-futures/api/rest-api/account#account-information)

- Synthetic:
  [sample](../../../internal/platform/binance/testdata/dapi_v1_account.json).
- Recorded:
  [one_way](../../../internal/platform/binance/evidence/dapi_v1_account.one_way.json)
  — Nonempty ETH wallet; inverse perpetual/delivery, cross/isolated; 5/81 rows
  retained.
  [hedge](../../../internal/platform/binance/evidence/dapi_v1_account.hedge.json)
  — Nonempty ETH wallet and simultaneous isolated ETHUSD_PERP LONG/SHORT; 8/111
  rows retained.

### [`cm: GET /dapi/v1/exchangeInfo`](https://developers.binance.com/en/docs/catalog/advanced-trading-derivatives-trading-coin-m-futures/api/rest-api/market-data)

- Synthetic:
  [sample](../../../internal/platform/binance/testdata/dapi_v1_exchangeInfo.json).
- Recorded:
  [sample](../../../internal/platform/binance/evidence/dapi_v1_exchangeInfo.json)
  — Seven of 30 representative/prepared markets retained; DOGE contract size 10;
  metadata is not position evidence.

### [`cm: GET /dapi/v1/positionRisk`](https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-coin-m-futures/api/rest-api/trade#position-information)

- Synthetic:
  [sample](../../../internal/platform/binance/testdata/dapi_v1_positionRisk.json).
- Recorded:
  [one_way](../../../internal/platform/binance/evidence/dapi_v1_positionRisk.one_way.json)
  — ETHUSD_PERP long/cross and ETHUSD_261225 short/isolated, BOTH; 3/30 rows
  retained.
  [hedge](../../../internal/platform/binance/evidence/dapi_v1_positionRisk.hedge.json)
  — ETHUSD_PERP LONG 1 / SHORT -1, isolated with separate nonzero wallets; 8/90
  rows retained, including representative zero BOTH/LONG/SHORT.

### [`portfolio: GET /papi/v1/account`](https://developers.binance.com/en/docs/catalog/advanced-trading-derivatives-trading-portfolio-margin/api/rest-api/account#account-information-user_data)

- Synthetic:
  [sample](../../../internal/platform/binance/testdata/papi_v1_account.json).
- Recorded:
  [sample](../../../internal/platform/binance/evidence/papi_v1_account.json) —
  PM account equity and risk measures.

### [`portfolio: GET /papi/v1/balance`](https://developers.binance.com/en/docs/catalog/advanced-trading-derivatives-trading-portfolio-margin/api/rest-api/account#account-balance-user_data)

- Synthetic:
  [sample](../../../internal/platform/binance/testdata/papi_v1_balance.json).
- Recorded:
  [sample](../../../internal/platform/binance/evidence/papi_v1_balance.json) —
  Nonzero borrowing and accrued interest; seven retained asset rows.

### [`portfolio: GET /papi/v1/cm/positionRisk`](https://developers.binance.com/en/docs/catalog/advanced-trading-derivatives-trading-portfolio-margin/api/rest-api/account#query-cm-position-information-user_data)

- Synthetic:
  [sample](../../../internal/platform/binance/testdata/papi_v1_cm_positionRisk.json).
- Recorded:
  [sample](../../../internal/platform/binance/evidence/papi_v1_cm_positionRisk.json)
  — One-way nonempty long: DOGEUSD_PERP.
  [hedge](../../../internal/platform/binance/evidence/papi_v1_cm_positionRisk.hedge.json)
  — ETHUSD_PERP LONG 1 / SHORT -1, string leverage 20, liquidationPrice
  2.82678988 on both sides; both nonempty rows retained without trimming.

### [`portfolio: GET /papi/v1/um/positionRisk`](https://developers.binance.com/en/docs/catalog/advanced-trading-derivatives-trading-portfolio-margin/api/rest-api/account#query-um-position-information-user_data)

- Synthetic:
  [sample](../../../internal/platform/binance/testdata/papi_v1_um_positionRisk.json).
- Recorded:
  [sample](../../../internal/platform/binance/evidence/papi_v1_um_positionRisk.json)
  — One-way nonempty shorts: 牛来USDT and ZECUSDT.
  [hedge](../../../internal/platform/binance/evidence/papi_v1_um_positionRisk.hedge.json)
  — ETHUSDT LONG 0.009 / SHORT -0.008, string leverage 5, liquidationPrice 0;
  both nonempty rows retained without trimming.

### [`spot: GET /api/v3/account`](https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md#account-information-user_data)

- Synthetic:
  [sample](../../../internal/platform/binance/testdata/api_v3_account.json).
- Recorded:
  [sample](../../../internal/platform/binance/evidence/api_v3_account.json) —
  Nonempty balances; UID masked.

### [`spot: GET /api/v3/ticker/price`](https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md#symbol-price-ticker)

- Synthetic:
  [sample](../../../internal/platform/binance/testdata/api_v3_ticker_price.json).
- Recorded:
  [sample](../../../internal/platform/binance/evidence/api_v3_ticker_price.json)
  — Nine of 3723 representative/prepared price rows retained.

### [`spot: GET /api/v3/time`](https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md#check-server-time)

- Synthetic:
  [sample](../../../internal/platform/binance/testdata/api_v3_time.json).
- Recorded:
  [sample](../../../internal/platform/binance/evidence/api_v3_time.json) —
  Public server-time response.

### [`spot: GET /sapi/v1/account/info`](https://developers.binance.com/docs/binance-spot-api-docs/errors)

- Synthetic:
  [bad_signature](../../../internal/platform/binance/testdata/sapi_v1_account_info.bad_signature.json),
  [classic](../../../internal/platform/binance/testdata/sapi_v1_account_info.classic.json),
  [invalid_key](../../../internal/platform/binance/testdata/sapi_v1_account_info.invalid_key.json),
  [pm](../../../internal/platform/binance/testdata/sapi_v1_account_info.pm.json),
  [stale_timestamp](../../../internal/platform/binance/testdata/sapi_v1_account_info.stale_timestamp.json).
- Recorded:
  [pm](../../../internal/platform/binance/evidence/sapi_v1_account_info.pm.json)
  — PM flag true; owner-declared PM main account.
  [classic](../../../internal/platform/binance/evidence/sapi_v1_account_info.classic.json)
  — Classic flag false; margin/futures enabled.

### [`spot: GET /sapi/v1/asset/wallet/balance`](https://developers.binance.com/en/docs/catalog/core-trading-wallet/api/rest-api/asset)

- Synthetic:
  [dual_only](../../../internal/platform/binance/testdata/sapi_v1_asset_wallet_balance.dual_only.json),
  [sample](../../../internal/platform/binance/testdata/sapi_v1_asset_wallet_balance.json).
- Recorded:
  [sample](../../../internal/platform/binance/evidence/sapi_v1_asset_wallet_balance.json)
  — Wallet totals retained; no cross-interface total acceptance.

### [`spot: GET /sapi/v1/dci/product/positions`](https://developers.binance.com/en/docs/catalog/investment-and-services-dual-investment/api/rest-api/trade#get-dual-investment-positions)

- Synthetic:
  [pending](../../../internal/platform/binance/testdata/sapi_v1_dci_product_positions.pending.json),
  [purchase_success](../../../internal/platform/binance/testdata/sapi_v1_dci_product_positions.purchase_success.json),
  [purchase_success_call](../../../internal/platform/binance/testdata/sapi_v1_dci_product_positions.purchase_success_call.json),
  [purchase_success_put](../../../internal/platform/binance/testdata/sapi_v1_dci_product_positions.purchase_success_put.json),
  [refunding](../../../internal/platform/binance/testdata/sapi_v1_dci_product_positions.refunding.json),
  [settling](../../../internal/platform/binance/testdata/sapi_v1_dci_product_positions.settling.json).
- Recorded:
  [purchase_success](../../../internal/platform/binance/evidence/sapi_v1_dci_product_positions.purchase_success.json)
  — PUT USDT/SOL and CALL SOL/USDT coexist; total 2; orderId is an integer as
  documented.

### [`spot: GET /sapi/v1/margin/account`](https://developers.binance.com/en/docs/catalog/core-trading-margin-trading/api/rest-api/account#query-cross-margin-account-details)

- Synthetic:
  [sample](../../../internal/platform/binance/testdata/sapi_v1_margin_account.json).
- Recorded:
  [sample](../../../internal/platform/binance/evidence/sapi_v1_margin_account.json)
  — Nonzero borrowing/interest and negative USDT netAsset; 3/486 rows retained.
  Observed totalCollateralValueInUSDT differs in case from the official example.

### [`spot: GET /sapi/v1/margin/isolated/account`](https://developers.binance.com/en/docs/catalog/core-trading-margin-trading/api/rest-api/account#query-isolated-margin-account-info)

- Synthetic:
  [sample](../../../internal/platform/binance/testdata/sapi_v1_margin_isolated_account.json).
- Recorded:
  [sample](../../../internal/platform/binance/evidence/sapi_v1_margin_isolated_account.json)
  — Two funded pairs, DOGEUSDT and ETHUSDT; ETHUSDT has nonzero USDT
  borrowing/interest.

### [`spot: GET /sapi/v1/simple-earn/flexible/position`](https://developers.binance.com/en/docs/catalog/investment-and-services-simple-earn/api/rest-api/flexible-locked#get-flexible-product-position)

- Synthetic:
  [empty](../../../internal/platform/binance/testdata/sapi_v1_simple_earn_flexible_position.empty.json),
  [sample](../../../internal/platform/binance/testdata/sapi_v1_simple_earn_flexible_position.json).
- Recorded:
  [sample](../../../internal/platform/binance/evidence/sapi_v1_simple_earn_flexible_position.json)
  — One USDT position; total 1.

### [`spot: GET /sapi/v1/simple-earn/locked/position`](https://developers.binance.com/en/docs/catalog/investment-and-services-simple-earn/api/rest-api/flexible-locked#get-locked-product-position)

- Synthetic:
  [empty](../../../internal/platform/binance/testdata/sapi_v1_simple_earn_locked_position.empty.json),
  [sample](../../../internal/platform/binance/testdata/sapi_v1_simple_earn_locked_position.json).
- Recorded:
  [sample](../../../internal/platform/binance/evidence/sapi_v1_simple_earn_locked_position.json)
  — One ETH position; duration 5, HOLDING; total 1.

### [`spot: POST /sapi/v1/asset/get-funding-asset`](https://developers.binance.com/en/docs/catalog/core-trading-wallet/api/rest-api/asset)

- Synthetic:
  [sample](../../../internal/platform/binance/testdata/sapi_v1_asset_get_funding_asset.json).
- Recorded:
  [sample](../../../internal/platform/binance/evidence/sapi_v1_asset_get_funding_asset.json)
  — Nonempty Funding wallet.

### [`spot: POST /sapi/v3/asset/getUserAsset`](https://developers.binance.com/en/docs/catalog/core-trading-wallet/api/rest-api/asset)

- Synthetic:
  [sample](../../../internal/platform/binance/testdata/sapi_v3_asset_getUserAsset.json).
- Recorded:
  [sample](../../../internal/platform/binance/evidence/sapi_v3_asset_getUserAsset.json)
  — Nonempty Spot wallet.

### [`um: GET /fapi/v1/exchangeInfo`](https://developers.binance.com/en/docs/catalog/advanced-trading-derivatives-trading-usds-margined-futures/api/rest-api/market-data)

- Synthetic:
  [sample](../../../internal/platform/binance/testdata/fapi_v1_exchangeInfo.json).
- Recorded:
  [sample](../../../internal/platform/binance/evidence/fapi_v1_exchangeInfo.json)
  — Eleven of 919 representative/prepared markets retained; metadata is not
  position evidence.

### [`um: GET /fapi/v1/multiAssetsMargin`](https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/rest-api/account#get-current-multi-assets-mode)

- Synthetic:
  [sample](../../../internal/platform/binance/testdata/fapi_v1_multiAssetsMargin.json),
  [single_asset](../../../internal/platform/binance/testdata/fapi_v1_multiAssetsMargin.single_asset.json).
- Recorded:
  [single_asset](../../../internal/platform/binance/evidence/fapi_v1_multiAssetsMargin.single_asset.json)
  — multiAssetsMargin false (Single-Asset).
  [multi_assets](../../../internal/platform/binance/evidence/fapi_v1_multiAssetsMargin.multi_assets.json)
  — multiAssetsMargin true, directly confirming Multi-Assets.

### [`um: GET /fapi/v1/symbolConfig`](https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/rest-api/account#symbol-configuration)

- Synthetic:
  [isolated](../../../internal/platform/binance/testdata/fapi_v1_symbolConfig.isolated.json),
  [sample](../../../internal/platform/binance/testdata/fapi_v1_symbolConfig.json).
- Recorded:
  [one_way](../../../internal/platform/binance/evidence/fapi_v1_symbolConfig.one_way.json)
  — Cross/isolated settings and integer leverage; all 919 rows retained.
  [hedge](../../../internal/platform/binance/evidence/fapi_v1_symbolConfig.hedge.json)
  — ETHUSDT ISOLATED, integer leverage 20; one symbol-level entry for both
  sides; all 919 rows retained.

### [`um: GET /fapi/v3/account`](https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/rest-api/account#account-information-v3)

- Synthetic:
  [sample](../../../internal/platform/binance/testdata/fapi_v3_account.json),
  [one_way_isolated](../../../internal/platform/binance/testdata/fapi_v3_account.one_way_isolated.json),
  [single_asset](../../../internal/platform/binance/testdata/fapi_v3_account.single_asset.json).
- Recorded:
  [single_asset](../../../internal/platform/binance/evidence/fapi_v3_account.single_asset.json)
  — Nonempty USDT; cross perpetual and isolated delivery; 4/13 balance/position
  rows retained.
  [hedge](../../../internal/platform/binance/evidence/fapi_v3_account.hedge.json)
  — Nonempty USDT and simultaneous isolated ETHUSDT LONG/SHORT; 4/13 rows
  retained. Single-Asset is owner-declared in this round.
  [multi_assets](../../../internal/platform/binance/evidence/fapi_v3_account.multi_assets.json)
  — Nonempty USDT/ETH collateral and ETHUSDT LONG 0.009 / SHORT -0.009; both
  isolated fields zero, positive account margin totals. All 11 assets / 2
  positions retained, including zero-wallet assets with nonzero
  availableBalance; observed asset scope is broader than the documentation list.

### [`um: GET /fapi/v3/positionRisk`](https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/rest-api/trade#position-information-v3)

- Synthetic:
  [hedge](../../../internal/platform/binance/testdata/fapi_v3_positionRisk.hedge.json),
  [sample](../../../internal/platform/binance/testdata/fapi_v3_positionRisk.json),
  [one_way_isolated](../../../internal/platform/binance/testdata/fapi_v3_positionRisk.one_way_isolated.json).
- Recorded:
  [one_way](../../../internal/platform/binance/evidence/fapi_v3_positionRisk.one_way.json)
  — ETHUSDT short/cross and ETHUSDT_261225 long/isolated, BOTH; nonzero isolated
  wallet/margin.
  [hedge](../../../internal/platform/binance/evidence/fapi_v3_positionRisk.hedge.json)
  — ETHUSDT LONG 0.009 / SHORT -0.010, both with nonzero
  isolatedWallet/isolatedMargin; both rows retained.

## Exchange-app validation checklist

Status: pending. This is an acceptance procedure, not a record of completed
checks. Recorded evidence and synthetic tests do not establish a reconciled
Account total. Human operators execute it on the remote test environment; agent
sessions remain limited to the isolated development instance.

Run these on the test environment against the Binance app on the same account,
within a few minutes of a collection. Items 5 and 6 need a Classic Trading
account.

1. **Identity and structure**: validation stores the UID shown under the app's
   profile. Validating the account as the other structure fails with
   `unsupported`, naming both. After switching structure in the app, the next
   Collection Run fails the same way without reading any Account.
2. **Portfolio Margin equity**: `Equity` equals the app's Portfolio Margin
   "Total equity" (actual equity) in USD. `AdjEq` equals the adjusted,
   collateral-rate equity. `UniMMR` equals the uniMMR on the Portfolio Margin
   page.
3. **Portfolio Margin balances**: each coin's equity matches the app's per-asset
   balance including unrealized PnL. A coin with a COIN-M loss can be negative.
4. **Positions**: every open USDⓈ-M and COIN-M position appears with the same
   direction, normalized Size (USDⓈ-M in Base coins, COIN-M in Quote face
   currency), entry price, and leverage. Liquidation prices agree within
   rounding.
5. **Classic margin**: each Cross Margin coin's equity equals its net asset and
   its liability equals its borrowed amount, and the margin level matches the
   app's. Isolated Margin shows each coin once, summed over its pairs. A User
   without margin sees both Accounts ok and empty.
6. **Classic futures**: USDⓈ-M per-asset margin balances match the futures
   wallet. In Multi-Assets Mode, `AdjEq` equals the margin balance and the
   maintenance margin ratio equals the Margin Ratio of the futures page. In
   Single-Asset Mode, those figures are empty. COIN-M balances are per coin, and
   source position quantities are in contracts; displayed Size is Quote face
   amount.
7. **Spot, funding, and Earn**: coin amounts match the Spot and Funding wallets,
   and flexible-earn receipts do not appear under spot. Simple Earn flexible and
   locked amounts match the Earn page. Each running dual investment position
   appears in the separate Dual Investment Account at its subscription amount in
   the deposit coin until it is paid out, and a settled one no longer appears.
   The Earn Account succeeds. Holding an Earn product Plutus does not read makes
   it Stale with both USD amounts.
8. **Account values**: the calculated USD value of every Account without
   reported equity is within about 0.1% of the app's USD value of that wallet.
9. **Failures**: removing the server IP from the key's whitelist sets the
   Connection `invalid` with a `-2015` detail at the next cycle. Restoring it
   and validating again resumes collection.
