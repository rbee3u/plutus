---
status: accepted
---

# Absolute Value Change and boundary flows

Measure absolute USD Value Change between observed Valuation Points after
accounting for User-recorded Capital Flows and derived Boundary Events.
Selection changes and Identity archive contribute or withdraw the affected
Account's value; internal transfers do not cross the Portfolio Boundary.

This answers how total value changed without trade-history reconciliation.
Treating boundary changes as flows prevents a newly selected funded Account from
appearing as a gain. The result is neither investment return nor attribution; it
depends on Users recording external transfers and cannot eliminate asynchronous
settlement discrepancies.

Calendar and custom comparisons use observed endpoints rather than invented
boundary values. Missing inputs, relevant policy or scope changes, and effective
Manual adjustments leave computed differences explicitly incomplete. Manual
adjustments do not invent Capital Flows; scope reclassification does not invent
contributions or withdrawals. Boundary completeness is frozen with its amount,
so old source-detail expiry cannot erase uncertainty. Explicit Capital Flow USD
amounts survive other input edits until deliberately replaced or cleared.

[Flow rules](../domain-rules.md#capital-flows-boundary-events-and-value-change)
and [calendar rules](../domain-rules.md#calendar-periods) own intervals and
endpoint selection.
[Presentation](../presentation.md#history-windows-and-tables) owns browsing
windows and pagination.
