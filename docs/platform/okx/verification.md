# OKX verification

Current evidence and pending exchange-app checks. See the
[Adapter mapping](./mapping.md) for field semantics,
[evidence workflow](../evidence-workflow.md) for recording and promotion, and
[Earn delivery report](../earn-presentation-delivery.md) for metadata coverage.

## Current evidence

**20 reviewed responses**, recorded and owner-reviewed on 2026-10-01: 12
PM/shared, three MCM, three Futures, and two borrowing-enabled Spot responses.
Configuration directly observes `acctLv` 4, 3, 2, and 1 respectively and
read-only permissions. Main-account ownership remains owner-declared.

PM/MCM observe `net_mode`; Futures observes `long_short_mode`,
`mgnIsoMode=auto_transfers_ccy`, and `ctIsoMode=automatic`. PM/MCM include
linear/inverse perpetual/delivery cross positions. Futures includes isolated
same-contract Hedge, cross inverse perpetual, and isolated MARGIN borrowing.
Spot directly observes enabled borrowing with distinct cash, equity, and
liability. Generic wallet, product, and market responses are shared references,
not proof of all account/mode combinations.

## Important observations

- USD-settled linear-delivery metadata has `settleCcy=USD`, `ctVal=0.001`, and
  `ctValCcy=ETH`, while the account position has `ccy=USDC`. Preserve both
  without substituting one currency for the other.
- Public time uses a numeric `code`; other responses use strings. Futures Hedge
  sizes are positive on both sides, so `posSide` supplies direction.
  Inapplicable account and currency fields are blank, not zero. Runtime
  deliberately omits MARGIN rows from contract Positions.
- PM cross leverage is zero with blank liquidation price; MCM has populated
  leverage and both blank and populated liquidation fields.
- Savings has USDT/USDC; Dual history includes both LIVE directions and SETTLED
  fields. On-chain Earn observes only `state=8`. Funding includes BETH, which
  does not imply another ETH staking Balance.

## Sanitation and replacements

Configuration UID, mainUID, IP, and label are masked. Market trimming preserves
whole rows, original counts, and held instruments. No accepted evidence was
replaced. Corrected Futures positions replaced a temporary candidate missing
SHORT before promotion; the accepted balance predates the added SHORT and is not
a matching snapshot. Responses from separate requests are not an atomic
aggregate proof.

## Remaining gaps

- Borrowing-disabled Spot is owner-deferred because no separate disable control
  was found. Enabled borrowing is this account's observed baseline, not a
  universal default.
- Nonzero accrued interest, other spot-margin collateral/mode settings, and
  unobserved contract settings remain gaps.
- On-chain Earn lifecycle states beyond the observed state and difficult
  pagination, transient, and refusal responses remain gaps.
- Futures mode's perpetual samples do not imply observed delivery positions in
  that mode. Exchange-app comparison and complete asset-total reconciliation
  remain pending.

## Interface references

Synthetic inputs and recorded evidence are listed separately. Recorded envelopes
retain exact UTC timestamps and trimming notes; sampling settings do not limit
an interface's applicability.

### `v5: GET /api/v5/account/balance`

