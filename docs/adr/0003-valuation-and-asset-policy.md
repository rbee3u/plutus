---
status: accepted
---

# USD valuation and asset policy

USD is the only valuation asset; USD, USDT, and USDC value at 1 USD while
remaining distinct canonical assets. Prefer reported account equity, otherwise
calculate from balance equity and eligible same-platform prices. Descriptive
holdings do not add to reported equity, and unknown prices never become zero.

Reported equity preserves exchange collateral calculations, and same-platform
prices preserve provenance. Stablecoin equivalence simplifies observation at the
cost of undetected de-peg distortion. Source quantities and prices remain
available so that this assumption is visible.

Map assets to canonical assets by normalized symbol with manual collision
corrections. Reviewed, reversible asset exclusions affect future calculated
values without changing source facts or reported equity. Automatic mapping
reduces routine catalog work; corrections take effect at the next point. Adopted
mapping changes mark crossing comparisons incomplete without rewriting history.
Exclusions express an auditable policy rather than treating unknown value as
worthless, so comparison rules must account for relevant changes in treatment.

[Valuation rules](../domain-rules.md#assets-and-valuation) own eligibility and
completeness. Stored price evidence preserves the adopted same-platform
observation and its source market.

Assets retain their portfolio and platform identity separately from the
canonical asset they map to. Futures contracts describe perpetual and delivery
contracts; a futures symbol is their platform code, not the contract object or a
trading pair. The domain uses these specific objects rather than a generic
instrument that would imply shared spot and option semantics. UI category labels
remain compact; the [presentation rules](../presentation.md#catalog-labels) own
them.

Asset is the short name for the portfolio- and platform-scoped object that
dominates holdings and collection. Canonical asset explicitly qualifies the less
frequent cross-platform identity. This keeps common references compact at the
cost of requiring portfolio and platform context to identify an asset; matching
symbols alone never establish shared asset identity.
