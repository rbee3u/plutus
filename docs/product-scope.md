# Product scope

Plutus is a read-only portfolio observation tool. It values selected exchange
Accounts in USD and compares observed totals after accounting for the owner's
transfers. The [domain model](../CONTEXT.md) defines the model and rules.

Two questions, in priority order:

1. **How much is there now, and where.** A Platform → Identity → Account tree
   with each Account's report fields (equity, adjusted equity, available equity,
   margin ratios), its Balances, Positions, Earn Positions, and Dual Positions,
   and USD totals per Account, Platform, and Portfolio. The numbers are meant to
   match each exchange app.
2. **How did the total change over a period, excluding my own transfers.** One
   Valuation Point per Collection Cycle (default every 15 minutes), Period
   Closes, and absolute USD Value Change by calendar day, ISO week, month,
   quarter, and year, after subtracting manual Capital Flows and derived
   Boundary Events.

## Portfolio presentation

Overview shows the latest total, recent points, and an expandable Platform →
Identity → Account Tree. Account details show report measures and holdings;
page-level Issues name omissions even when Accounts are collapsed. Only freshly
confirmed empty Accounts hide. See
[ADR 0016](./adr/0016-overview-and-holdings-navigation.md) and
[Manual/empty Account rules](../CONTEXT.md#manual-balances).

Holdings has independent Balances, Futures, Earn, and Dual Investment views,
with Balances as the default. Earn preserves source product granularity and
qualifies rates, amounts, rewards, and lifecycle stages. Options is not offered.

## History presentation

History shows Valuation Points, grouped curves, Period Closes, and Value Changes
in a selected time window, defaulting to seven days. Tables paginate
independently; Value Changes also offers a custom comparison. Window and
pagination choices are recorded in
[ADR 0017](./adr/0017-history-windows-and-pagination.md).

## Supported Accounts

The implemented Adapters offer the following Account Structures, Margin Modes,
and Account Types. Offered scope is distinct from exchange-app validation; see
[integration status](#integration-status).

| Platform | Account Structure (declared) | Margin Modes (observed)                                          | Account Types                                                                                       |
| -------- | ---------------------------- | ---------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| Binance  | Classic Trading              | none                                                             | spot, cross margin, isolated margin, USDⓈ-M futures, COIN-M futures, funding, earn, dual investment |
| Binance  | Portfolio Margin             | none                                                             | spot, portfolio margin, funding, earn, dual investment                                              |
| OKX      | Unified account              | Spot mode, Futures mode, Multi-currency margin, Portfolio margin | trading, funding, earn, dual investment                                                             |
| Bybit    | Unified Trading Account 2.0  | Isolated Margin, Cross Margin, Portfolio Margin                  | unified, funding, earn, dual investment                                                             |
| Kraken   | Separate wallets             | none                                                             | main, multi-m, coin-m, derivatives holding, earn                                                    |
| Kraken   | Unified Wallet               | none                                                             | main, coin-m, derivatives holding, earn                                                             |

Each Identity also owns a fixed, always-selected Manual Account, independent of
Connection health. It supplements API-uncovered signed quantities using public
Platform prices. Pending edits are separate from collected values. See
[ADR 0019](./adr/0019-manual-balances-and-empty-accounts.md).

## Product boundaries

- Only selected Accounts are collected and valued. Main and sub-account logins
  are separate Identities, each with its own read-only credentials.
- USD is the only Valuation Asset. USDT and USDC count as 1 USD; other
  stablecoins are priced normally.
- Capital Flows are manual; selection changes produce Boundary Events. Manual
  quantity adjustments create no flow and make crossing comparisons incomplete.
  Value Change is an absolute amount, not an investment return.
- Trading, transfers, trade history, automatic deposit import, percentage
  returns, attribution, non-USD display, and Exposure aggregation are out of
  scope. Options, unlisted products and structures, shared Portfolios, public
  registration, and manually tracked non-exchange assets are not offered.

Collection success, stale fallback, unknown value, report scopes, Earn/Dual
completeness, exclusions, and history calculations are defined in
[domain model](../CONTEXT.md). Technical deployment limits belong in
[architecture](./architecture.md).

## Integration status

Support in this matrix does not mean an integration has been validated against
its exchange app. See the [README status](../README.md#current-status) and the
per-Platform verification notes: [Binance](./platform/binance/verification.md),
[OKX](./platform/okx/verification.md),
[Bybit](./platform/bybit/verification.md), and
[Kraken](./platform/kraken/verification.md).
