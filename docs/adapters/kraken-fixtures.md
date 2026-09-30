# Kraken interface sample inventory

Read the [shared contract and workflow](./fixtures.md) and [mapping rules](./kraken.md). The library currently contains 28 **documented** responses and **zero recorded responses**. All old recordings were removed; no new-format real capture, owner acceptance or freeze is claimed. Inline boundary/error overrides are synthetic.

Documented mapping scope: Separate wallets and Unified Wallet routing; Main, Multi-M, Coin-M and Holding, linear/inverse contracts, Spot Margin net value, Earn/Auto Earn classification and refusals. Tests retain pagination, malformed values, incomplete products, failure isolation and error classification. Metadata proves instrument mapping, never a nonempty real holding.

## Acquisition status

Every interface below awaits real evidence under ADR 0036. Record one selected request only after agreeing its concrete account preparation. First acquire Binance; OKX, Bybit and Kraken follow independently. Future preparation distinguishes request/mapping variants from unrelated account settings; coexisting products can share a response. Permission/product availability, transient settlement states and difficult pagination boundaries remain explicit real evidence gaps until observed. No account preparation or exchange request was performed during this rebuild.

## Files

| File | Exact request | Evidence |
| --- | --- | --- |
| [0_private_BalanceEx.separate.json](../../internal/adapters/kraken/testdata/0_private_BalanceEx.separate.json) | `spot: POST /0/private/BalanceEx` | [documented](https://docs.kraken.com/api-reference/account-data/get-extended-balance) |
| [0_private_BalanceEx.unified.json](../../internal/adapters/kraken/testdata/0_private_BalanceEx.unified.json) | `spot: POST /0/private/BalanceEx` | [documented](https://docs.kraken.com/api-reference/account-data/get-extended-balance) |
| [0_private_Earn_Allocations.json](../../internal/adapters/kraken/testdata/0_private_Earn_Allocations.json) | `spot: POST /0/private/Earn/Allocations {"form":{"converted_asset":"USD","hide_zero_allocations":"true"}}` | [documented](https://docs.kraken.com/api-reference/earn/list-earn-allocations) |
| [0_private_Earn_Allocations.permission_denied.json](../../internal/adapters/kraken/testdata/0_private_Earn_Allocations.permission_denied.json) | `spot: POST /0/private/Earn/Allocations {"form":{"converted_asset":"USD","hide_zero_allocations":"true"}}` | [documented](https://docs.kraken.com/exchange/guides/general/errors) |
| [0_private_Earn_Allocations.rate_limited.json](../../internal/adapters/kraken/testdata/0_private_Earn_Allocations.rate_limited.json) | `spot: POST /0/private/Earn/Allocations {"form":{"converted_asset":"USD","hide_zero_allocations":"true"}}` | [documented](https://docs.kraken.com/exchange/guides/rest/ratelimits) |
| [0_private_Earn_Allocations.throttled.json](../../internal/adapters/kraken/testdata/0_private_Earn_Allocations.throttled.json) | `spot: POST /0/private/Earn/Allocations {"form":{"converted_asset":"USD","hide_zero_allocations":"true"}}` | [documented](https://docs.kraken.com/exchange/guides/rest/ratelimits) |
| [0_private_Earn_Allocations.too_many_requests.json](../../internal/adapters/kraken/testdata/0_private_Earn_Allocations.too_many_requests.json) | `spot: POST /0/private/Earn/Allocations {"form":{"converted_asset":"USD","hide_zero_allocations":"true"}}` | [documented](https://docs.kraken.com/exchange/guides/general/errors) |
| [0_private_Earn_Allocations.unavailable.json](../../internal/adapters/kraken/testdata/0_private_Earn_Allocations.unavailable.json) | `spot: POST /0/private/Earn/Allocations {"form":{"converted_asset":"USD","hide_zero_allocations":"true"}}` | [documented](https://docs.kraken.com/exchange/guides/general/errors) |
| [0_private_Earn_Strategies.json](../../internal/adapters/kraken/testdata/0_private_Earn_Strategies.json) | `spot: POST /0/private/Earn/Strategies` | [documented](https://docs.kraken.com/api-reference/earn/list-earn-strategies) |
| [0_private_GetApiKeyInfo.bad_signature.json](../../internal/adapters/kraken/testdata/0_private_GetApiKeyInfo.bad_signature.json) | `spot: POST /0/private/GetApiKeyInfo` | [documented](https://docs.kraken.com/exchange/guides/general/errors) |
| [0_private_GetApiKeyInfo.invalid_key.json](../../internal/adapters/kraken/testdata/0_private_GetApiKeyInfo.invalid_key.json) | `spot: POST /0/private/GetApiKeyInfo` | [documented](https://docs.kraken.com/exchange/guides/general/errors) |
| [0_private_GetApiKeyInfo.invalid_nonce.json](../../internal/adapters/kraken/testdata/0_private_GetApiKeyInfo.invalid_nonce.json) | `spot: POST /0/private/GetApiKeyInfo` | [documented](https://docs.kraken.com/exchange/guides/general/errors) |
| [0_private_GetApiKeyInfo.json](../../internal/adapters/kraken/testdata/0_private_GetApiKeyInfo.json) | `spot: POST /0/private/GetApiKeyInfo` | [documented](https://docs.kraken.com/api/docs/rest-api/get-api-key-info) |
| [0_private_TradeBalance.separate.json](../../internal/adapters/kraken/testdata/0_private_TradeBalance.separate.json) | `spot: POST /0/private/TradeBalance {"form":{"asset":"ZUSD"}}` | [documented](https://docs.kraken.com/api-reference/account-data/get-trade-balance) |
| [0_private_TradeBalance.unified.json](../../internal/adapters/kraken/testdata/0_private_TradeBalance.unified.json) | `spot: POST /0/private/TradeBalance {"form":{"asset":"ZUSD"}}` | [documented](https://docs.kraken.com/api-reference/account-data/get-trade-balance) |
| [0_public_AssetPairs.json](../../internal/adapters/kraken/testdata/0_public_AssetPairs.json) | `spot: GET /0/public/AssetPairs` | [documented](https://docs.kraken.com/api-reference/market-data/get-tradable-asset-pairs) |
| [0_public_Assets.json](../../internal/adapters/kraken/testdata/0_public_Assets.json) | `spot: GET /0/public/Assets` | [documented](https://docs.kraken.com/api-reference/market-data/get-asset-info) |
| [0_public_Ticker.json](../../internal/adapters/kraken/testdata/0_public_Ticker.json) | `spot: GET /0/public/Ticker` | [documented](https://docs.kraken.com/api-reference/market-data/get-ticker-information) |
| [api_auth_v1_api_keys_v3_check.bad_signature.json](../../internal/adapters/kraken/testdata/api_auth_v1_api_keys_v3_check.bad_signature.json) | `futures-auth: GET /api/auth/v1/api-keys/v3/check` | [documented](https://docs.kraken.com/exchange/guides/general/errors) |
| [api_auth_v1_api_keys_v3_check.invalid_key.json](../../internal/adapters/kraken/testdata/api_auth_v1_api_keys_v3_check.invalid_key.json) | `futures-auth: GET /api/auth/v1/api-keys/v3/check` | [documented](https://docs.kraken.com/exchange/guides/general/errors) |
| [api_auth_v1_api_keys_v3_check.json](../../internal/adapters/kraken/testdata/api_auth_v1_api_keys_v3_check.json) | `futures-auth: GET /api/auth/v1/api-keys/v3/check` | [documented](https://docs.kraken.com/api/docs/futures-api/auth/check-v-3-api-key) |
| [derivatives_api_v3_instruments.json](../../internal/adapters/kraken/testdata/derivatives_api_v3_instruments.json) | `futures: GET /derivatives/api/v3/instruments {"query":{"contractType":"all"}}` | [documented](https://docs.kraken.com/api-reference/instrument-details/get-instruments) |
| [derivatives_api_v3_openpositions.api_limit_exceeded.json](../../internal/adapters/kraken/testdata/derivatives_api_v3_openpositions.api_limit_exceeded.json) | `futures: GET /derivatives/api/v3/openpositions` | [documented](https://docs.kraken.com/exchange/guides/futures/ratelimits) |
| [derivatives_api_v3_openpositions.authentication_error.json](../../internal/adapters/kraken/testdata/derivatives_api_v3_openpositions.authentication_error.json) | `futures: GET /derivatives/api/v3/openpositions` | [documented](https://docs.kraken.com/api-reference/account-information/get-open-positions) |
| [derivatives_api_v3_openpositions.json](../../internal/adapters/kraken/testdata/derivatives_api_v3_openpositions.json) | `futures: GET /derivatives/api/v3/openpositions` | [documented](https://docs.kraken.com/api-reference/account-information/get-open-positions) |
| [derivatives_api_v3_tickers.json](../../internal/adapters/kraken/testdata/derivatives_api_v3_tickers.json) | `futures: GET /derivatives/api/v3/tickers` | [documented](https://docs.kraken.com/api-reference/market-data/get-tickers) |
| [ws_v1_balances_snapshot.separate.json](../../internal/adapters/kraken/testdata/ws_v1_balances_snapshot.separate.json) | `futures-ws: WS /ws/v1/balances_snapshot` | [documented](https://docs.kraken.com/exchange/api-reference/futures-websocket/balances) |
| [ws_v1_balances_snapshot.unified.json](../../internal/adapters/kraken/testdata/ws_v1_balances_snapshot.unified.json) | `futures-ws: WS /ws/v1/balances_snapshot` | [documented](https://docs.kraken.com/exchange/api-reference/futures-websocket/balances) |

## Freeze log

No recorded samples are frozen under this standard. Promotion entries must identify the reviewed file, actual recording date, exercised state, offline test result and reason for adding/replacing it. Empty access evidence does not close nonempty mapping gaps.
