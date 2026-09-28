# Kraken adapter

Package `internal/adapters/kraken`. Account Structures `kraken_separate` (Separate wallets) and `kraken_unified` (Unified Wallet), [ADR 0026](../adr/0026-support-kraken-separate-and-unified-wallets.md). Neither has a Margin Mode ([ADR 0028](../adr/0028-separate-account-structure-from-margin-mode.md)): validation and every batch return an empty one. Only Unified Wallet has been exercised against a real account; Separate wallets are covered by documentation-derived responses ([ADR 0027](../adr/0027-allow-documented-adapter-scenarios-without-live-coverage.md)). Every call is a read-only balance, position, account, or market-data endpoint of Kraken's Spot REST API or Futures REST API, plus one balances snapshot of the Futures WebSocket feed.

| Account Type | `kraken_separate` | `kraken_unified` |
| --- | --- | --- |
| `main` (Main) | the Spot wallet, with Kraken Rewards (Auto Earn) and the net result of open Spot Margin positions | the Unified Wallet: Spot, Spot Margin, and multi-collateral futures in one wallet |
| `earn` (Earn) | funds allocated to Earn strategies | same |
| `multi_m` (Multi-M) | the multi-collateral futures wallet | not offered: it is Main |
| `coin_m` (Coin-M) | the single-collateral futures wallets | same |
| `derivatives_holding` (Derivatives Holding) | the futures platform's non-trading holding wallet | same |

## Credentials

One Connection holds a Spot key pair and a Futures key pair, replaced together. Main and Earn need the Spot pair; Main, Multi-M, Coin-M, and Derivatives Holding need the Futures pair, which Main uses to check the Account Structure. An Identity with Earn alone needs only the Spot pair. Validation reads `iban` from the Spot key and `iiban` from the Futures key, removes spaces, uppercases both, and refuses keys of different customers with `authentication`. The Identity's external reference is the Spot `iban`, else the Futures `iiban`. Kraken documents `iiban` as nullable: without it the two pairs cannot be compared, and a Futures-only Identity is bound to the Futures `accountUid` instead. Adding the Spot pair to such an Identity later changes its reference, which validation refuses as a different account ([ADR 0028](../adr/0028-separate-account-structure-from-margin-mode.md)), so add both key groups before the first validation, or create a new Identity. Read-only key permissions are guidance, not inspected ([ADR 0020](../adr/0020-record-capital-flows-manually-with-read-only-credentials.md)).

## Endpoints

| Use | Endpoint | Signed |
| --- | --- | --- |
| Validate: customer reference | Spot `POST /0/private/GetApiKeyInfo`, Futures `GET /api/auth/v1/api-keys/v3/check` | yes |
| Account Structure check; Main in Unified Wallet, Multi-M, Coin-M, Derivatives Holding | Futures WebSocket `wss://futures.kraken.com/ws/v1`, feed `balances`, its first `balances_snapshot` | yes (signed challenge) |
| Main in Separate wallets | Spot `POST /0/private/BalanceEx`, `POST /0/private/TradeBalance` with `asset=ZUSD` | yes |
| Positions, read once per run | Futures `GET /derivatives/api/v3/openpositions` | yes |
| Instruments and mark prices, fetched once per run and only when a position is open | Futures `GET /derivatives/api/v3/instruments?contractType=all`, `GET /derivatives/api/v3/tickers` | no |
| `earn` | Spot `POST /0/private/Earn/Strategies`, following `next_cursor`; `POST /0/private/Earn/Allocations` with `converted_asset=USD` and `hide_zero_allocations=true` | yes |
| Asset names | Spot `GET /0/public/Assets` | no |
| Prices | Spot `GET /0/public/AssetPairs`, `GET /0/public/Ticker` (every pair, one call each, fetched before the Account calls so their time precedes every report) | no |

