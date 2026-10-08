---
status: accepted
---

# USD valuation and Asset Listing policy

USD is the only Valuation Asset; USD, USDT, and USDC value at 1 USD while
remaining distinct Assets. Prefer reported Account equity, otherwise calculate
from Balance equity and eligible same-Platform prices. Descriptive holdings do
not add to reported equity, and unknown prices never become zero.

Reported equity preserves exchange collateral calculations, and same-Platform
prices preserve provenance. Stablecoin equivalence simplifies observation at the
cost of undetected de-peg distortion. Source quantities and prices remain
available so that this assumption is visible.

Map Asset Listings by normalized symbol with manual collision corrections.
Reviewed, reversible Listing Exclusions affect future calculated values without
changing source facts or reported equity. Automatic mapping reduces routine
catalog work; corrections require rebuilds. Exclusions express an auditable
policy rather than treating unknown value as worthless, so comparison rules must
account for relevant changes in treatment.

[Valuation rules](../domain-rules.md#assets-and-valuation) own eligibility and
completeness. The
[known pricing discrepancy](../architecture.md#known-implementation-discrepancy)
is an unresolved implementation gap, not permission for cross-Platform fallback.