[Contract](https://www.okx.com/docs-v5/en/#trading-account-rest-api-get-balance)

Synthetic inputs:

- [api_v5_account_balance.futures.json](../../../internal/platform/okx/testdata/api_v5_account_balance.futures.json)
- [api_v5_account_balance.mcm.json](../../../internal/platform/okx/testdata/api_v5_account_balance.mcm.json)
- [api_v5_account_balance.pm.json](../../../internal/platform/okx/testdata/api_v5_account_balance.pm.json)
- [api_v5_account_balance.spot.json](../../../internal/platform/okx/testdata/api_v5_account_balance.spot.json)

Recorded evidence:

[api_v5_account_balance.pm.json](../../../internal/platform/okx/evidence/api_v5_account_balance.pm.json);
PM; four currencies, nonzero HYPE/USDC liabilities; interest is zero;
[api_v5_account_balance.mcm.json](../../../internal/platform/okx/evidence/api_v5_account_balance.mcm.json),
MCM; delta fields empty, nonzero HYPE/USDC liabilities, interest zero;
[api_v5_account_balance.futures.json](../../../internal/platform/okx/evidence/api_v5_account_balance.futures.json),
Futures; nonzero equity/isoEq, account margin and currency liability fields
empty;
[api_v5_account_balance.spot.json](../../../internal/platform/okx/evidence/api_v5_account_balance.spot.json),
Spot with USDT/ETH balances and HYPE liab=0.2; cash/equity distinct, optional
blanks preserved

### `v5: GET /api/v5/account/config`

[Contract](https://www.okx.com/docs-v5/en/#error-code-rest-api)

Synthetic inputs:

- [api_v5_account_config.bad_signature.json](../../../internal/platform/okx/testdata/api_v5_account_config.bad_signature.json)
- [api_v5_account_config.futures.json](../../../internal/platform/okx/testdata/api_v5_account_config.futures.json)
- [api_v5_account_config.invalid_key.json](../../../internal/platform/okx/testdata/api_v5_account_config.invalid_key.json)
- [api_v5_account_config.mcm.json](../../../internal/platform/okx/testdata/api_v5_account_config.mcm.json)
- [api_v5_account_config.pm.json](../../../internal/platform/okx/testdata/api_v5_account_config.pm.json)
- [api_v5_account_config.spot.json](../../../internal/platform/okx/testdata/api_v5_account_config.spot.json)
- [api_v5_account_config.stale_timestamp.json](../../../internal/platform/okx/testdata/api_v5_account_config.stale_timestamp.json)
- [api_v5_account_config.wrong_passphrase.json](../../../internal/platform/okx/testdata/api_v5_account_config.wrong_passphrase.json)

Recorded evidence:

[api_v5_account_config.pm.json](../../../internal/platform/okx/evidence/api_v5_account_config.pm.json);
PM acctLv=4, net_mode, read_only; identity/IP/label masked;
[api_v5_account_config.mcm.json](../../../internal/platform/okx/evidence/api_v5_account_config.mcm.json),
MCM acctLv=3, net_mode, read_only;
[api_v5_account_config.futures.json](../../../internal/platform/okx/evidence/api_v5_account_config.futures.json),
Futures acctLv=2, long_short_mode, auto_transfers_ccy;
[api_v5_account_config.spot.json](../../../internal/platform/okx/evidence/api_v5_account_config.spot.json),
Spot acctLv=1, enableSpotBorrow=true, read_only

### `v5: GET /api/v5/account/positions`

[Contract](https://www.okx.com/docs-v5/en/#trading-account-rest-api-get-positions)

Synthetic inputs:

- [api_v5_account_positions.futures.json](../../../internal/platform/okx/testdata/api_v5_account_positions.futures.json)
- [api_v5_account_positions.mcm.json](../../../internal/platform/okx/testdata/api_v5_account_positions.mcm.json)
- [api_v5_account_positions.pm.json](../../../internal/platform/okx/testdata/api_v5_account_positions.pm.json)
- [api_v5_account_positions.spot.json](../../../internal/platform/okx/testdata/api_v5_account_positions.spot.json)

Recorded evidence:

[api_v5_account_positions.pm.json](../../../internal/platform/okx/evidence/api_v5_account_positions.pm.json);
Five nonzero cross/net positions; linear/inverse perpetual/delivery; lever=0,
liqPx empty;
[api_v5_account_positions.mcm.json](../../../internal/platform/okx/evidence/api_v5_account_positions.mcm.json),
MCM; five cross/net positions, lever=3; liqPx populated on three rows and empty
on two;
[api_v5_account_positions.futures.json](../../../internal/platform/okx/evidence/api_v5_account_positions.futures.json),
Futures; same-contract isolated LONG/SHORT, cross inverse SHORT, isolated MARGIN
HYPE liability

### `v5: GET /api/v5/asset/asset-valuation`

[Contract](https://www.okx.com/docs-v5/en/#funding-account-rest-api-get-account-asset-valuation)

Synthetic inputs:

- [api_v5_asset_asset_valuation.json](../../../internal/platform/okx/testdata/api_v5_asset_asset_valuation.json)

Recorded evidence:

[api_v5_asset_asset_valuation.json](../../../internal/platform/okx/evidence/api_v5_asset_asset_valuation.json);
ccy=USDT; Earn/Funding/Trading totals, no aggregate reconciliation

### `v5: GET /api/v5/asset/balances`

[Contract](https://www.okx.com/docs-v5/en/#funding-account-rest-api-get-balance)

Synthetic inputs:

- [api_v5_asset_balances.json](../../../internal/platform/okx/testdata/api_v5_asset_balances.json)

Recorded evidence:

[api_v5_asset_balances.json](../../../internal/platform/okx/evidence/api_v5_asset_balances.json);
Five nonzero balances including BETH; small ENS/NIGHT retained

### `v5: GET /api/v5/finance/savings/balance`

[Contract](https://www.okx.com/docs-v5/en/#financial-product-simple-earn-flexible-get-saving-balance)

Synthetic inputs:

- [api_v5_finance_savings_balance.json](../../../internal/platform/okx/testdata/api_v5_finance_savings_balance.json)

Recorded evidence:

[api_v5_finance_savings_balance.json](../../../internal/platform/okx/evidence/api_v5_finance_savings_balance.json);
USDT/USDC flexible principal, earnings, loan/pending amounts

### `v5: GET /api/v5/finance/sfp/dcd/order-history`

[Contract](https://www.okx.com/docs-v5/en/#financial-product-dual-investment-get-order-history)

Synthetic inputs:

- [api_v5_finance_sfp_dcd_order_history.json](../../../internal/platform/okx/testdata/api_v5_finance_sfp_dcd_order_history.json)
- [api_v5_finance_sfp_dcd_order_history.put.json](../../../internal/platform/okx/testdata/api_v5_finance_sfp_dcd_order_history.put.json)

Recorded evidence:

[api_v5_finance_sfp_dcd_order_history.json](../../../internal/platform/okx/evidence/api_v5_finance_sfp_dcd_order_history.json);
limit=100; LIVE Buy Low/Sell High together and a SETTLED product

### `v5: GET /api/v5/finance/staking-defi/orders-active`

[Contract](https://www.okx.com/docs-v5/en/#financial-product-on-chain-earn-get-active-orders)

Synthetic inputs:

- [api_v5_finance_staking_defi_orders_active.json](../../../internal/platform/okx/testdata/api_v5_finance_staking_defi_orders_active.json)

Recorded evidence:

[api_v5_finance_staking_defi_orders_active.json](../../../internal/platform/okx/evidence/api_v5_finance_staking_defi_orders_active.json);
USDT investData amt=5, defi, state=8

### `v5: GET /api/v5/market/tickers`

[Contract](https://www.okx.com/docs-v5/en/#order-book-trading-market-data-get-tickers)

Synthetic inputs:

- [api_v5_market_tickers.json](../../../internal/platform/okx/testdata/api_v5_market_tickers.json)

Recorded evidence:

[api_v5_market_tickers.json](../../../internal/platform/okx/evidence/api_v5_market_tickers.json);
instType=SPOT; four of 1148 rows retained

### `v5: GET /api/v5/public/instruments`

[Contract](https://www.okx.com/docs-v5/en/#public-data-rest-api-get-instruments)

Synthetic inputs:

- [api_v5_public_instruments.futures.json](../../../internal/platform/okx/testdata/api_v5_public_instruments.futures.json)
- [api_v5_public_instruments.swap.json](../../../internal/platform/okx/testdata/api_v5_public_instruments.swap.json)

Recorded evidence:

[swap](../../../internal/platform/okx/evidence/api_v5_public_instruments.swap.json),
[futures](../../../internal/platform/okx/evidence/api_v5_public_instruments.futures.json);
5/494 and 4/254 rows retained, including every held contract

### `v5: GET /api/v5/public/time`

[Contract](https://www.okx.com/docs-v5/en/#public-data-rest-api-system-time)

Synthetic inputs:

- [api_v5_public_time.json](../../../internal/platform/okx/testdata/api_v5_public_time.json)

Recorded evidence:

[api_v5_public_time.json](../../../internal/platform/okx/evidence/api_v5_public_time.json);
Numeric code 0, unlike string code on the other eleven responses

## Exchange-app validation checklist

Status: pending. This is an acceptance procedure, not a record of completed
checks. Recorded evidence and synthetic tests do not establish a reconciled
Account total. Human operators execute it on the remote test environment; agent
sessions remain limited to the isolated development instance.

Run on the test environment against the OKX app on the same account, within a
few minutes of a collection.

**Every Margin Mode:**

1. **Identity**: validation stores the UID shown under the app's profile, and
   the Identity shows the Margin Mode that matches the app's account mode.
2. **Trading equity**: `Equity` equals the trading account's total equity in
   USD.
3. **Trading balances**: each coin's equity matches the app's per-coin equity in
   the trading account, including unrealized PnL.
4. **Positions**: every open swap and futures position appears with the same
   direction, size in contracts, entry price, and mark price.
5. **Funding**: coin amounts match the Funding account.
6. **Earn and Dual Investment**: ordinary Earn contains Simple Earn and on-chain
   earn. Dual Investment contains each outstanding subscription at its invested
   principal, with a separate Position showing target, expiry, rate, and state.
   Their combined value matches the platform Earn aggregate. BETH and OKSOL
   appear under `trading` or `funding`, not `earn`.
7. **Account values**: the calculated USD values of `funding` and `earn` are
   within about 0.1% of the app's USD valuation of each account.

**Per Margin Mode:**

8. **Portfolio margin and Multi-currency margin**: `AdjEq` equals the adjusted
   equity and `UniMMR` the margin ratio the app shows, possibly as a percentage.
   In Portfolio margin, cross positions show no leverage and no liquidation
   price.
9. **Spot mode**: with spot borrowing on, `AdjEq` and `UniMMR` match the app.
   Each borrowed coin shows its borrowing, interest included, and its interest
   alone. Without borrowing, the margin figures may stay empty.
10. **Futures mode**: account-level margin figures stay empty. Spot margin
    borrowings appear neither as Balances' liabilities nor as positions, but the
    coin equities still add up to the total equity.

**Changes and failures:**

11. **Mode switch**: after switching the account mode in the app, the next cycle
    reports the new Margin Mode with no failed Account, and the audit log
    records the change.
12. **Dual investment across a settlement**: if an order settles during
    acceptance, note whether its principal dips or counts twice between the
    settlement and the Funding payout. Confirm the next cycle is correct.
13. **Earn check**: holding an Earn product Plutus does not itemize, such as
    Shark Fin, turns `earn` stale with an `unsupported` detail naming OKX's and
    Plutus's amounts, provided OKX counts the product in its Earn valuation.
14. **Failures**: removing the server IP from the key's whitelist sets the
    Connection `invalid` with a `50110` detail at the next cycle. Restoring it
    and validating again resumes collection.
15. **Other modes**: once sub-accounts in Spot mode, Futures mode, and
    Multi-currency margin have their own read-only keys, enter each as an
    Identity and repeat 1–10.
