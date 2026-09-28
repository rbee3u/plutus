---
status: accepted
---

# Merge Asset Listings by normalized symbol with manual overrides

The first catalog refused to equate `BTC` on two Platforms until the User confirmed each mapping, so every new exchange or coin split the total into unmapped provisional rows until someone clicked through them. On the supported Platforms symbol collisions are rare exceptions, so an Asset Listing now maps automatically to the Asset with the same normalized symbol, and the User edits the mapping only for exceptions such as a Platform-specific spelling or a same-symbol different coin. Instruments are discovered from Positions; there is no manual Instrument management and no price pinning.

## Consequences

- A mapping override recalculates the Valuation Points whose Accounts held that listing, as allowed by [ADR 0014](./0014-store-one-valuation-point-per-collection-cycle.md). [ADR 0032](./0032-allow-reviewed-portfolio-listing-exclusions.md) later adds a Portfolio-scoped exclusion action that affects only future Collection Cycles.
- Prices are collected only for Asset Listings and Instruments currently held in selected Accounts. Catalog pins are removed.
- `USD` and `USDT` remain distinct Assets; their valuation equivalence is [ADR 0017](./0017-value-in-usd-and-treat-major-stablecoins-as-usd.md), not a mapping.
- This supersedes the catalog rules of [ADR 0006](./0006-configure-identity-access-and-discover-catalog-entries.md) and the pinning rule of [ADR 0004](./0004-preserve-platform-specific-price-observations.md).
