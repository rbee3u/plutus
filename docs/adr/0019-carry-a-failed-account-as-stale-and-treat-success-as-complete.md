---
status: accepted
---

# Carry a failed Account's last report as stale and treat a successful report as the complete set

The primary view is "how much is there now", so a transient timeout on one wallet must not knock a slice out of the total, yet a coin that was fully transferred away must not linger because the Platform simply stopped listing it. These two cases are distinguished by the outcome of the Account's Collection Run, never by the presence of an individual coin: when the run fails, the Account's most recent successful Account Report, including all of its Balances and Positions, is used and labeled Stale with the age of that report, with no time limit; when the run succeeds, its Balances and Positions are the Complete Set and anything absent is no longer held.

## Consequences

- Carry-forward is Account-level. There is no per-coin or per-position carry-forward.
- An Account that has never had a successful report contributes nothing and makes the Valuation Point incomplete.
- Each Account Valuation records which Account Report it used, so history shows where a point relied on stale data.
- A Portfolio with no Collection Cycle on a calendar day has no Period Close for that day; a Value Change spanning it is incomplete rather than borrowed from the previous day.
- This supersedes [ADR 0013](./0013-select-identity-state-by-snapshot-batch-and-archive-time.md).

[ADR 0033](./0033-separate-dual-investment-accounts-and-positions.md) limits stale carry-forward to reports of the same Account scope. Legacy combined Earn reports cannot fill a new ordinary Earn report; without compatible history the value is explicitly missing.
