---
status: accepted
---

# ADR 0004: Absolute Value Change over observed points

## Decision

Measure absolute USD Value Change between Valuation Points after subtracting
contributions and adding withdrawals. Users record Capital Flows manually;
selection changes and Identity archive generate implicit flows through Boundary
Events. Internal transfers do not cross the Portfolio Boundary.

Calendar periods use the Portfolio timezone and ISO weeks. Period Closes use
points within their periods; Value Change uses selected observed endpoints.
Missing endpoints and unvalued flows or boundary amounts remain explicit. Scope
reclassification and differing relevant exclusion policies make affected
comparisons incomplete rather than producing fabricated flows.

Effective Manual quantity adjustments generate no flow and mark crossing
comparisons incomplete while retaining the calculated difference. Price-only
changes and edits reverted before collection do not mark adjustments. See
[ADR 0019](./0019-manual-balances-and-empty-accounts.md).

## Reason and trade-offs

Observation plus manual flows answers how total value changed without trade
history or transaction reconciliation. Treating selection as a flow prevents
adding a funded Account from appearing as a gain. The result is not investment
return or attribution, and correctness depends on recording external transfers.
Read timing and product settlement can still disagree.

See
[flow rules](../../CONTEXT.md#capital-flows-boundary-events-and-value-change)
and [calendar semantics](../../CONTEXT.md#calendar-periods) for endpoint
selection and interval details.
