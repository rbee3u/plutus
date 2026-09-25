---
status: accepted, supported list maintained in ADR-0028
---

# Support explicitly selected Account Structures and Margin Modes

Account terminology and product coverage differ between Platforms, and compatibility with unused account configurations would expand integration work without serving the portfolio. Support only explicitly listed configurations. The first list was Binance Portfolio Margin, OKX Multi-currency margin and Portfolio margin, and Bybit Unified Trading Account 2.0. It now also covers Binance Classic Trading, OKX Spot and Futures modes, Bybit Isolated and Cross Margin, and both Kraken wallet structures. [ADR 0028](./0028-separate-account-structure-from-margin-mode.md) keeps the current list of Account Structures and Margin Modes. Target spot balances and stablecoin-margined (linear) and coin-margined (inverse) perpetual and delivery contracts, verifying product availability and equity-report coverage for each supported configuration.

## Consequences

- Other configurations, such as Binance Portfolio Margin Pro, Bybit classic accounts, and Bybit UTA 1.0, are outside the scope. Unsupported configurations must be identified explicitly rather than silently treated as supported.
- Supported describes a Platform integration's capability for these Account Structures and Margin Modes; discovering or tracking an Asset Listing or Instrument does not expand that capability. Apply the Account Type selection rules in [ADR 0015](./0015-select-account-types-per-identity.md).
- Options, dedicated borrowing or lending products, other exchanges, and manual asset management are outside the first release. Liabilities included in Reported Equity for the supported scope still matter to its net value. Dual investment is read as part of Earn ([ADR 0029](./0029-count-dual-investment-in-earn-and-check-earn-completeness.md)).
- The Account Structure decides which Account Types exist; the Adapter declares them per structure and the User selects them ([ADR 0015](./0015-select-account-types-per-identity.md)).
- Keep the shared domain language independent of any one Platform so later integrations can extend coverage. Manual management is a later product-release candidate; its detailed rules are not first-release commitments.
