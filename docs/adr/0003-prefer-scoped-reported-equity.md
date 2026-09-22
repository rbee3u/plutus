---
status: superseded by ADR-0021
---

# Prefer scoped reported equity

Platform equity reports can incorporate liabilities, unrealized profit and loss, and product coverage that cannot be reliably reconstructed from a partial set of Position details. Build Portfolio Valuations from Reported Equity for its declared scope, preserve Calculated Valuation separately to explain differences, and use the calculated value only when no report is available for that scope and sufficient valuation inputs exist. Aggregate only non-overlapping contributions: overlapping Balance measures are not separate value, and adding Balances, margin, or unrealized profit and loss already included in a report would count the same value twice; contract notional and margin requirements are not additional equity.

## Consequences

- Preserve a reported total when its details are incomplete, and expose missing coverage and any measurable unexplained difference. Unknown quantities or values are not zero; insufficient inputs leave a calculation incomplete.
- Reports must describe their Identity and Account coverage. A trading Account report may exclude a funding Account, while a report for a parent Identity may include subordinate Identities; those relationships must be established before totals are added.
- For example, if a reported equity value of USD 10,000 already includes a separately observed Balance valued at USD 2,000, the contribution is USD 10,000. Add that Balance separately only when it is known to fall outside the report's scope.
- At an evaluation cutoff, use each Identity's latest successful scope snapshot at or before that cutoff, as defined in [ADR 0013](./0013-select-identity-state-by-snapshot-batch-and-archive-time.md). If an Identity refresh fails, retain its previous successful scope with its original observation times and expose the failure and freshness information; an Identity with no successful snapshot remains missing when it is required coverage.
- Keep coverage, observation times, and completeness visible for current and historical Portfolio Valuations. Neither one Identity Snapshot nor a Portfolio Valuation guarantees simultaneous observations across its sources.
