---
status: accepted
---

# Aligned reports and compatible history

Use one aligned Account Report per selected Account and Collection Run. Reported
equity and fixed account measures accompany the holdings they describe;
unsupported measures stay unknown. Only successful reports establish Complete
Sets and remove absent holdings. Failed Accounts retain compatible successful
history as stale; without it, their value is unknown.

This preserves the Platform's Account boundary and useful last-known value
without claiming freshness or reusing reports with different coverage. Known
value is summed, omissions are named, and freshness remains independent of
completeness. Immutable Account scope, stale ages, and missing-data handling
remain necessary throughout valuation and presentation.

Immutable source facts and committed valuation evidence preserve provenance. A
completed cycle produces one Valuation Point; recovery retains committed
reports, including failures, and reuses an existing point. Uncommitted work can
use current boundary and Connection state, so recovery is not a frozen Platform
snapshot or exactly-once collection.

[Report rules](../domain-rules.md#collection-and-account-reports) own financial
semantics; [collection lifecycle](../architecture.md#collection-cycle-lifecycle)
owns commit and recovery mechanisms.
