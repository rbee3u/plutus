---
status: accepted
---

# ADR 0003: USD valuation and Asset Listing policy

## Decision

Use USD as the only Valuation Asset. USD, USDT, and USDC value at exactly 1 USD
but remain distinct Assets. Prefer an Account's reported equity; otherwise sum
its Balance equities at eligible same-Platform prices. Balance and Position
breakdowns never add to reported equity. Unknown prices never become zero.

Map Asset Listings automatically by normalized symbol, with manual overrides for
exceptions. Mapping corrections rebuild affected Portfolios. A reviewed,
reversible Portfolio Listing Exclusion affects future calculated values without
changing source facts or reported Account equity; its reason remains auditable.

## Reason and trade-offs

Reported equity preserves exchange-specific collateral and margin calculations.
Same-Platform prices preserve provenance. Symbol mapping removes routine manual
catalog work while permitting collision corrections. Stablecoin equivalence
simplifies observation but accepts undetected de-peg distortion. Exclusions
express an explicit valuation policy, not an assertion that unknown value is
worthless.

The implementation currently allows cross-Platform pricing in places that
contradict this decision. The exact discrepancy and current lookup limits are
recorded in [architecture](../architecture.md#known-implementation-discrepancy).
This record does not accept that behavior as a new policy.
