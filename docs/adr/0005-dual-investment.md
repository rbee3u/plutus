---
status: accepted
---

# ADR 0005: Separate Dual Investment and check Earn completeness

## Decision

Binance, OKX, and Bybit have separately selected Earn and Dual Investment
Accounts. Each outstanding Dual subscription is a distinct Dual Position;
Balances aggregate invested principal by currency. Expected yield is excluded,
and Positions describe the same value without adding it again.

When either Account is selected, read both product groups for a combined
completeness check; emit and value only selected Accounts. Required auxiliary
failures and unexplained aggregate shortfalls fail both selected dependents. Do
not read either when neither is selected.

Reports preserve product scope. Stale fallback requires compatible scope;
splitting observed combined Earn into ordinary Earn and Dual is
reclassification, not an additional contribution. Comparisons spanning the split
are incomplete.

## Reason and trade-offs

Separate product Accounts preserve subscription terms and selection boundaries
without pretending Dual has a transferable cash wallet. Shared completeness
reads detect otherwise omitted Earn products, at the cost of a coupled failure
dependency. Non-atomic subscription, settlement, and payout reads can still
produce temporary gaps or double counts.

[Domain model](../../CONTEXT.md#dual-investment-and-earn-completeness) owns
normalization, tolerance, and transition rules.
[Deployment](../deployment.md#dual-investment-split-upgrade) owns the
existing-data upgrade procedure.
