# Kraken verification

Current evidence and pending exchange-app checks. See the
[Adapter mapping](./mapping.md) for field semantics,
[evidence workflow](../evidence-workflow.md) for recording and promotion, and
[Earn delivery report](../earn-presentation-delivery.md) for metadata coverage.

## Current evidence

**11 reviewed responses**, recorded and owner-reviewed on 2026-10-01. The owner
declares Unified Wallet; WS independently confirms `unified_balances=true`.
Cross and isolated wallet settings are not Identity Margin Modes. Spot
`query-funds` and Futures `READ_ONLY` / `NO_ACCESS` permissions are directly
observed; separately masked references do not prove shared credential ownership.

Samples cover nonzero Unified Main, Coin-M, and Holding wallets; linear
perpetual/delivery and inverse perpetual positions; and instant, bonded, and
timed USDT Earn. They are interface evidence, not a complete Account Report or
an atomic asset-total proof. Plutus does not collect Kraken Dual Investment; the
public API contract gap is described below.

## Dual Investment contract review

Reviewed on 2026-10-05. Kraken offers
[Dual Investments](https://support.kraken.com/articles/kraken-pro-dual-investments)
on Kraken Pro. Its product guide describes invested principal, an alternate
payout Asset, a trigger price, a fixed APR, and settlement to the Spot balance.
Product availability does not establish an API contract.

The published
[Spot REST OpenAPI](https://docs.kraken.com/openapi/spot-rest.yaml) and
[Futures REST OpenAPI](https://docs.kraken.com/openapi/futures-rest.yaml),
together with the [developer index](https://docs.kraken.com/llms.txt) and
[sitemap](https://docs.kraken.com/sitemap.xml), did not expose a Dual Investment
subscription or holdings contract in this review. This is a finding about the
reviewed public documentation, not a claim that no private interface exists.

The adjacent interfaces do not establish a substitute:

- [Earn Strategies](https://docs.kraken.com/api-reference/earn/list-earn-strategies)
  and
  [Earn Allocations](https://docs.kraken.com/api-reference/earn/list-earn-allocations)
  describe strategies and allocated amounts. Their published contracts do not
  identify Dual subscriptions or their paired Assets and trigger prices. A
  `timed` strategy alone does not identify a Dual Investment.
- [BalanceEx](https://docs.kraken.com/api-reference/account-data/get-extended-balance)
  documents held amounts for Spot non-margin orders; `hold_trade` is not a
  documented Dual principal source.
- [List Wallet Accounts](https://docs.kraken.com/api-reference/account-data/list-wallet-accounts)
  lists wallet identifiers. It does not document Dual subscription terms or
  establish whether invested principal is included in Main or Earn.

Implementation is blocked on a documented read-only contract: service and
authentication permissions; subscription identity and principal Asset/amount;
pagination and complete-set guarantees; running, settlement, and pending-payout
states; and overlap with Main, Earn, and any combined aggregate. Available
product terms must also have documented field meanings and units. These facts
are needed to avoid omitted or double-counted principal and premature removal of
holdings.

The owner purchased a 100 USD Dual Investment. On 2026-10-05, explicit recording
with the existing recording credentials checked the current interfaces:

| Capture (UTC) | Interface                       | Observed result                                            |
| ------------- | ------------------------------- | ---------------------------------------------------------- |
| 08:02:47      | Unified Main WebSocket balances | USD 33.7841; Main portfolio value 158.15 USD               |
| 08:03:24      | Spot BalanceEx                  | ZUSD 33.7841; `hold_trade` 0                               |
| 08:03:57      | Earn Allocations                | Three USDT allocations; no USD principal; null next cursor |

Together with the owner's subscription confirmation, these observations show
that Main and Earn currently omit that 100 USD principal. Main's aggregate
includes other assets and is not the subscription amount. Manual can supplement
the omission, with user cleanup after settlement. This narrow principal-coverage
check does not complete exchange-app totals, simultaneous wallet reconciliation,
product lifecycle, or settlement validation.

The sanitized review groups remain under `/tmp/plutus/kraken-dual-check/kraken/`
with group IDs `20261005T080247Z-2728805597`, `20261005T080324Z-1206188283`, and
`20261005T080357Z-2917705898`. They have not been promoted into repository
evidence, and do not replace or upgrade the 2026-10-01 accepted responses.
Temporary files are not durable evidence links. No guessed Dual endpoint,
fixture, or selectable exchange Account was added.

## Important observations

- WS includes undocumented `isolated_list`. Flexible delivery belongs to Unified
  Main rather than Coin-M; all held contracts have metadata and mark-price rows.
- Position `fillTime` is an epoch placeholder, not a known entry time. Flexible
  delivery funding is null. Inverse position `pnlCurrency` is absent, so the
  mapper leaves its PnL unset rather than inventing a settlement currency.
- Earn has `amount_allocated.timed`; mapping uses `total.native`. The strategy
  schema lists timed even though its introductory explanation is incomplete. A
  listed flex strategy does not prove nonzero Auto Earn. Null first-page cursors
  do not establish pagination boundaries.
- Option and inverse-delivery metadata establish no nonempty holdings in those
  products. No vanilla futures contract was returned; options remain
  unsupported.

## Sanitation and replacements

Keys, customer references, labels, and IP allowlists are masked; strategy IDs
and public market identifiers remain intact. Whole-row market trimming preserves
held contracts and required aliases. Local review also trimmed strategies to
24/403 and flex currencies to 14/69, keeping allocated IDs, every nonzero
financial row, and representative field and setting shapes. Those local review
changes and the exact rules/counts are disclosed in file notes.

Retained rows, original aggregates, precision, nulls, and empty fields remain
unchanged. No accepted evidence was replaced. Trimming made no extra exchange
requests.

## Remaining gaps

- Separate-wallet WS, BalanceEx, and TradeBalance have no real samples. The
  owner has no Separate account and defers further account preparation.
- Nonzero Auto Earn flex allocation, bonding, ETH exit queue, and non-USDT Earn
  allocation remain unobserved.
- Inverse delivery and single-collateral vanilla positions remain unobserved;
  metadata alone does not close these gaps.
- Spot Margin debt and its separate net result are not established by the
  Unified sample. Null-iiban fallback, mismatched ownership, pagination
  boundaries, and refusal/error responses remain gaps.
- Exchange-app comparison and complete asset-total reconciliation remain
  pending.

## Interface references

Each interface links its contract, independent synthetic inputs, and reviewed
recordings. Coverage notes below qualify those samples; exact UTC capture times
and trimming details remain in each recording's `recordedAt` and `notes`. Link
labels name file variants; `sample` denotes an unsuffixed file. Sampling
settings do not limit an interface's applicability.

### [`futures: GET /derivatives/api/v3/instruments`](https://docs.kraken.com/api-reference/instrument-details/get-instruments)

- Synthetic:
  [sample](../../../internal/platform/kraken/testdata/derivatives_api_v3_instruments.json).
- Recorded:
  [sample](../../../internal/platform/kraken/evidence/derivatives_api_v3_instruments.json);
  18 of 424 rows; all four held contracts; flexible perpetual/delivery and
  inverse contracts; one option metadata row does not add support or nonempty
  coverage

### [`futures: GET /derivatives/api/v3/openpositions`](https://docs.kraken.com/api-reference/account-information/get-open-positions)

- Synthetic:
  [api_limit_exceeded](../../../internal/platform/kraken/testdata/derivatives_api_v3_openpositions.api_limit_exceeded.json),
  [authentication_error](../../../internal/platform/kraken/testdata/derivatives_api_v3_openpositions.authentication_error.json),
  [sample](../../../internal/platform/kraken/testdata/derivatives_api_v3_openpositions.json).
- Recorded:
  [sample](../../../internal/platform/kraken/evidence/derivatives_api_v3_openpositions.json);
  Four nonzero long/short positions: PF DOGE/ETH, FF ETH delivery, PI ETH; epoch
  fillTime; inverse pnlCurrency absent; delivery funding null

### [`futures: GET /derivatives/api/v3/tickers`](https://docs.kraken.com/api-reference/market-data/get-tickers)

- Synthetic:
  [sample](../../../internal/platform/kraken/testdata/derivatives_api_v3_tickers.json).
- Recorded:
  [sample](../../../internal/platform/kraken/evidence/derivatives_api_v3_tickers.json);
  11 of 300 rows; all four held contracts retain markPrice and
  delivery/perpetual tags

### [`futures-auth: GET /api/auth/v1/api-keys/v3/check`](https://docs.kraken.com/exchange/guides/general/errors)

- Synthetic:
  [bad_signature](../../../internal/platform/kraken/testdata/api_auth_v1_api_keys_v3_check.bad_signature.json),
  [invalid_key](../../../internal/platform/kraken/testdata/api_auth_v1_api_keys_v3_check.invalid_key.json),
  [sample](../../../internal/platform/kraken/testdata/api_auth_v1_api_keys_v3_check.json).
- Recorded:
  [sample](../../../internal/platform/kraken/evidence/api_auth_v1_api_keys_v3_check.json);
  READ_ONLY / NO_ACCESS; nonempty iiban and accountUid are masked; ownership
  across separate captures is not proven

### [`futures-ws: WS /ws/v1/balances_snapshot`](https://docs.kraken.com/exchange/api-reference/futures-websocket/balances)

- Synthetic:
  [separate](../../../internal/platform/kraken/testdata/ws_v1_balances_snapshot.separate.json),
  [unified](../../../internal/platform/kraken/testdata/ws_v1_balances_snapshot.unified.json).
- Recorded:
  [unified](../../../internal/platform/kraken/evidence/ws_v1_balances_snapshot.unified.json);
  unified_balances=true; nonzero Main/Coin-M/Holding; cross and isolated fields
  coexist; additional isolated_list array

### [`spot: GET /0/public/AssetPairs`](https://docs.kraken.com/api-reference/market-data/get-tradable-asset-pairs)

- Synthetic:
  [sample](../../../internal/platform/kraken/testdata/0_public_AssetPairs.json).
- Recorded:
  [sample](../../../internal/platform/kraken/evidence/0_public_AssetPairs.json);
  41 of 1455 rows; ETH/DOGE/XRP/USDT/BTC USD pairs and representative
  quotes/field shapes

### [`spot: GET /0/public/Assets`](https://docs.kraken.com/api-reference/market-data/get-asset-info)

- Synthetic:
  [sample](../../../internal/platform/kraken/testdata/0_public_Assets.json).
- Recorded:
  [sample](../../../internal/platform/kraken/evidence/0_public_Assets.json); 9
  of 852 rows; held currencies and legacy asset aliases preserved

### [`spot: GET /0/public/Ticker`](https://docs.kraken.com/api-reference/market-data/get-ticker-information)

- Synthetic:
  [sample](../../../internal/platform/kraken/testdata/0_public_Ticker.json).
- Recorded:
  [sample](../../../internal/platform/kraken/evidence/0_public_Ticker.json); 6
  of 1484 rows; corresponding held-asset USD pairs and a representative field
  shape

### [`spot: POST /0/private/BalanceEx`](https://docs.kraken.com/api-reference/account-data/get-extended-balance)

- Synthetic:
  [separate](../../../internal/platform/kraken/testdata/0_private_BalanceEx.separate.json),
  [unified](../../../internal/platform/kraken/testdata/0_private_BalanceEx.unified.json).
- Recorded: Missing: no Separate account; not requested in the Unified round

### [`spot: POST /0/private/Earn/Allocations`](https://docs.kraken.com/api-reference/earn/list-earn-allocations)

- Synthetic:
  [sample](../../../internal/platform/kraken/testdata/0_private_Earn_Allocations.json),
  [permission_denied](../../../internal/platform/kraken/testdata/0_private_Earn_Allocations.permission_denied.json),
  [rate_limited](../../../internal/platform/kraken/testdata/0_private_Earn_Allocations.rate_limited.json),
  [throttled](../../../internal/platform/kraken/testdata/0_private_Earn_Allocations.throttled.json),
  [too_many_requests](../../../internal/platform/kraken/testdata/0_private_Earn_Allocations.too_many_requests.json),
  [unavailable](../../../internal/platform/kraken/testdata/0_private_Earn_Allocations.unavailable.json).
- Recorded:
  [sample](../../../internal/platform/kraken/evidence/0_private_Earn_Allocations.json);
  Four nonzero USDT allocations; instant, bonded/unbonding and timed 6/18
  months; next_cursor null

### [`spot: POST /0/private/Earn/Strategies`](https://docs.kraken.com/api-reference/earn/list-earn-strategies)

- Synthetic:
  [sample](../../../internal/platform/kraken/testdata/0_private_Earn_Strategies.json).
- Recorded:
  [sample](../../../internal/platform/kraken/evidence/0_private_Earn_Strategies.json);
  24 of 403 rows; every allocated strategy; flex/instant/bonded/timed types and
  representative terms; next_cursor null

### [`spot: POST /0/private/GetApiKeyInfo`](https://docs.kraken.com/exchange/guides/general/errors)

- Synthetic:
  [bad_signature](../../../internal/platform/kraken/testdata/0_private_GetApiKeyInfo.bad_signature.json),
  [invalid_key](../../../internal/platform/kraken/testdata/0_private_GetApiKeyInfo.invalid_key.json),
  [invalid_nonce](../../../internal/platform/kraken/testdata/0_private_GetApiKeyInfo.invalid_nonce.json),
  [sample](../../../internal/platform/kraken/testdata/0_private_GetApiKeyInfo.json).
- Recorded:
  [sample](../../../internal/platform/kraken/evidence/0_private_GetApiKeyInfo.json);
  query-funds only; key, name, iban and IP allowlist masked

### [`spot: POST /0/private/TradeBalance`](https://docs.kraken.com/api-reference/account-data/get-trade-balance)

- Synthetic:
  [separate](../../../internal/platform/kraken/testdata/0_private_TradeBalance.separate.json),
  [unified](../../../internal/platform/kraken/testdata/0_private_TradeBalance.unified.json).
- Recorded: Missing: no Separate account; not requested in the Unified round

## Exchange-app validation checklist

Status: pending. This is an acceptance procedure, not a record of completed
checks. Recorded evidence and synthetic tests do not establish a reconciled
Account total. Human operators execute it on the remote test environment; agent
sessions remain limited to the isolated development instance.

Run on the test environment against Kraken Pro on the same account, within a few
minutes of a collection:

1. **Identity**: validation stores the public account ID shown in Kraken's
   account settings, without spaces.
2. **Structure**: the Identity's structure matches the Unified Wallet setting in
   Kraken Pro; an Identity with the other structure is refused at validation,
   naming `unified_balances`.
3. **Multi-collateral wallet** (Main in Unified Wallet, Multi-M in Separate
   wallets): `Equity` equals the wallet's portfolio value in USD, `AdjEq` its
   margin equity, and `AvailEq` its available margin; the margin ratios equal
   initial and maintenance margin ÷ margin equity; each currency's quantity
   matches the wallet.
4. **Positions**: every open flexible futures position, including delivery
   contracts, appears under that wallet with Both and signed Base Size, entry
   price, and mark price; inverse and vanilla contracts, perpetual or delivery,
   appear under Coin-M with signed Quote face Size. Source quantities remain
   contract counts. Isolated Margin excludes PnL and Cross wallet risk is not
   assigned to individual Positions.
5. **Coin-M**: each single-collateral wallet's balance in its own coin matches,
   and `Equity` equals their USD value with unrealized profit.
6. **Derivatives Holding**: coin amounts match the holding wallet.
7. **Earn**: allocated amounts per coin match the Earn page, including bonding
   and unbonding amounts; Kraken Rewards (Auto Earn) stays in Main and does not
   appear under Earn.
8. **Separate wallets**: on a Separate-wallet account, Main's coins match the
   Spot wallet, with Kraken Rewards and without Earn allocations, and its equity
   equals those coins at their USD prices plus the open Spot Margin result;
   repeat 3–7.
9. **Structure switch**: after Unified Wallet is switched on or off, the next
   cycle fails with `unsupported` naming both structures and the Accounts turn
   stale; a new Identity with the new structure collects normally.
10. **Failures**: revoking either key sets the Connection `invalid` at the next
    cycle.
