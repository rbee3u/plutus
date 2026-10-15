---
status: accepted
---

# Absolute value change and boundary flows

Measure absolute USD value change between observed valuation points after
accounting for user-recorded capital flows and derived boundary events.
Selection changes and identity archive contribute or withdraw the affected
account's value; internal transfers do not cross the portfolio boundary.

This answers how total value changed without trade-history reconciliation.
Treating boundary changes as flows prevents a newly selected funded account from
appearing as a gain. The result is neither investment return nor attribution; it
depends on users recording external transfers and cannot eliminate asynchronous
settlement discrepancies.

Calendar and custom comparisons use observed endpoints rather than invented
boundary values. Missing inputs, relevant policy or scope changes, and effective
manual adjustments leave computed differences explicitly incomplete. Manual
adjustments do not invent capital flows; scope reclassification does not invent
contributions or withdrawals. Boundary completeness is frozen with its amount,
so old source-detail expiry cannot erase uncertainty. Explicit capital flow USD
amounts survive other input edits until deliberately replaced or cleared.

[Flow rules](../domain-rules.md#capital-flows-boundary-events-and-value-change)
and [calendar rules](../domain-rules.md#calendar-periods) own intervals and
endpoint selection.
[Presentation](../presentation.md#history-windows-and-tables) owns browsing
windows and pagination.
