---
status: accepted
---

# Aligned reports and compatible history

Use one aligned account report per selected account and collection run. Reported
equity and fixed account measures accompany the holdings they describe;
unsupported measures stay unknown. Only successful reports establish complete
sets and remove absent holdings. Failed accounts retain compatible successful
history as stale; without it, their value is unknown.

This preserves the platform's account boundary and useful last-known value
without claiming freshness or reusing reports with different coverage. Known
value is summed, omissions are named, and freshness remains independent of
completeness. Immutable account scope, stale ages, and missing-data handling
remain necessary throughout valuation and presentation.

Immutable source facts and committed valuation evidence preserve provenance. A
completed cycle produces one valuation point; recovery retains committed
reports, including failures, and reuses an existing point. Uncommitted work can
use current boundary and connection state, so recovery is not a frozen platform
snapshot or exactly-once collection.

[Report rules](../domain-rules.md#collection-and-account-reports) own financial
semantics; [collection lifecycle](../architecture.md#collection-cycle-lifecycle)
owns commit and recovery mechanisms.
