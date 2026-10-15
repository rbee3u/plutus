# Product scope

Plutus is a read-only portfolio observation tool. It values selected exchange
accounts in USD and compares observed totals after accounting for the owner's
transfers. The [glossary](../GLOSSARY.md) defines terms and
[domain rules](./domain-rules.md) define business behavior.

Two questions, in priority order:

1. **How much is there now, and where.** A platform → identity → account tree
   with each account's report fields (equity, adjusted equity, available equity,
   margin ratios), its balances, positions, earn positions, and dual positions,
   and USD totals per account, platform, and portfolio. The numbers are meant to
   match each exchange app.
2. **How did the total change over a period, excluding my own transfers.** One
   valuation point per collection cycle (default every 30 minutes), period
   closes, and absolute USD value change by calendar day, ISO week, month,
   quarter, and year, after subtracting manual capital flows and derived
   boundary events.

## Portfolio presentation

Overview shows the latest total, recent points, and an expandable platform →
identity → account tree. Account details show report measures and holdings;
page-level Issues name omissions even when accounts are collapsed. Only freshly
confirmed empty accounts hide. See
[Overview and navigation](./presentation.md#overview-and-navigation) and
[manual/empty account rules](./domain-rules.md#manual-balances).

Holdings has independent balances, futures, earn, and dual investment views,
with balances as the default. Earn preserves source product granularity and
qualifies rates, amounts, rewards, and lifecycle stages. Options is not offered.

## History presentation

History shows valuation points, grouped curves, period closes, and value changes
in a selected time window, defaulting to three days. Tables paginate
independently; value changes also offers a custom comparison. Window and
pagination choices are recorded in
[History presentation](./presentation.md#history-windows-and-tables).

## Supported accounts

The implemented adapters offer the following account structures, margin modes,
and account types. Offered scope is distinct from exchange-app validation; see
[integration status](#integration-status).

| Platform | Account structure (declared)  | Margin modes (observed)                                                  | Account types                                                                                                       |
| -------- | ----------------------------- | ------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------- |
| Binance  | `Classic trading`             | None                                                                     | `Spot`, `Cross margin`, `Isolated margin`, `USDⓈ-M futures`, `COIN-M futures`, `Funding`, `Earn`, `Dual investment` |
| Binance  | `Portfolio margin`            | None                                                                     | `Spot`, `Portfolio margin`, `Funding`, `Earn`, `Dual investment`                                                    |
| OKX      | `Unified account`             | `Spot mode`, `Futures mode`, `Multi-currency margin`, `Portfolio margin` | `Trading`, `Funding`, `Earn`, `Dual investment`                                                                     |
| Bybit    | `Unified Trading Account 2.0` | `Isolated margin`, `Cross margin`, `Portfolio margin`                    | `Unified`, `Funding`, `Earn`, `Dual investment`                                                                     |
| Kraken   | `Separate wallets`            | None                                                                     | `Main`, `Multi-M`, `Coin-M`, `Derivatives holding`, `Earn`                                                          |
| Kraken   | `Unified wallet`              | None                                                                     | `Main`, `Coin-M`, `Derivatives holding`, `Earn`                                                                     |

Each identity also owns a fixed, always-selected manual account, independent of
connection health. It supplements API-uncovered signed quantities using public
platform prices. Pending edits are separate from collected values. See
[ADR 0006](./adr/0006-manual-balances.md).

## Product boundaries

- Only selected accounts are collected and valued. Main and sub-account logins
  are separate identities, each with its own read-only credentials.
- USD is the only valuation asset. USDT and USDC count as 1 USD; other
  stablecoins are priced normally.
- Capital flows are manual; selection changes produce boundary events. Manual
  quantity adjustments create no flow and make crossing comparisons incomplete.
  Value change is an absolute amount, not an investment return.
- Trading, transfers, trade history, automatic deposit import, percentage
  returns, attribution, non-USD display, and exposure aggregation are out of
  scope. Options, unlisted products and structures, shared portfolios, public
  registration, and manually tracked non-exchange assets are not offered.

Collection success, stale fallback, unknown value, report scopes, earn/dual
completeness, exclusions, and history calculations are defined in
[domain rules](./domain-rules.md). Technical deployment limits belong in
[architecture](./architecture.md).

## Integration status

Support in this matrix does not mean an integration has been validated against
its exchange app. See the [README status](../README.md#current-status) and the
per-platform verification notes: [Binance](./platform/binance/verification.md),
[OKX](./platform/okx/verification.md),
[Bybit](./platform/bybit/verification.md), and
[Kraken](./platform/kraken/verification.md).