- Spot private requests are POST because Kraken signs a form body. Each carries a strictly increasing microsecond `nonce`, `API-Key`, and `API-Sign`: the Base64 HMAC-SHA512 of the path and SHA-256(nonce + body), keyed with the Base64-decoded secret.
- Futures private requests carry `APIKey` and `Authent`: the Base64 HMAC-SHA512 of SHA-256(query + path without `/derivatives`), keyed with the decoded Futures secret.
- The balances feed opens with a `challenge` request; the `subscribe` message carries the challenge signed like `Authent`. The adapter reads the first `balances_snapshot`, within 30 messages and 20 seconds, and closes the connection.
- With every Account selected, a Unified Wallet run reads one balances snapshot and makes three signed REST calls (Earn strategies, one more per additional page; Earn allocations; open positions) and three to five public ones. Separate wallets add BalanceEx and TradeBalance.

## Account Structure check

Kraken reports the structure only as `flex_futures.unified_balances` on the balances snapshot: `true` is `kraken_unified`, `false` is `kraken_separate`. A missing flag fails the batch with `normalization`; it is never read as Separate wallets. A structure other than the Identity's fails with `unsupported` at validation when the Identity has a Futures key, and at the start of every Collection Run that selects an Account other than Earn, before any Account is read ([ADR 0025](../adr/0025-re-check-the-account-structure-in-every-collection-run.md)): "Kraken reports unified_balances = true (Unified Wallet), but this Identity is configured as Separate wallets. Create a new Identity for the new structure and archive this one." The Connection stays enabled and its Accounts turn stale. A run with Earn alone reads no snapshot: the Spot key cannot read the feed, and Earn is the same Account in both structures.

## Account Report

| Plutus column | multi-collateral wallet: `main` in Unified Wallet, `multi_m` (`flex_futures`) | `coin_m` | `main` in Separate wallets | `earn`, `derivatives_holding` |
| --- | --- | --- | --- | --- |
| `equity` | `portfolio_value` (USD, with unrealized profit and funding); required | `futures_upnl_cash_value` (USD value of every single-collateral wallet with unrealized profit); required | Σ Balance × USD price + `TradeBalance.n`, see below | null, so the value is `calculated` from Balances |
| `adjusted_equity` | `margin_equity` (after collateral haircuts) | null | null | null |
| `available_equity` | `available_margin` | null | null | null |
| `initial_margin_ratio` | derived: `initial_margin ÷ margin_equity`, 8 decimals; null when `margin_equity ≤ 0` or the field is absent | null | null | null |
| `maintenance_margin_ratio` | derived: `maintenance_margin ÷ margin_equity`, same rules | null | null | null |
| `unified_margin_ratio` | null | null | null | null |
| `observed_at` | the snapshot's `timestamp` | same | local time after BalanceEx | local time (`earn`); the snapshot's `timestamp` (`derivatives_holding`) |

- `portfolio_value` already holds unrealized profit and funding and is used alone. The haircut-adjusted `margin_equity` is only the adjusted equity.
- Main in Separate wallets never uses `TradeBalance.e`, which values the whole balance by Kraken's own rules. Its equity adds `n`, the net result of open Spot Margin positions in USD, to the Balances valued at the prices below. When a Balance has no price and `n = 0`, the equity is null and the value is calculated from Balances, marked incomplete as usual. When `n ≠ 0`, Main fails with `normalization`: a calculated value cannot hold the margin result. Spot Margin positions are not itemized, as with OKX.
- A nonzero `credit_used` on any BalanceEx row fails Main with `unsupported`: Plutus does not support Kraken credit.
- Not stored: `flex_futures` `balance_value`, `collateral_value`, `pnl`, `unrealized_funding`, `total_unrealized`, `total_unrealized_as_margin`, `initial_margin_without_orders`, `total_position_size`, `isolated`, `isolated_list`, and `cross`; the snapshot's `futures_cash_value`; TradeBalance fields other than `n`.

## Balance

| Plutus column | multi-collateral wallet (`flex_futures.currencies`) | `coin_m` (snapshot `futures`) | `derivatives_holding` (snapshot `holding`) | `main` in Separate wallets (`BalanceEx`) | `earn` (`Earn/Allocations`) |
| --- | --- | --- | --- | --- | --- |
| `symbol` | currency code | the wallet's `unit` | currency code | asset code | `native_asset` |
| `equity` (coin units) | `quantity` | `portfolio_value`, summed per unit | the amount, summed per symbol | `balance`, summed with its `.F` row | `amount_allocated.total.native` of `instant`, `bonded`, and `timed` strategies, summed per coin |
| `available_to_trade` | `available` | null | null | null | null |
| other columns | null | null | null | null | null |

