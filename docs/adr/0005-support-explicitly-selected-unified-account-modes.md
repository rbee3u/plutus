---
status: accepted
---

# Support explicitly selected unified account modes

Unified account terminology and product coverage differ between Platforms, and compatibility with unused account configurations would expand integration work without serving the initial portfolio. Limit initial integrations to Binance Portfolio Margin (PM), OKX Multi-currency margin (MCM) and Portfolio margin (PM), and Bybit Unified Trading Account 2.0 (UTA 2.0). Target spot balances and stablecoin-margined (linear) and coin-margined (inverse) perpetual and delivery contracts, verifying product availability and equity-report coverage for each selected Platform and mode.

## Consequences

- Traditional or classic accounts and other mode variants are outside the initial integration scope. Unsupported modes must be identified explicitly rather than silently treated as supported configurations.
- Supported describes a Platform integration's capability for these account modes; discovering or tracking an Asset Listing or Instrument does not expand that capability. Apply the Account Type selection rules in [ADR 0015](./0015-select-account-types-per-identity.md).
- Options, dedicated borrowing or lending products, other exchanges, and manual asset management are outside the first release. Liabilities included in Reported Equity for the supported scope still matter to its net value.
- A unified trading mode does not by itself say which Account Types exist; the Adapter declares them per mode and the User selects them ([ADR 0015](./0015-select-account-types-per-identity.md)).
- Keep the shared domain language independent of any one Platform so later integrations can extend coverage. Manual management is a later product-release candidate; its detailed rules are not first-release commitments.
