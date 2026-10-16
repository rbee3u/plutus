# Query performance measurement: 2026-10-16

These local synthetic measurements compare commit `0471116` with the query
optimization working tree. [Architecture](./architecture.md) owns the read
mechanisms; [testing](./testing.md#read-query-benchmarks) owns reproduction.
There are no schema, index, OpenAPI, or frontend changes.

## Workloads and measurement

The host was an Apple M5, running Go benchmarks with ten logical CPUs and MySQL
through the existing temporary-database harness. Both revisions used the same
benchmark source and independent migrated `plutus_gotest_*` databases. The
original checkout was copied under `/tmp/plutus`; no production database or dump
was used.

The history fixture contains 175,344 summaries at 30-minute intervals from
2016-01-01 through 2026-01-01, with expired detail and no comparison events. It
measures timeline and request overhead, rather than the cost of hydrating many
accounts or frequent policy changes. Week requests use daily comparisons; year
and ten-year requests use monthly comparisons. The original baseline used UTC
and daily curves. Follow-up measurements also cover Asia/Shanghai and
America/New_York, hourly curves, and point-limit rejection.

The revision fixture contains one manual account, 10,000 revisions, 9,999
historical balance rows, 10,000 alternating exclusion versions, and 200 capital
flows referencing that account. Latest-state assertions run during measurement.
This is a stress case for version growth, not a typical personal ledger.

Counters include application reads and consumed rows, excluding transaction
setup and writes. Allocations are Go's cumulative bytes per operation, not peak
RSS. History baseline timings used one iteration; final timings and revision
benchmarks used three iterations. Values are rounded, host-specific evidence,
not latency guarantees for the deployment target.

## Results

| Workload                                 | Reads before → after | Time before → after   |
| ---------------------------------------- | -------------------- | --------------------- |
| Seven-day calendar changes               | 120 → 12             | 57.8 ms → 6.1 ms      |
| One-year calendar changes                | 217 → 12             | 106.8 ms → 15.1 ms    |
| Ten-year calendar changes                | 2,149 → 11           | 1,033.6 ms → 108.3 ms |
| Ten-year daily curve                     | 184 → 26             | 1,528.5 ms → 225.2 ms |
| Ten-year period closes                   | 7 → 7                | 115.5 ms → 49.1 ms    |
| Current manual overlay, 10,000 revisions | 3 → 3                | 22,590.1 ms → 1.1 ms  |
| Catalog exclusion page, 10,000 versions  | 3 → 2                | 16,972.7 ms → 1.0 ms  |
| Capital flow page, 50 rows               | 52 → 2               | 13.7 ms → 1.2 ms      |

Period closes consume 175,465 rows instead of 349,323 for the ten-year window:
the timeline is no longer read twice. The daily curve consumes the same 178,997
rows but reduces header round trips from 176 to 18. Calendar changes consume
175,586 rows instead of 174,930; their improvement comes primarily from sharing
window reads and endpoint hydration.

Batching trades memory for fewer round trips. Ten-year calendar changes allocate
77.6 MB rather than 52.5 MB; the daily curve allocates 74.5 MB rather than 63.9
MB. Period-close allocations fall from 140.8 MB to 87.0 MB. Custom ranges use
endpoint seeks: the ten-year request consumes eight rows in 14 reads and
allocates about 40 KB, avoiding a full timeline allocation. Workloads with many
comparison events can consume more rows.

## Timezone and buffer follow-up

A review found that calendar coverage loaded the timezone once per timeline
entry. UTC bypassed timezone-file parsing and concealed this cost. A separate
three-iteration comparison against the pre-fix working tree measured:

| Workload                            | Time before → after  | Allocation before → after |
| ----------------------------------- | -------------------- | ------------------------- |
| Shanghai ten-year calendar changes  | 1,901.4 ms → 99.1 ms | 319.0 MB → 77.7 MB        |
| UTC ten-year daily curve            | 263.9 ms → 225.2 ms  | 93.8 MB → 74.5 MB         |
| UTC one-year hourly curve           | 38.2 ms → 36.1 ms    | 20.5 MB → 20.2 MB         |
| UTC ten-year hourly limit rejection | 17.3 ms → 12.8 ms    | 17.1 MB → 15.9 MB         |

Timezone loading now happens once per operation. Curves reuse the conversion
buffer and append samples directly into their selected output rather than
allocating an input-sized output for every page. Query counts and consumed rows
remain unchanged for these follow-up comparisons. One-page hourly allocation can
vary slightly with output slice growth: the UTC week case changed from 399 KB to
403 KB. This is a cumulative-allocation trade-off, not a measured increase in
peak or retained memory.

New York ten-year comparisons allocate 78.6 MB and take 100.6 ms in the same
run. Daily curves in all three zones allocate about 74.5 MB. Ten-year hourly
curves still reject excess output after three header reads, consuming 30,000
headers without hydrating summaries. Timing differences are local observations,
not deployment guarantees.

## Scan evidence

`EXPLAIN ANALYZE` showed the original manual query consuming 9,999 historical
balances and executing a 10,000-row revision aggregate 9,999 times. The new
query resolves revision 10,000 first, then reads one balance through its full
primary key.

The original current-exclusion query consumed 10,000 versions and executed a
10,000-row aggregate 10,000 times. The new grouped maximum uses a covering index
skip scan for grouping, then one primary-key lookup of the winning version.
Catalog's per-asset latest-version lookup reads backward with `LIMIT 1`.

A seven-day timeline query against the ten-year fixture used the covering
`idx_valuation_points_timeline` range and consumed 336 entries. No indexes were
added to obtain these plans.

The new asset-change interval query initially used an outer asset join, which
could read 5,000 historical entries to return two recent changes. It now reads
only assets whose endpoint exclusion differs, using constant asset IDs in
256-item batches and the existing primary key. With 5,000 changes on each of
three assets, selecting the final hour for two assets reads four primary-key
range entries and returns four changes. `EXPLAIN ANALYZE` confirms both asset
and time bounds, including after refreshing statistics. Unrelated assets are
outside the ranges; no index or schema change is needed.

## Regression and runtime verification

The ordered `make format && make web && make build && make test && make lint`
gates passed, including temporary MySQL coverage, 283 frontend tests, and zero
Go lint findings. Regeneration reproduced the generated Go and TypeScript
bindings without further changes.

Query-budget regressions cover seven versus thirty calendar periods, forty fresh
empty accounts, shared point/account hydration, one versus fifty capital flow
rows, and page-local Catalog rules. Authentication tests cover joined reads,
session touch, and the microsecond expiry boundary. Existing retention,
stale/missing value, manual editing, and cross-portfolio tests also passed.
Batched calendar results match independent comparisons across equal-time cycles,
mapping/version changes, policy round trips, and missing periods. The follow-up
regressions also exercise 9,999/10,000/10,001-header boundaries, equal-time
cycle successors, DST hours, exact point limits, early rejection, partial and
empty raw pages, and 257 changing assets across policy batches.

On the isolated fake-adapter `plutus_agent` instance at :8081, the original and
optimized binaries returned matching results for 143 requests on the same seed
data and session. Authentication activity timestamps were normalized before
comparison. Checks covered Overview, all four holdings lists and their second
pages, account reports, manual ledgers, history series, calendar and custom
comparisons, snapshots, collection details, Catalog, capital flows, and access
resources. Every request returned 200. The embedded SPA and its four referenced
assets also returned 200.

The timezone/buffer follow-up compared another 126 history responses between the
pre-fix and fixed builds on the same agent data. UTC, Shanghai, and New York
results matched for raw/hour/day curves, raw pages, snapshots, point details,
all calendar kinds, custom comparisons, and expected point-limit errors. Only
request trace IDs in error responses were normalized. Ten default-history smoke
requests and the SPA's four referenced assets also passed. Temporary portfolio
timezone settings were restored, and the verification session was revoked.

Both session-started servers were stopped; health became unreachable and :8081
had no listener. The human's :8080 instance was untouched. These fake-data
checks do not establish live exchange validation or production latency.

## Read-scope follow-up: 2026-10-17

This follow-up compares the staged query-optimization tree at the start of the
session with the additional read-scope changes. It does not compare against
`0471116` again. The Apple M5 host, Go caches, temporary MySQL harness, and
schema were unchanged. Each revision ran the same portable benchmarks for 20
iterations, sequentially, with fixture setup outside the timed sections.

`BenchmarkReadScopeQueries` contains 2,000 assets with five rule versions each,
100 manual accounts, 99 current balances, and four expired summary points.
Catalog requests select 20 assets at offset 20. The policy operation reads four
endpoint policies; the manual operation selects one account. The applied-ledger
case reads a current revision already referenced by a committed manual report.
`BenchmarkPolicyVersionQueries` contains one asset with 10,000 versions and
compares four endpoints in either a recent window or an early window spanning 90
changes. These stress cases separate growth in assets from growth in versions.

| Workload                          | Reads before → after | Consumed rows before → after | Time before → after |
| --------------------------------- | -------------------- | ---------------------------- | ------------------- |
| Policies, 2,000 assets            | 2 → 2                | 10,004 → 2,004               | 7.57 ms → 4.64 ms   |
| Policies, 10,000 versions, recent | 2 → 2                | 10,004 → 5                   | 5.33 ms → 1.77 ms   |
| Policies, 10,000 versions, early  | 2 → 2                | 95 → 95                      | 0.76 ms → 0.61 ms   |
| Catalog, no state filter          | 2 → 2                | 21 → 21                      | 8.02 ms → 1.58 ms   |
| Catalog, excluded filter control  | 2 → 2                | 21 → 21                      | 7.82 ms → 7.91 ms   |
| One manual overlay                | 3 → 3                | 2,101 → 3                    | 4.13 ms → 0.86 ms   |
| Already-applied manual ledger     | 5 → 4                | 5 → 4                        | 1.55 ms → 1.22 ms   |

The recent single-asset policy operation allocates about 7.4 KB rather than 4.31
MB, while the scoped manual overlay allocates 7.2 KB rather than 1.27 MB. The
unfiltered Catalog path allocates 53.7 KB rather than 37.3 KB because it
converts its separately generated page row type. This small allocation increase
trades against substantially less database work. The early policy window
transfers the same rows. An earlier 20-iteration run measured 0.55 ms before and
0.65 ms after, so no consistent speedup is claimed for that short window. The
state-filtered Catalog path is unchanged and serves as a timing-variation
control. These figures are local observations, not deployment latency
guarantees.

### Physical work and remaining bounds

`EXPLAIN ANALYZE` on the same fixtures confirms:

- The new unfiltered Catalog count visits 2,000 asset rows and no rules. The
  original count additionally performs 2,000 rule joins and 6,000 dependent
  latest-version subquery executions in the observed plan.
- The original unfiltered list expands rules for the 40 rows preceding its
  `LIMIT/OFFSET`; the new derived page limits expansion to its 20 returned
  assets. Both still scan and sort the candidate assets for business ordering.
- Opening policy aggregation still scans 10,000 historical versions for the
  many-asset fixture, then joins 2,000 winning rows. It reduces transferred rows
  and repeated application traversal, rather than eliminating the opening scan.
  Window changes use `idx_asset_exclusion_versions_effective` with both time
  bounds. Highest eligible version selection does not assume monotonic times.
- A scoped manual overlay reads one revision, one balance, and one current rule;
  the rule lookup resolves its requested asset through the existing primary key.
  An empty ledger retains its version and skips the rule query entirely.

All three affected creation endpoints now authenticate once before idempotency
lookup, then reuse the user for portfolio ownership checks on fresh requests.
HTTP query budgets verify six reads for capital flow creation and seven for
identity or cycle creation. Each avoids one duplicate joined session read and
one session touch. Replays still perform three reads and one touch, without
repeating business writes. Conflict, invalid CSRF, missing ownership, and
revoked session cases retain their rejection behavior.

### API review disposition and verification

| API group                    | Disposition                                                                                                                               |
| ---------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| Authentication and sessions  | Keep keyed session/user reads, active-session filtering, and per-request touch; reuse authentication only inside the same create request. |
| Portfolios and identities    | Keep ownership/version checks and batched account lists; no per-identity query was added.                                                 |
| Overview and account reports | Keep shared point evidence and source-qualified stale reads; narrow only current manual overlays.                                         |
| Four Holdings lists          | Keep exact totals, whole-scope diagnostics, and page detail loading; pagination must not discard required aggregation inputs.             |
| History and comparisons      | Keep bounded headers and shared endpoint hydration; narrow exclusion history and traverse it once.                                        |
| Collection cycles            | Keep indexed cursor pages and shared detail hydration; avoid repeated authentication during enqueue.                                      |
| Capital and boundary flows   | Keep indexed time ranges, exact counts, deterministic pages, and joined account labels.                                                   |
| Catalog assets and contracts | Narrow unfiltered asset reads; retain rule-aware state filters and literal substring search.                                              |
| Manual ledgers and audit     | Reuse equal applied revisions; keep immutable audit/version resolution and cursor reads.                                                  |

Rare audit filters and deep offset pages can still examine many entries. No
representative workload established a need for extra indexes or a different
pagination contract. Worker collection and retention are outside this API
follow-up. There are no schema, index, OpenAPI, or frontend source changes.

The ordered validation gates passed, including temporary MySQL coverage, all 283
frontend tests, and zero lint findings. Code generation reproduced all generated
sources. New regressions cover strict and equal-time policy cutoffs,
nonmonotonic effective times, policy restoration, foreign portfolios, empty and
scoped manual ledgers, current/applied revision separation, Catalog later/empty
pages, and all three creation endpoints' fresh and replay query/write budgets.
Existing retention and comparison regressions also passed.

The saved baseline and optimized binaries returned matching results for 346
fixed requests on the same `plutus_agent` seed data and session: 292 successful
responses and 54 expected account-outside-boundary 404 responses. Checks
included current and pending manual presentation, all four Holdings lists,
account reports, Catalog filters and pages, all calendar kinds, custom
comparisons, curves, snapshots, flows, audit, and access resources. Only session
activity expiry/timestamps and request trace IDs were normalized. The embedded
SPA and its four referenced assets returned 200. The verification session was
revoked, both comparison servers stopped, and :8081 was released. The human's
:8080 instance was untouched.

A further agent run exercised fresh creation and replay for identities, capital
flows, and collection cycles. All three also rejected invalid CSRF and foreign
portfolio IDs; a changed flow body rejected key reuse, and all three rejected
requests after logout. The temporary portfolio was deleted, its verification
session revoked, and the session-started server stopped with :8081 released.
These checks use fake data and establish neither live exchange validation nor
production latency.

## Authentication and sparse-read repair: 2026-10-17

A follow-up review found that reusing the initially authenticated user also
removed the fresh creation's authentication check after waiting for the
idempotency user lock. A session revoked or expired during that wait could still
create data. Fresh creation now rechecks session validity and user status inside
the transaction after obtaining the lock, without touching the session twice.
The earlier claim that fresh creation eliminates a session read is superseded:
capital creation uses seven reads and four writes; identity and cycle creation
use eight reads, retaining their existing business write counts. Replay remains
three reads and one touch. Authentication and CSRF still precede replay.

Twelve concurrent regressions cover the three creation resources while the user
lock is held and the session is revoked, the user is disabled, the absolute
expiry passes, or the password is reset with session revocation. Each returns
401 and leaves no business row or idempotency response. Fresh, replay, conflict,
invalid CSRF, foreign ownership, and already revoked session cases also pass.

The query fixture contains 17,520 expired points over one year, two manual and
boundary change markers, and one remapped asset. Its sparse variant has two
calculated asset markers; its dense variant has 17,520 alternating included and
absent markers. The saved pre-repair source and final source used the same
benchmark fixture and separate temporary databases. Values below are medians of
three runs of ten timed iterations, with fixture setup excluded.

| Workload                                 | Reads before → after | Consumed rows before → after | Time before → after | Allocated bytes before → after |
| ---------------------------------------- | -------------------- | ---------------------------- | ------------------- | ------------------------------ |
| Sparse one-year custom comparison        | 19 → 19              | 17,535 → 18                  | 28.92 ms → 21.64 ms | 6,876,155 → 66,910             |
| Dense history, final-hour event headers  | 1 → 1                | 1 → 1                        | 6.02 ms → 0.41 ms   | 2,601 → 3,009                  |
| Sparse history, final-hour event headers | 1 → 1                | 1 → 1                        | 0.34 ms → 0.41 ms   | 2,764 → 3,043                  |
| Sparse history, one-year event headers   | 1 → 1                | 2 → 2                        | 0.32 ms → 0.73 ms   | 2,897 → 3,220                  |
| Dense history, one-year event headers    | 1 → 1                | 8,761 → 8,761                | 11.69 ms → 12.00 ms | 2,736,201 → 2,736,609          |

Sparse predecessor hydration no longer transfers the intervening timeline.
Allocation falls by about 99%; the request still scans calculation-version
headers inside MySQL to preserve version transitions, so its elapsed time does
not fall proportionally to transferred rows. Constant-bound predecessor seeks
are batched into one query for at most 256 selected points; larger sets retain
the shared range read. Equal-time cycle order, first points, calculation
versions, cross-portfolio isolation, and predecessors before the public history
boundary are covered at both sides of that batch threshold.

Event-header plans now cap the initial timeline probe at 1,001 entries. A window
containing at most 1,000 points seeks manual, boundary, and remapped
included-asset evidence by cycle. Longer windows use the original sparse-event
path. In the final-hour dense fixture, the old plan scans 17,520 asset markers
and performs 8,760 point lookups for one result; the new path considers two
window points and leaves the historical event branches unexecuted. Plans were
checked both before and after `ANALYZE TABLE`; merely allowing join reordering
was rejected because fresh statistics still chose the full historical scan.

The bounded probe adds about 0.4 ms to the sparse year query on this host. It
avoids forcing 17,520 timeline lookups for only two events and gives short dense
windows a stable scan bound. These are local synthetic trade-offs, not
production latency guarantees. Regression results match independent fixture
facts for empty windows and windows of 1, 2, 1,000, 1,001, and 17,520 points,
including inclusive boundaries and deduplication across evidence types.

The other API groups retain the dispositions above. Calculation-version scans,
rare audit filters, and deep offset pages remain candidates without sufficient
workload evidence for another change. No cache, precomputed table, index,
migration, public API, or frontend source was added.

The ordered validation gates passed after the repair, including all Go tests,
283 frontend tests, and zero lint findings. The concurrent authentication cases
also passed with Go's race detector. Regeneration reproduced the generated
sources; all 236 API, frontend, and migration files match the saved pre-repair
snapshot.

On the same `plutus_agent` data and browser session, the saved baseline and
final binary returned identical responses for 296 fixed requests: 242 successful
responses and 54 expected account-outside-boundary 404 responses. Paths include
point details, custom and all five calendar comparison kinds, period closes,
raw/hour/day curves, current and pending manual overlays, account reports,
Holdings, Catalog, flows, audit, and access resources. Session activity times
and trace IDs were normalized; event windows used fixed timestamps. The SPA and
its four referenced assets returned 200. Fresh creation and replay passed for
identities, capital flows, and cycles, together with CSRF, ownership, conflict,
and post-logout rejection checks. The temporary portfolio was deleted and the
verification sessions revoked. All session-started servers stopped and :8081 was
released; :8080 was untouched. This is fake-data API verification, not live
exchange validation.