- A code Assets lists becomes its `altname` (`XXBT` → `XBT`, `ZUSD` → `USD`), then the aliases `XBT` → `BTC` and `XDG` → `DOGE` apply ([ADR 0018](../adr/0018-merge-asset-listings-by-symbol-with-manual-overrides.md)). Other suffixes, such as `.T` for tokenized assets, are kept. A failed Assets call fails every Account.
- Zero totals are dropped. Every amount is required: a missing or non-decimal one fails its Account with `normalization`, naming the field; it is never read as zero.
- The multi-collateral wallet's derivative profit is in its equity only; it is not split among currencies.
- BalanceEx rows ending in `.B` (yield-bearing), `.S` (staked), and `.M` (opt-in rewards) are Earn allocations and are left to `earn`. `.F` rows are Kraken Rewards (Auto Earn), which earn on Main balances and stay in Main. `hold_trade` is part of `balance` and is not subtracted.
- An Earn allocation's total already holds its bonding, unbonding, exit-queue, and timed amounts; paid rewards are not added again. `flex` strategies are Kraken Rewards, reported in Main and skipped here. Kraken can return historical zero-balance allocations whose strategies are no longer available, so Earn requests `hide_zero_allocations=true` and ignores any zero total still returned. Every total must still be a valid decimal. A nonzero allocation whose strategy Earn/Strategies does not list, or whose lock type is none of `flex`, `instant`, `bonded`, and `timed`, fails Earn with `normalization`, as does an allocation list with a `next_cursor`, which Kraken documents as unpaged. Skipping such a holding would violate the Complete Set rule.
- `name` is never set: these endpoints report no full asset name.

## Position

Futures `openpositions` lists every futures wallet's positions. Flexible futures (`PF_` symbols) belong to the multi-collateral wallet: `main` in Unified Wallet, `multi_m` in Separate wallets. Inverse and vanilla futures belong to `coin_m`.

| Plutus column | Source |
| --- | --- |
| `platform_position_ref` | `<symbol>:<side>`, such as `PF_DOGEUSD:long` |
| `side` | `side`, `long` or `short` |
| `quantity`, `quantity_unit` | `|size|` in `contracts` |
| `entry_price` | `price` |
| `mark_price` | `markPrice` from Futures tickers |
| `unrealized_pnl` | `unrealizedPnl` when `pnlCurrency` is the settlement coin, or a USD equivalent for a USD-settled contract; a flexible position without `pnlCurrency` reports USD. Null otherwise |
| `liquidation_price`, `leverage`, `margin` | not mapped |

- Rows with `size = 0` are dropped.
- An open option position fails the multi-collateral report with `normalization`: Plutus has no Instrument kind for options.
- A position whose contract is missing from the instrument list, or has an undocumented type, fails its report with `normalization`; it is never dropped.

| Instrument column | Source (`instruments`) |
| --- | --- |
| `symbol` | `symbol` |
| `kind` | `flexible_futures` and `futures_vanilla` → `linear_perpetual`, `futures_inverse` → `inverse_perpetual`; with a `lastTradingTime`, the `*_delivery` equivalent |
| `base_symbol`, `quote_symbol` | `base` and `quote`, else the two halves of `pair` (`XBT:USD`), normalized as Balance symbols |
| `settlement_symbol` | USD for flexible futures, the base coin for inverse futures, the quote for vanilla futures |
| `expiry_at` | `lastTradingTime`; a contract without one whose ticker `tag` is not `perpetual` fails with `normalization` |
| `contract_size` | `contractSize`, required: base coins per contract for linear contracts, USD per contract for inverse ones |

## Prices

For every Balance symbol other than USD, USDT, and USDC, the Spot last trade price (`c[0]`) of its USD pair, else its USDT pair, else its USDC pair, observed when the tickers were fetched. USD, USDT, and USDC get no price row ([ADR 0017](../adr/0017-value-in-usd-and-treat-major-stablecoins-as-usd.md)). A symbol with none of the three pairs gets no price and is valued as `no_price`. A failed AssetPairs or Ticker call leaves the batch without prices, and valuation falls back to the latest price within 24 hours. The same prices value Main in Separate wallets. Positions carry their own mark price; contracts get no separate price.

