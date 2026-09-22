---
status: accepted
---

# Store one valuation point per collection cycle

The first design rebuilt every derived result into versioned projection generations so that any source correction could be replayed. That engine existed for corrections that almost never happen, and it made "projection" the least understood part of the product. Plutus now runs one Collection Cycle per Portfolio (default every 15 minutes), and at the end of each cycle writes one Valuation Point with one Account Valuation per Account. Those rows are written once and read forever; Value Change, Period Close, and Boundary Events are computed at query time from Valuation Points and Capital Flows. Only an Asset Listing mapping correction recalculates already-written points, and it recalculates just the points whose Accounts held that listing.

## Considered options

- Keep projection generations with full rebuilds: correct for every correction, but a heavy engine for a personal tool whose primary view is the present total.
- Compute everything at read time: simplest, but every history query would re-price every past Observation.

## Consequences

- The Collection Cycle is Portfolio-level. Every enabled Connection in the Portfolio is collected in the same cycle, staggered by seconds, so one Valuation Point mixes only data from that cycle or Stale carry-forward. Connections no longer have individual intervals.
- Observations are never edited; there is no Observation correction endpoint and no audit before/after document for source facts. Wrong data is replaced by the next cycle.
- `projection_generations` and every `projection_*` table, `POST /projection-rebuilds`, and `plutus projection rebuild` are removed.
- Valuation Points are kept indefinitely; no retention or downsampling is implemented until real data shows a need.
- This supersedes the projection model in [ADR 0007](./0007-persist-normalized-observations-and-rebuild-derived-results.md) and the snapshot-batch selection in [ADR 0013](./0013-select-identity-state-by-snapshot-batch-and-archive-time.md).
