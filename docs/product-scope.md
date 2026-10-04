# Product scope

Plutus is a read-only portfolio observation tool. It values selected exchange
Accounts in USD and compares observed totals after accounting for the owner's
transfers. The [domain model](../CONTEXT.md) defines the model and rules.

Two questions, in priority order:

1. **How much is there now, and where.** A Platform → Identity → Account tree
   with each Account's report fields (equity, adjusted equity, available equity,
   margin ratios), its Balances, Positions, and Dual Positions, and USD totals
   per Account, Platform, and Portfolio. The numbers are meant to match each
   exchange app.
2. **How did the total change over a period, excluding my own transfers.** One
   Valuation Point per Collection Cycle (default every 15 minutes), Period
   Closes, and absolute USD Value Change by calendar day, ISO week, month,
   quarter, and year, after subtracting manual Capital Flows and derived
   Boundary Events.

## Portfolio presentation

Overview presents the latest total, recent Valuation Points, and the ownership
Tree. Each node presents a name and adjacent USD value without repeated counts
or status badges. Its default rows show Platform and Identity; expanding an
Identity shows Accounts, and expanding an Account shows report measures and
holding details. Balances remain visible even when empty. Futures and Dual
Investment details appear when held; loading and collection failures remain
explicit inside Account details. Report provenance is available in a folded
Report details section. Page-level Issues explain omissions even when Accounts
are folded.

Holdings presents separate Balances, Futures, and Dual Investment tables, each
with independent view filters. Balances is the default view. Dedicated Options
and Earn views are not offered in this delivery; existing Earn Account
collection and valuation remain supported.

## History presentation

History defaults to an explicit rolling seven-day window, with shorter and
longer presets and Portfolio-timezone custom endpoints. Valuation Points shows
an automatically grouped full-window curve, Period closes, and independently
paginated raw points. Value Changes shows complete calendar results in the same
selected window and retains its independent custom comparison. All three tables
show newest records first, default to 20 rows, and offer 10/20/50 rows per page.
See [ADR 0017](./adr/0017-history-windows-and-pagination.md).

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

## Product boundaries

- Only selected Accounts are collected and valued. Main and sub-account logins
  are separate Identities, each with its own read-only credentials.
- USD is the only Valuation Asset. USDT and USDC count as 1 USD; other
  stablecoins are priced normally.
- Capital Flows are manual; selection changes are implicit flows. Value Change
  is an absolute amount, not an investment return.
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
per-Platform verification notes: [Binance](./adapters/binance/verification.md),
[OKX](./adapters/okx/verification.md),
[Bybit](./adapters/bybit/verification.md), and
[Kraken](./adapters/kraken/verification.md).