## Earn completeness

Kraken is not checked against an Earn valuation of its own ([ADR 0029](../adr/0029-count-dual-investment-in-earn-and-check-earn-completeness.md)): the only USD figure it reports, `total_allocated` on Earn/Allocations, values the same allocation list Plutus itemizes, including Kraken Rewards that stay in Main. The list is documented as complete and unpaged, and a nonzero allocation that cannot be classified fails Earn instead of being skipped.

## Errors

| Kraken answer | Class | Scope |
| --- | --- | --- |
| Spot `EAPI:Invalid key`, which Kraken also answered to a wrong secret when recorded, and `EAPI:Invalid signature`; Futures `authenticationError`; HTTP 401, which the key check answers for an unknown key (`api_key_not_found`) or a wrong signature (`request_signature_invalid`); a secret that is not Base64; keys of different customers; a key group the selected Accounts need is missing | `authentication` | the whole batch; the Connection becomes `invalid` |
| Spot `EGeneral:Permission denied`; HTTP 403; a refused balances subscription | `authorization` | the whole batch; the Connection becomes `invalid` |
| Spot `EAPI:Rate limit exceeded` (and the `EOrder` and `EAuth` variants), `EAuth:Too many requests`, `EService:Throttled`; Futures `apiLimitExceeded`; HTTP 429 | `rate_limited` | the Account; the batch when every Account failed |
| `EAPI:Invalid nonce`, `EService:Unavailable`, HTTP 5xx, timeouts, network errors, a balances feed that ends before its snapshot | `temporary` | the Account, or the batch for the snapshot; the batch when every Account failed |
| any other error | `temporary`, detail names the endpoint and HTTP status | the Account |
| the other Account Structure; an Account Type the structure lacks; used credit on Main in Separate wallets | `unsupported` | the whole batch for the structure, otherwise the Account; not retried |
| a missing `unified_balances`; unparseable or non-decimal fields; a missing required field; unknown contracts; option positions in the multi-collateral wallet; unclassified Earn allocations; a Spot Margin result beside an unpriced Balance | `normalization` | the Account, or the batch for the structure flag |

