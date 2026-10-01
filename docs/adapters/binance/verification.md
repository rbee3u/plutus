# Binance verification

This document owns current coverage, evidence review observations, remaining
real gaps, and pending exchange-app checks. See the
[Adapter mapping](./mapping.md) for field semantics and the
[evidence workflow](../evidence-workflow.md) for recording and promotion rules.
Synthetic tests, recorded responses, and exchange-app validation are distinct.

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

The following independent synthetic inputs test deliberate request, field,
mapping, pagination, and failure scenarios. Recorded links identify reviewed
responses and their observed limits. Actual per-file UTC timestamps and trimming
notes are retained in each recorded envelope. Generic interfaces are not limited
to the structure used to sample them.

### `cm: GET /dapi/v1/account`

[Contract](https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-coin-m-futures/api/rest-api/account#account-information)

Synthetic inputs:

- [dapi_v1_account.json](../../../internal/adapters/binance/testdata/dapi_v1_account.json)

Recorded evidence:

[dapi_v1_account.one_way.json](../../../internal/adapters/binance/evidence/dapi_v1_account.one_way.json);
2026-09-30T17:32:42.285334Z. Nonempty ETH wallet; inverse perpetual/delivery,
cross/isolated; 5/81 rows retained.

[dapi_v1_account.hedge.json](../../../internal/adapters/binance/evidence/dapi_v1_account.hedge.json);
2026-09-30T17:56:35.268755Z. Nonempty ETH wallet and simultaneous isolated
ETHUSD_PERP LONG/SHORT; 8/111 rows retained.

### `cm: GET /dapi/v1/exchangeInfo`

[Contract](https://developers.binance.com/en/docs/catalog/advanced-trading-derivatives-trading-coin-m-futures/api/rest-api/market-data)

Synthetic inputs:

- [dapi_v1_exchangeInfo.json](../../../internal/adapters/binance/testdata/dapi_v1_exchangeInfo.json)

Recorded evidence:

[dapi_v1_exchangeInfo.json](../../../internal/adapters/binance/evidence/dapi_v1_exchangeInfo.json);
2026-09-30T17:03:03.410241Z. Seven of 30 representative/prepared markets
retained; DOGE contract size 10; metadata is not position evidence.

### `cm: GET /dapi/v1/positionRisk`

[Contract](https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-coin-m-futures/api/rest-api/trade#position-information)

Synthetic inputs:

- [dapi_v1_positionRisk.json](../../../internal/adapters/binance/testdata/dapi_v1_positionRisk.json)

Recorded evidence:

[dapi_v1_positionRisk.one_way.json](../../../internal/adapters/binance/evidence/dapi_v1_positionRisk.one_way.json);
2026-09-30T17:32:46.729834Z. ETHUSD_PERP long/cross and ETHUSD_261225
short/isolated, BOTH; 3/30 rows retained.

[dapi_v1_positionRisk.hedge.json](../../../internal/adapters/binance/evidence/dapi_v1_positionRisk.hedge.json);
2026-09-30T17:56:38.776229Z. ETHUSD_PERP LONG 1 / SHORT -1, isolated with
separate nonzero wallets; 8/90 rows retained, including representative zero
BOTH/LONG/SHORT.

### `portfolio: GET /papi/v1/account`

[Contract](https://developers.binance.com/en/docs/catalog/advanced-trading-derivatives-trading-portfolio-margin/api/rest-api/account#account-information-user_data)

Synthetic inputs:

- [papi_v1_account.json](../../../internal/adapters/binance/testdata/papi_v1_account.json)

Recorded evidence:

[papi_v1_account.json](../../../internal/adapters/binance/evidence/papi_v1_account.json);
2026-09-30T17:01:09.101514Z. PM account equity and risk measures.

### `portfolio: GET /papi/v1/balance`

[Contract](https://developers.binance.com/en/docs/catalog/advanced-trading-derivatives-trading-portfolio-margin/api/rest-api/account#account-balance-user_data)

Synthetic inputs:

- [papi_v1_balance.json](../../../internal/adapters/binance/testdata/papi_v1_balance.json)

Recorded evidence:

[papi_v1_balance.json](../../../internal/adapters/binance/evidence/papi_v1_balance.json);
2026-09-30T17:01:13.120698Z. Nonzero borrowing and accrued interest; seven
retained asset rows.

### `portfolio: GET /papi/v1/cm/positionRisk`

[Contract](https://developers.binance.com/en/docs/catalog/advanced-trading-derivatives-trading-portfolio-margin/api/rest-api/account#query-cm-position-information-user_data)

Synthetic inputs:

- [papi_v1_cm_positionRisk.json](../../../internal/adapters/binance/testdata/papi_v1_cm_positionRisk.json)

Recorded evidence:

[papi_v1_cm_positionRisk.json](../../../internal/adapters/binance/evidence/papi_v1_cm_positionRisk.json);
2026-09-30T17:01:21.297902Z. One-way nonempty long: DOGEUSD_PERP.

[papi_v1_cm_positionRisk.hedge.json](../../../internal/adapters/binance/evidence/papi_v1_cm_positionRisk.hedge.json);
2026-09-30T18:31:20.863606Z. ETHUSD_PERP LONG 1 / SHORT -1, string leverage 20,
liquidationPrice 2.82678988 on both sides; both nonempty rows retained without
trimming.

### `portfolio: GET /papi/v1/um/positionRisk`

[Contract](https://developers.binance.com/en/docs/catalog/advanced-trading-derivatives-trading-portfolio-margin/api/rest-api/account#query-um-position-information-user_data)

Synthetic inputs:

- [papi_v1_um_positionRisk.json](../../../internal/adapters/binance/testdata/papi_v1_um_positionRisk.json)

Recorded evidence:

[papi_v1_um_positionRisk.json](../../../internal/adapters/binance/evidence/papi_v1_um_positionRisk.json);
2026-09-30T17:01:17.728886Z. One-way nonempty shorts: 牛来USDT and ZECUSDT.

[papi_v1_um_positionRisk.hedge.json](../../../internal/adapters/binance/evidence/papi_v1_um_positionRisk.hedge.json);
2026-09-30T18:31:16.597003Z. ETHUSDT LONG 0.009 / SHORT -0.008, string leverage
5, liquidationPrice 0; both nonempty rows retained without trimming.

### `spot: GET /api/v3/account`

[Contract](https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md#account-information-user_data)

Synthetic inputs:

- [api_v3_account.json](../../../internal/adapters/binance/testdata/api_v3_account.json)

Recorded evidence:

[api_v3_account.json](../../../internal/adapters/binance/evidence/api_v3_account.json);
2026-09-30T17:00:56.6164Z. Nonempty balances; UID masked.

### `spot: GET /api/v3/ticker/price`

[Contract](https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md#symbol-price-ticker)

Synthetic inputs:

- [api_v3_ticker_price.json](../../../internal/adapters/binance/testdata/api_v3_ticker_price.json)

Recorded evidence:

[api_v3_ticker_price.json](../../../internal/adapters/binance/evidence/api_v3_ticker_price.json);
2026-09-30T17:03:57.472302Z. Nine of 3723 representative/prepared price rows
retained.

### `spot: GET /api/v3/time`

[Contract](https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md#check-server-time)

Synthetic inputs:

- [api_v3_time.json](../../../internal/adapters/binance/testdata/api_v3_time.json)

Recorded evidence:

[api_v3_time.json](../../../internal/adapters/binance/evidence/api_v3_time.json);
2026-09-30T17:02:50.567098Z. Public server-time response.

### `spot: GET /sapi/v1/account/info`

[Contract](https://developers.binance.com/docs/binance-spot-api-docs/errors)

Synthetic inputs:

- [sapi_v1_account_info.bad_signature.json](../../../internal/adapters/binance/testdata/sapi_v1_account_info.bad_signature.json)
- [sapi_v1_account_info.classic.json](../../../internal/adapters/binance/testdata/sapi_v1_account_info.classic.json)
- [sapi_v1_account_info.invalid_key.json](../../../internal/adapters/binance/testdata/sapi_v1_account_info.invalid_key.json)
- [sapi_v1_account_info.pm.json](../../../internal/adapters/binance/testdata/sapi_v1_account_info.pm.json)
- [sapi_v1_account_info.stale_timestamp.json](../../../internal/adapters/binance/testdata/sapi_v1_account_info.stale_timestamp.json)

Recorded evidence:

[sapi_v1_account_info.pm.json](../../../internal/adapters/binance/evidence/sapi_v1_account_info.pm.json);
2026-09-30T17:00:14.877426Z. PM flag true; owner-declared PM main account.

[sapi_v1_account_info.classic.json](../../../internal/adapters/binance/evidence/sapi_v1_account_info.classic.json);
2026-09-30T17:31:33.923818Z. Classic flag false; margin/futures enabled.

### `spot: GET /sapi/v1/asset/wallet/balance`

[Contract](https://developers.binance.com/en/docs/catalog/core-trading-wallet/api/rest-api/asset)

Synthetic inputs:

- [sapi_v1_asset_wallet_balance.dual_only.json](../../../internal/adapters/binance/testdata/sapi_v1_asset_wallet_balance.dual_only.json)
- [sapi_v1_asset_wallet_balance.json](../../../internal/adapters/binance/testdata/sapi_v1_asset_wallet_balance.json)

Recorded evidence:

[sapi_v1_asset_wallet_balance.json](../../../internal/adapters/binance/evidence/sapi_v1_asset_wallet_balance.json);
2026-09-30T17:02:30.782555Z. Wallet totals retained; no cross-interface total
acceptance.

### `spot: GET /sapi/v1/dci/product/positions`

[Contract](https://developers.binance.com/en/docs/catalog/investment-and-services-dual-investment/api/rest-api/trade#get-dual-investment-positions)

Synthetic inputs:

- [sapi_v1_dci_product_positions.pending.json](../../../internal/adapters/binance/testdata/sapi_v1_dci_product_positions.pending.json)
- [sapi_v1_dci_product_positions.purchase_success.json](../../../internal/adapters/binance/testdata/sapi_v1_dci_product_positions.purchase_success.json)
- [sapi_v1_dci_product_positions.purchase_success_call.json](../../../internal/adapters/binance/testdata/sapi_v1_dci_product_positions.purchase_success_call.json)
- [sapi_v1_dci_product_positions.purchase_success_put.json](../../../internal/adapters/binance/testdata/sapi_v1_dci_product_positions.purchase_success_put.json)
- [sapi_v1_dci_product_positions.refunding.json](../../../internal/adapters/binance/testdata/sapi_v1_dci_product_positions.refunding.json)
- [sapi_v1_dci_product_positions.settling.json](../../../internal/adapters/binance/testdata/sapi_v1_dci_product_positions.settling.json)

Recorded evidence:

[sapi_v1_dci_product_positions.purchase_success.json](../../../internal/adapters/binance/evidence/sapi_v1_dci_product_positions.purchase_success.json);
2026-09-30T17:02:27.2005Z. PUT USDT/SOL and CALL SOL/USDT coexist; total 2;
orderId is an integer as documented.

### `spot: GET /sapi/v1/margin/account`

[Contract](https://developers.binance.com/en/docs/catalog/core-trading-margin-trading/api/rest-api/account#query-cross-margin-account-details)

Synthetic inputs:

- [sapi_v1_margin_account.json](../../../internal/adapters/binance/testdata/sapi_v1_margin_account.json)

Recorded evidence:

[sapi_v1_margin_account.json](../../../internal/adapters/binance/evidence/sapi_v1_margin_account.json);
2026-09-30T17:32:23.922973Z. Nonzero borrowing/interest and negative USDT
netAsset; 3/486 rows retained. Observed totalCollateralValueInUSDT differs in
case from the official example.

### `spot: GET /sapi/v1/margin/isolated/account`

[Contract](https://developers.binance.com/en/docs/catalog/core-trading-margin-trading/api/rest-api/account#query-isolated-margin-account-info)

Synthetic inputs:

- [sapi_v1_margin_isolated_account.json](../../../internal/adapters/binance/testdata/sapi_v1_margin_isolated_account.json)

Recorded evidence:

[sapi_v1_margin_isolated_account.json](../../../internal/adapters/binance/evidence/sapi_v1_margin_isolated_account.json);
2026-09-30T17:40:03.535114Z. Two funded pairs, DOGEUSDT and ETHUSDT; ETHUSDT has
nonzero USDT borrowing/interest.

### `spot: GET /sapi/v1/simple-earn/flexible/position`

[Contract](https://developers.binance.com/en/docs/catalog/investment-and-services-simple-earn/api/rest-api/flexible-locked#get-flexible-product-position)

Synthetic inputs:

- [sapi_v1_simple_earn_flexible_position.empty.json](../../../internal/adapters/binance/testdata/sapi_v1_simple_earn_flexible_position.empty.json)
- [sapi_v1_simple_earn_flexible_position.json](../../../internal/adapters/binance/testdata/sapi_v1_simple_earn_flexible_position.json)

Recorded evidence:

[sapi_v1_simple_earn_flexible_position.json](../../../internal/adapters/binance/evidence/sapi_v1_simple_earn_flexible_position.json);
2026-09-30T17:02:17.362927Z. One USDT position; total 1.

### `spot: GET /sapi/v1/simple-earn/locked/position`

[Contract](https://developers.binance.com/en/docs/catalog/investment-and-services-simple-earn/api/rest-api/flexible-locked#get-locked-product-position)

Synthetic inputs:

- [sapi_v1_simple_earn_locked_position.empty.json](../../../internal/adapters/binance/testdata/sapi_v1_simple_earn_locked_position.empty.json)
- [sapi_v1_simple_earn_locked_position.json](../../../internal/adapters/binance/testdata/sapi_v1_simple_earn_locked_position.json)

Recorded evidence:

[sapi_v1_simple_earn_locked_position.json](../../../internal/adapters/binance/evidence/sapi_v1_simple_earn_locked_position.json);
2026-09-30T17:02:22.643714Z. One ETH position; duration 5, HOLDING; total 1.

### `spot: POST /sapi/v1/asset/get-funding-asset`

[Contract](https://developers.binance.com/en/docs/catalog/core-trading-wallet/api/rest-api/asset)

Synthetic inputs:

- [sapi_v1_asset_get_funding_asset.json](../../../internal/adapters/binance/testdata/sapi_v1_asset_get_funding_asset.json)

Recorded evidence:

[sapi_v1_asset_get_funding_asset.json](../../../internal/adapters/binance/evidence/sapi_v1_asset_get_funding_asset.json);
2026-09-30T17:01:05.717296Z. Nonempty Funding wallet.

### `spot: POST /sapi/v3/asset/getUserAsset`

[Contract](https://developers.binance.com/en/docs/catalog/core-trading-wallet/api/rest-api/asset)

Synthetic inputs:

- [sapi_v3_asset_getUserAsset.json](../../../internal/adapters/binance/testdata/sapi_v3_asset_getUserAsset.json)

Recorded evidence:

[sapi_v3_asset_getUserAsset.json](../../../internal/adapters/binance/evidence/sapi_v3_asset_getUserAsset.json);
2026-09-30T17:01:01.234487Z. Nonempty Spot wallet.

### `um: GET /fapi/v1/exchangeInfo`

[Contract](https://developers.binance.com/en/docs/catalog/advanced-trading-derivatives-trading-usds-margined-futures/api/rest-api/market-data)

Synthetic inputs:

- [fapi_v1_exchangeInfo.json](../../../internal/adapters/binance/testdata/fapi_v1_exchangeInfo.json)

Recorded evidence:

[fapi_v1_exchangeInfo.json](../../../internal/adapters/binance/evidence/fapi_v1_exchangeInfo.json);
2026-09-30T17:02:58.246573Z. Eleven of 919 representative/prepared markets
retained; metadata is not position evidence.

### `um: GET /fapi/v1/multiAssetsMargin`

[Contract](https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/rest-api/account#get-current-multi-assets-mode)

Synthetic inputs:

- [fapi_v1_multiAssetsMargin.json](../../../internal/adapters/binance/testdata/fapi_v1_multiAssetsMargin.json)
- [fapi_v1_multiAssetsMargin.single_asset.json](../../../internal/adapters/binance/testdata/fapi_v1_multiAssetsMargin.single_asset.json)

Recorded evidence:

[fapi_v1_multiAssetsMargin.single_asset.json](../../../internal/adapters/binance/evidence/fapi_v1_multiAssetsMargin.single_asset.json);
2026-09-30T17:31:58.680116Z. multiAssetsMargin false (Single-Asset).

[fapi_v1_multiAssetsMargin.multi_assets.json](../../../internal/adapters/binance/evidence/fapi_v1_multiAssetsMargin.multi_assets.json);
2026-09-30T18:13:38.506073Z. multiAssetsMargin true, directly confirming
Multi-Assets.

### `um: GET /fapi/v1/symbolConfig`

[Contract](https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/rest-api/account#symbol-configuration)

Synthetic inputs:

- [fapi_v1_symbolConfig.isolated.json](../../../internal/adapters/binance/testdata/fapi_v1_symbolConfig.isolated.json)
- [fapi_v1_symbolConfig.json](../../../internal/adapters/binance/testdata/fapi_v1_symbolConfig.json)

Recorded evidence:

[fapi_v1_symbolConfig.one_way.json](../../../internal/adapters/binance/evidence/fapi_v1_symbolConfig.one_way.json);
2026-09-30T17:40:16.796561Z. Cross/isolated settings and integer leverage; all
919 rows retained.

[fapi_v1_symbolConfig.hedge.json](../../../internal/adapters/binance/evidence/fapi_v1_symbolConfig.hedge.json);
2026-09-30T17:56:31.774781Z. ETHUSDT ISOLATED, integer leverage 20; one
symbol-level entry for both sides; all 919 rows retained.

### `um: GET /fapi/v3/account`

[Contract](https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/rest-api/account#account-information-v3)

Synthetic inputs:

- [fapi_v3_account.json](../../../internal/adapters/binance/testdata/fapi_v3_account.json)
- [fapi_v3_account.one_way_isolated.json](../../../internal/adapters/binance/testdata/fapi_v3_account.one_way_isolated.json)
- [fapi_v3_account.single_asset.json](../../../internal/adapters/binance/testdata/fapi_v3_account.single_asset.json)

Recorded evidence:

[fapi_v3_account.single_asset.json](../../../internal/adapters/binance/evidence/fapi_v3_account.single_asset.json);
2026-09-30T17:40:08.417681Z. Nonempty USDT; cross perpetual and isolated
delivery; 4/13 balance/position rows retained.

[fapi_v3_account.hedge.json](../../../internal/adapters/binance/evidence/fapi_v3_account.hedge.json);
2026-09-30T17:56:22.49378Z. Nonempty USDT and simultaneous isolated ETHUSDT
LONG/SHORT; 4/13 rows retained. Single-Asset is owner-declared in this round.

[fapi_v3_account.multi_assets.json](../../../internal/adapters/binance/evidence/fapi_v3_account.multi_assets.json);
2026-09-30T18:13:42.992579Z. Nonempty USDT/ETH collateral and ETHUSDT LONG 0.009
/ SHORT -0.009; both isolated fields zero, positive account margin totals. All
11 assets / 2 positions retained, including zero-wallet assets with nonzero
availableBalance; observed asset scope is broader than the documentation list.

### `um: GET /fapi/v3/positionRisk`

[Contract](https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/rest-api/trade#position-information-v3)

Synthetic inputs:

- [fapi_v3_positionRisk.hedge.json](../../../internal/adapters/binance/testdata/fapi_v3_positionRisk.hedge.json)
- [fapi_v3_positionRisk.json](../../../internal/adapters/binance/testdata/fapi_v3_positionRisk.json)
- [fapi_v3_positionRisk.one_way_isolated.json](../../../internal/adapters/binance/testdata/fapi_v3_positionRisk.one_way_isolated.json)

Recorded evidence:

[fapi_v3_positionRisk.one_way.json](../../../internal/adapters/binance/evidence/fapi_v3_positionRisk.one_way.json);
2026-09-30T17:40:12.355753Z. ETHUSDT short/cross and ETHUSDT_261225
long/isolated, BOTH; nonzero isolated wallet/margin.

[fapi_v3_positionRisk.hedge.json](../../../internal/adapters/binance/evidence/fapi_v3_positionRisk.hedge.json);
2026-09-30T17:56:28.522273Z. ETHUSDT LONG 0.009 / SHORT -0.010, both with
nonzero isolatedWallet/isolatedMargin; both rows retained.

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
   side, size (USDⓈ-M in coins, COIN-M in contracts), entry price, and leverage.
   Liquidation prices agree within rounding.
5. **Classic margin**: each Cross Margin coin's equity equals its net asset and
   its liability equals its borrowed amount, and the margin level matches the
   app's. Isolated Margin shows each coin once, summed over its pairs. A User
   without margin sees both Accounts ok and empty.
6. **Classic futures**: USDⓈ-M per-asset margin balances match the futures
   wallet. In Multi-Assets Mode, `AdjEq` equals the margin balance and the
   maintenance margin ratio equals the Margin Ratio of the futures page. In
   Single-Asset Mode, those figures are empty. COIN-M balances are per coin, and
   positions are in contracts.
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
