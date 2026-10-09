---
status: accepted
---

# USD valuation and Platform Asset policy

USD is the only Valuation Asset; USD, USDT, and USDC value at 1 USD while
remaining distinct Assets. Prefer reported Account equity, otherwise calculate
from Balance equity and eligible same-Platform prices. Descriptive holdings do
not add to reported equity, and unknown prices never become zero.

Reported equity preserves exchange collateral calculations, and same-Platform
prices preserve provenance. Stablecoin equivalence simplifies observation at the
cost of undetected de-peg distortion. Source quantities and prices remain
available so that this assumption is visible.

Map Platform Assets by normalized symbol with manual collision corrections.
Reviewed, reversible Platform Asset Exclusions affect future calculated values
without changing source facts or reported equity. Automatic mapping reduces
routine catalog work; corrections require rebuilds. Exclusions express an
auditable policy rather than treating unknown value as worthless, so comparison
rules must account for relevant changes in treatment.

[Valuation rules](../domain-rules.md#assets-and-valuation) own eligibility and
completeness. The
[known pricing discrepancy](../architecture.md#known-implementation-discrepancy)
is an unresolved implementation gap, not permission for cross-Platform fallback.

Platform Assets retain their Platform identity separately from the
cross-Platform Asset they map to. Futures Contracts describe perpetual and
delivery contracts; a Futures Symbol is their Platform code, not the contract
object or a trading pair. The domain uses these specific objects rather than a
generic Instrument that would imply shared spot and option semantics. UI
category labels remain compact; the
[presentation rules](../presentation.md#catalog-labels) own them.