Kraken answers many Spot failures with HTTP 200 and an `error` list, so both the status and the list decide. Details name the endpoint, the HTTP status, and the matched error name ("Invalid key.", or "API key not found." for the key check's 401), never Kraken's text, which can echo the request. On a 401, 403, 429, or 5xx the status decides the class, and a documented error name in the body only sharpens the detail.

## Fixtures

`plutus adapter record --platform kraken` reads `PLUTUS_GOTEST_KRAKEN_SPOT_APIKEY` / `_SPOT_SECRET` and `PLUTUS_GOTEST_KRAKEN_FUTURES_APIKEY` / `_FUTURES_SECRET` from the environment or `config/gotest.env`; both groups are required. It reads the Account Structure from the balances feed, validates, collects every Account Type of that structure, and on a Unified Wallet account also calls BalanceEx and TradeBalance. It writes the last answer of each endpoint to `tmp/fixtures/kraken/<endpoint>.json`, with a header naming the structure read from the account, `scenario: recorded`, and the recording date, so BalanceEx and TradeBalance recorded on a Unified Wallet account do not claim to prove the Separate mapping. Private holdings are kept complete and as recorded. The recorder:

- trims Assets to the recorded coins plus BTC, ETH, USD, USDT, and USDC, and AssetPairs and Ticker to the USD, USDT, and USDC pairs of those coins;
- trims Futures instruments and tickers to held contracts plus the BTC and ETH futures, dropping unheld options, and Earn strategies to those of recorded allocations plus the first strategy of each lock type;
- replaces customer and key identifiers (`iban`, `iiban`, `accountUid`, `account`, `api_key`, `apiKey`, `api_key_name`, `apiKeyName`), IP restrictions (`ip_allowlist`, `ipAllowlist`, `allowedCidrBlock`, `allowedCidrBlocks`), and key dates (`created_time`, `modified_time`, `last_used`, `createdAt`) with `REDACTED`, `100001` for a number, or an empty list, and any echo of a credential or identifier inside another string with `REDACTED`. A null or an empty list names nothing and is kept. Each header lists what was replaced, and nothing is written if a credential or identifier survives.

It then records five refusals, each provoked in one request that Kraken refuses before reading anything, written as `<endpoint>.<scenario>.json` with that scenario in the header and echoes redacted: on `GetApiKeyInfo`, a key no account has (`invalid_key`), a wrong secret (`bad_signature`), and nonce `1` (`invalid_nonce`); on the Futures key check, `invalid_key` and `bad_signature`. The recording fails if Kraken accepts one of them. Permission, rate-limit, and service errors cannot be provoked safely: `testdata/documented/errors/` holds those answers as Kraken documents them, on Earn/Allocations and open positions, each noting the HTTP status it assumes.

After review the files move to `internal/adapters/kraken/testdata/`, where `httptest` replays them. The replay checks every key, the Spot `API-Sign` and nonce, the Futures `Authent`, and the signed WebSocket challenge, and answers a request Kraken would refuse with the matching error answer. Every refusal must be present, and every error answer must map to its class. The committed set, refusals included, comes from a Unified Wallet account recorded on 2026-09-25.

Separate wallets have no recording. `testdata/documented/kraken_separate/` holds six responses built from Kraken's documented examples: the balances snapshot, BalanceEx, TradeBalance, open positions, instruments, and tickers. Each header has `scenario: documented`, no recording date, its documentation URL, `accountStructure: kraken_separate`, and a `trimmed` note naming every change, such as `unified_balances = false` added to the snapshot, invented `.F`, `.B`, and zero BalanceEx rows, and contracts renamed so every position resolves. The replay lays them over the recorded set, which still supplies the key checks, asset names, prices, and Earn. They prove the field mapping of the documented shapes (Main from BalanceEx with a Spot Margin result, Multi-M, a nonempty Coin-M with inverse perpetual and delivery positions, and a ten-coin holding wallet), not live access.

## Coverage

The live smoke test reads the structure from the account, validates it with both key groups and with the Futures pair alone, confirms that the other structure is refused at validation and collection, and collects every Account Type of that structure, asserting structure and success, never numbers. On 2026-09-25 the test account had Unified Wallet: Main held two currencies and one flexible futures position, Earn one coin, Derivatives Holding two coins, and Coin-M was empty; its Futures key check carried `iiban`, so the Futures pair alone reached the same reference.

Not covered live: Separate wallets (Main from BalanceEx, Multi-M), a nonempty Coin-M, inverse and delivery positions, a nonzero Spot Margin result, BalanceEx `.F`, `.B`, `.S`, and `.M` rows, and a Futures key check without `iiban`.

## Comparison checklist (test environment)

Run on the test environment against Kraken Pro on the same account, within a few minutes of a collection:

1. **Identity**: validation stores the public account ID shown in Kraken's account settings, without spaces.
2. **Structure**: the Identity's structure matches the Unified Wallet setting in Kraken Pro; an Identity with the other structure is refused at validation, naming `unified_balances`.
3. **Multi-collateral wallet** (Main in Unified Wallet, Multi-M in Separate wallets): `Equity` equals the wallet's portfolio value in USD, `AdjEq` its margin equity, and `AvailEq` its available margin; the margin ratios equal initial and maintenance margin ÷ margin equity; each currency's quantity matches the wallet.
4. **Positions**: every open flexible futures position appears under that wallet with the same side, size in contracts, entry price, and mark price; inverse and dated contracts appear under Coin-M.
5. **Coin-M**: each single-collateral wallet's balance in its own coin matches, and `Equity` equals their USD value with unrealized profit.
6. **Derivatives Holding**: coin amounts match the holding wallet.
7. **Earn**: allocated amounts per coin match the Earn page, including bonding and unbonding amounts; Kraken Rewards (Auto Earn) stays in Main and does not appear under Earn.
8. **Separate wallets**: on a Separate-wallet account, Main's coins match the Spot wallet, with Kraken Rewards and without Earn allocations, and its equity equals those coins at their USD prices plus the open Spot Margin result; repeat 3–7.
9. **Structure switch**: after Unified Wallet is switched on or off, the next cycle fails with `unsupported` naming both structures and the Accounts turn stale; a new Identity with the new structure collects normally.
10. **Failures**: revoking either key sets the Connection `invalid` at the next cycle.
