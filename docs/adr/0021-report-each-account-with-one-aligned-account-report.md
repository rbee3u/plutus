---
status: accepted
---

# Report each Account with one aligned Account Report

Because every Account is a User-selected unit under one Identity ([ADR 0015](./0015-select-account-types-per-identity.md)), a Platform equity figure always describes exactly one Account, and the coverage tables that handled nested or overlapping equity reports have no remaining case. At the same time the owner wants account-level measures such as adjusted equity, available equity, initial and maintenance margin ratios, and UniMMR-style ratios shown in one layout that does not change shape between Platforms. Reported Equity and Account Metric are therefore merged into one Account Report per Account per Collection Run: a nullable `equity` that feeds valuation and a fixed set of Plutus-defined display measures that Adapters map and leave null when a Platform lacks them.

## Consequences

- Account Valuation uses `equity` when present, otherwise the sum of Balance equity times price ([ADR 0017](./0017-value-in-usd-and-treat-major-stablecoins-as-usd.md)). Per-Balance USD values are breakdown only and are never added to an Account's reported equity.
- Adding a display measure requires defining its mapping, or its absence, for every supported Platform before it is added.
- Balance and Position columns follow the same rule: a fixed Plutus-defined set, mapped per Platform, null when unsupported.
- Exposure (long/short/net by underlying Asset) is removed from the first release; Positions show their own size, prices, and unrealized profit instead.
- This supersedes [ADR 0003](./0003-prefer-scoped-reported-equity.md). Its principle that a Platform's equity figure is preferred over a recomputation still holds.
