# OKX interface sample inventory

Read the [shared contract and workflow](./fixtures.md) and [mapping rules](./okx.md). The library currently contains 26 **documented** responses and **zero recorded responses**. All old recordings were removed; no new-format real capture, owner acceptance or freeze is claimed. Inline boundary/error overrides are synthetic.

Documented mapping scope: Unified Spot/Futures/MCM/PM responses, contract metadata, nonempty balances and contracts, Simple Earn, active On-chain Earn and both Dual investment directions. Tests retain pagination, malformed values, incomplete products, failure isolation and error classification. Metadata proves instrument mapping, never a nonempty real holding.

## Acquisition status

Every interface below awaits real evidence under ADR 0036. Record one selected request only after agreeing its concrete account preparation. First acquire Binance; OKX, Bybit and Kraken follow independently. Future preparation distinguishes request/mapping variants from unrelated account settings; coexisting products can share a response. Permission/product availability, transient settlement states and difficult pagination boundaries remain explicit real evidence gaps until observed. No account preparation or exchange request was performed during this rebuild.

## Files

| File | Exact request | Evidence |
| --- | --- | --- |
| [api_v5_account_balance.futures.json](../../internal/adapters/okx/testdata/api_v5_account_balance.futures.json) | `v5: GET /api/v5/account/balance` | [documented](https://www.okx.com/docs-v5/en/#trading-account-rest-api-get-balance) |
| [api_v5_account_balance.mcm.json](../../internal/adapters/okx/testdata/api_v5_account_balance.mcm.json) | `v5: GET /api/v5/account/balance` | [documented](https://www.okx.com/docs-v5/en/#trading-account-rest-api-get-balance) |
| [api_v5_account_balance.pm.json](../../internal/adapters/okx/testdata/api_v5_account_balance.pm.json) | `v5: GET /api/v5/account/balance` | [documented](https://www.okx.com/docs-v5/en/#trading-account-rest-api-get-balance) |
| [api_v5_account_balance.spot.json](../../internal/adapters/okx/testdata/api_v5_account_balance.spot.json) | `v5: GET /api/v5/account/balance` | [documented](https://www.okx.com/docs-v5/en/#trading-account-rest-api-get-balance) |
| [api_v5_account_config.bad_signature.json](../../internal/adapters/okx/testdata/api_v5_account_config.bad_signature.json) | `v5: GET /api/v5/account/config` | [documented](https://www.okx.com/docs-v5/en/#error-code-rest-api) |
| [api_v5_account_config.futures.json](../../internal/adapters/okx/testdata/api_v5_account_config.futures.json) | `v5: GET /api/v5/account/config` | [documented](https://www.okx.com/docs-v5/en/#trading-account-rest-api-get-account-configuration) |
| [api_v5_account_config.invalid_key.json](../../internal/adapters/okx/testdata/api_v5_account_config.invalid_key.json) | `v5: GET /api/v5/account/config` | [documented](https://www.okx.com/docs-v5/en/#error-code-rest-api) |
| [api_v5_account_config.mcm.json](../../internal/adapters/okx/testdata/api_v5_account_config.mcm.json) | `v5: GET /api/v5/account/config` | [documented](https://www.okx.com/docs-v5/en/#trading-account-rest-api-get-account-configuration) |
| [api_v5_account_config.pm.json](../../internal/adapters/okx/testdata/api_v5_account_config.pm.json) | `v5: GET /api/v5/account/config` | [documented](https://www.okx.com/docs-v5/en/#trading-account-rest-api-get-account-configuration) |
| [api_v5_account_config.spot.json](../../internal/adapters/okx/testdata/api_v5_account_config.spot.json) | `v5: GET /api/v5/account/config` | [documented](https://www.okx.com/docs-v5/en/#trading-account-rest-api-get-account-configuration) |
| [api_v5_account_config.stale_timestamp.json](../../internal/adapters/okx/testdata/api_v5_account_config.stale_timestamp.json) | `v5: GET /api/v5/account/config` | [documented](https://www.okx.com/docs-v5/en/#error-code-rest-api) |
| [api_v5_account_config.wrong_passphrase.json](../../internal/adapters/okx/testdata/api_v5_account_config.wrong_passphrase.json) | `v5: GET /api/v5/account/config` | [documented](https://www.okx.com/docs-v5/en/#error-code-rest-api) |
| [api_v5_account_positions.futures.json](../../internal/adapters/okx/testdata/api_v5_account_positions.futures.json) | `v5: GET /api/v5/account/positions` | [documented](https://www.okx.com/docs-v5/en/#trading-account-rest-api-get-positions) |
| [api_v5_account_positions.mcm.json](../../internal/adapters/okx/testdata/api_v5_account_positions.mcm.json) | `v5: GET /api/v5/account/positions` | [documented](https://www.okx.com/docs-v5/en/#trading-account-rest-api-get-positions) |
| [api_v5_account_positions.pm.json](../../internal/adapters/okx/testdata/api_v5_account_positions.pm.json) | `v5: GET /api/v5/account/positions` | [documented](https://www.okx.com/docs-v5/en/#trading-account-rest-api-get-positions) |
| [api_v5_account_positions.spot.json](../../internal/adapters/okx/testdata/api_v5_account_positions.spot.json) | `v5: GET /api/v5/account/positions` | [documented](https://www.okx.com/docs-v5/en/#trading-account-rest-api-get-positions) |
| [api_v5_asset_asset_valuation.json](../../internal/adapters/okx/testdata/api_v5_asset_asset_valuation.json) | `v5: GET /api/v5/asset/asset-valuation {"query":{"ccy":"USDT"}}` | [documented](https://www.okx.com/docs-v5/en/#funding-account-rest-api-get-account-asset-valuation) |
| [api_v5_asset_balances.json](../../internal/adapters/okx/testdata/api_v5_asset_balances.json) | `v5: GET /api/v5/asset/balances` | [documented](https://www.okx.com/docs-v5/en/#funding-account-rest-api-get-balance) |
| [api_v5_finance_savings_balance.json](../../internal/adapters/okx/testdata/api_v5_finance_savings_balance.json) | `v5: GET /api/v5/finance/savings/balance` | [documented](https://www.okx.com/docs-v5/en/#financial-product-simple-earn-flexible-get-saving-balance) |
| [api_v5_finance_sfp_dcd_order_history.json](../../internal/adapters/okx/testdata/api_v5_finance_sfp_dcd_order_history.json) | `v5: GET /api/v5/finance/sfp/dcd/order-history {"query":{"limit":"100"}}` | [documented](https://www.okx.com/docs-v5/en/#financial-product-dual-investment-get-order-history) |
| [api_v5_finance_sfp_dcd_order_history.put.json](../../internal/adapters/okx/testdata/api_v5_finance_sfp_dcd_order_history.put.json) | `v5: GET /api/v5/finance/sfp/dcd/order-history {"query":{"limit":"100"}}` | [documented](https://www.okx.com/docs-v5/en/#financial-product-dual-investment-get-order-history) |
| [api_v5_finance_staking_defi_orders_active.json](../../internal/adapters/okx/testdata/api_v5_finance_staking_defi_orders_active.json) | `v5: GET /api/v5/finance/staking-defi/orders-active` | [documented](https://www.okx.com/docs-v5/en/#financial-product-on-chain-earn-get-active-orders) |
| [api_v5_market_tickers.json](../../internal/adapters/okx/testdata/api_v5_market_tickers.json) | `v5: GET /api/v5/market/tickers {"query":{"instType":"SPOT"}}` | [documented](https://www.okx.com/docs-v5/en/#order-book-trading-market-data-get-tickers) |
| [api_v5_public_instruments.futures.json](../../internal/adapters/okx/testdata/api_v5_public_instruments.futures.json) | `v5: GET /api/v5/public/instruments {"query":{"instType":"FUTURES"}}` | [documented](https://www.okx.com/docs-v5/en/#public-data-rest-api-get-instruments) |
| [api_v5_public_instruments.swap.json](../../internal/adapters/okx/testdata/api_v5_public_instruments.swap.json) | `v5: GET /api/v5/public/instruments {"query":{"instType":"SWAP"}}` | [documented](https://www.okx.com/docs-v5/en/#public-data-rest-api-get-instruments) |
| [api_v5_public_time.json](../../internal/adapters/okx/testdata/api_v5_public_time.json) | `v5: GET /api/v5/public/time` | [documented](https://www.okx.com/docs-v5/en/#public-data-rest-api-system-time) |

## Freeze log

No recorded samples are frozen under this standard. Promotion entries must identify the reviewed file, actual recording date, exercised state, offline test result and reason for adding/replacing it. Empty access evidence does not close nonempty mapping gaps.
