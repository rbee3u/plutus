# Prelaunch schema and history delivery

The baseline defines 26 tables. Collection Cycles carry their own attempts and
leases, Portfolio deletion is driven by the root's request, and every
Portfolio-owned primary key starts with `portfolio_id`. Other responsibilities
remain separate; `manual_balance_changes` and sparse `calculated_asset_changes`
preserve comparison facts after detail expiry.
[ADR 0015](../adr/0015-prelaunch-schema.md) owns schema trade-offs;
[ADR 0011](../adr/0011-retained-data-and-upgrades.md) owns retention. The
[baseline SQL](../../db/migrations/20261010160000_schema.sql) owns exact fields,
keys, and constraints.
[Architecture](../architecture.md#persistence-and-retention) owns the
persistence and cleanup mechanisms.

The approved delivery includes immutable first commits, 30-day detail and daily
anchors, protected stale sources and adopted quotes, daily historical prices,
explicit snapshot fallback, durable Manual/policy comparison markers, and
bounded asynchronous Portfolio deletion. Public price supplementation precedes
report persistence; source observation times remain distinct. Connection outcome
updates use the captured Identity version. Historical price eligibility remains
same-Platform, nonfuture, and at most 24 hours old.

Naming distinguishes initial Portfolios, Asset mapping versions, Position source
quantities and units, and Asset Exclusions. Extensible domain catalogs remain in
runtime validation; stable SQL ownership and shape checks remain. Earn optional
description lengths, OKX missing currencies, positive quote fallback, exact
Catalog text, manual Capital Flow overrides, and atomic authentication/session
updates are part of the same prelaunch delivery.

The second review removed write-only columns (cycle requester, revision and
exclusion actors, Kraken source codes, the detail watermark, idempotency IDs,
and copied exclusion versions), renamed successful report status to `succeeded`
and policy pricing to `excluded`, required USD quotes, and stored one price per
Platform and Canonical Asset per cycle attempt. The Jobs API and Activity's Jobs
tab were removed; cycles expose attempts and the last error. Cycle detail
returns the cycle and any Point summary while the cycle is queued, running or
failed and after its holdings expire. Latest-price selection and the
stale-source quote pin were rewritten after the capacity load showed their
per-row lookups scanning whole histories.

The third review merged Futures Contract versions into `futures_contracts`: each
row is one symbol with one immutable set of observed terms, and every row is
kept. Manual Account Reports reference the revision they reported instead of
copying per-cycle inputs; `manual_reported_at` marks a cycle's completed Manual
reports. Stored Point issues and boundary totals gave way to read-time
derivation, leaving `manual_balance_changes` and sparse
`calculated_asset_changes` with an explicit treatment. Capital Flows name their
pricing method and pricing attempt, Assets record observation bounds,
zero-equity pricing and user-requested cycles have their own values, retained
daily prices are anchors, and the shortest collection interval is 900 seconds.
The capacity load then showed correlated latest-change lookups sorting each
Asset's whole history, mapping-change reads walking every Asset's later changes,
and the applied Manual revision scanning the Portfolio's reports; all three were
rewritten.

## Verification

Verified on 2026-10-10 after the third review:

- The ordered `make format`, `make web`, `make build`, `make test`, and
  `make lint` gates passed after the last change: 31 Go packages on isolated
  temporary MySQL databases, 193 frontend unit tests, and no golangci-lint,
  TypeScript or ESLint findings. A Linux amd64 build passed.
- Re-running `make generate` left all 903 tracked and untracked files
  byte-identical. Markdown formatting and 562 local paths/heading links were
  checked.
- Regression tests cover immutable retries, comparison preservation after
  cleanup, timezone-stable daily anchors, cross-day fallback, stale source and
  quote pins, pruning interruption/restart, complete asynchronous deletion and
  rejection of in-flight collection writes after its deletion request. They also
  cover captured Identity versions, mapped-price collection ordering, manual USD
  overrides, Unicode/optional text limits, and password/session races. The
  retention test keeps a quote inside an Account's last successful report window
  while deleting an older unreferenced one; an HTTP test reads running, failed
  and holdings-expired cycles. Both fail against the previous code.
- Third-review tests cover Futures Contract reuse across A → B → A terms with
  widened observation bounds, Manual adjustments from reported revisions across
  equal evaluation times and read batches, boundary totals on listed cycles, and
  an archived Identity keeping its applied Manual revision. Seeding the fake
  Portfolios slowed from 41.7 s to 53.7 s because each Point commit now reads
  back its derived Account issues and comparison evidence.

### Capacity load

A throwaway `plutus_gotest_capacity` database on MySQL 8.4.11 held one Portfolio
with 2 Identities. Each had 5 Platform Accounts of 10 Balances and 16 Futures, 3
Earn and 1 Dual Positions, plus an empty Manual Account revised weekly. It had
365 days of 15-minute cycles and Points and about 244 prices per cycle across
four Platforms. Comparison facts were conservative: daily Manual adjustments,
weekly Boundary Events, and a weekly calculated change per Asset. Sixty days had
detail; bulk SQL simulated pruning 30 of them to daily anchors. This measured
storage after pruning, not the production worker's cleanup throughput. Per-Point
costs came from `innodb_index_stats`. Holdings tables were scaled tenfold to 20
Identities × (50 Balances + 20 Positions) with their Manual Accounts. Prices
were not scaled because they are shared per Platform and Canonical Asset; this
assumes about 61 priced Assets per Platform. The
[planning scale](../architecture.md#persistence-and-retention) owns the
resulting 3.0 GB window and 8.9 GB ten-year totals; permanent comparison facts
take about 0.2 GB of them.

`EXPLAIN ANALYZE` on the pruned load:

| Read                                  | Access path                                       | Round trip |
| ------------------------------------- | ------------------------------------------------- | ---------- |
| Cycle claim                           | `uq_collection_cycles_active` range               | 0.8 ms     |
| Latest prices, 50 symbols             | Covering 24-hour selection range, ranked, PK join | 5.1 ms     |
| One-year history, 35,040 Points       | `idx_valuation_points_timeline` range             | 27.6 ms    |
| 30-day comparison timeline            | Timeline range                                    | 4.5 ms     |
| Report retention page, 64 rows        | Covering `idx_account_reports_retention` range    | 2.1 ms     |
| Report pin                            | Covering source-report and fallback lookups       | 2.1 ms     |
| Applied Manual revision               | Backward fallback range, first row                | 1.7 ms     |
| Prunable Manual revision              | Primary key and revision-reference anti-join      | 2.5 ms     |
| Latest calculated changes, 120 Assets | Backward primary-key row per Asset                | 3.1 ms     |
| Point calculated Assets               | Primary-key lookups                               | 3.2 ms     |
| Unpinned price predicate              | Price lookups and a skip scan per Account         | 2.5 ms     |

Comparison facts were then grown to a pessimistic ten-year size: 1,200 Assets
with about 634,000 calculated changes and 70,080 Manual adjustments; five Assets
had a newer mapping version. The latest-change read at Point commit took 16.8
ms. A one-year range read in 137 batches of 256 Points took 0.20 s for mapping
changes, 0.15 s for Manual adjustments, and 0.09 s for Boundary Events.
Calculated Assets at a one-year-old start took 130 ms: MySQL cannot seek a
correlated lookup to an outer-row bound, so each Asset walks back over its later
changes.

Before the rewrites, the same mapping-change read took 24.6 s and the
latest-change read 235 ms: unless `ORDER BY` names the full primary-key prefix,
MySQL sorts each Asset's whole history. The applied Manual revision preferred a
Portfolio-wide report scan, 21.8 ms with only 2 Identities, until its
per-Account index was forced. These are single-host plan checks, not a benchmark
on the target 4-core/8-GB machine.

### Agent runtime

The embedded SPA on a freshly seeded, fake-backed `plutus_agent` instance at
:8081 was used after the third review. Overview, Activity, Catalog and History
loaded without errors; Catalog shows Last observed, and cycles list boundary
values and the User trigger. Collection interval presets start at 15m; 600
seconds shows the 900-second minimum and clamps to 900 on blur; the exercise
found and fixed a stale "(1m)" label on that minimum.

A Manual balance of 3 USD saved as pending version 2 while version 1 stayed
applied. Collect now applied version 2, raised the total by 3 USD, and showed
`manual_balance_changed`. Remapping Binance USDC to USDT and collecting again
showed `asset_mapping_changed` on the new Point; the day's Value Change listed
both reasons. A Capital Flow with an explicit 7 USD value returned `explicit`
pricing with an attempt time and was deleted; Audit recorded the Manual, mapping
and flow changes. The server logged no errors. Session-started processes were
stopped; :8081 had no listener and its health endpoint was unreachable. The
:8080 instance was not used.

Capacity tooling, gate and runtime logs are under `/tmp/plutus/schema-impl/`. No
new live exchange recording or exchange-app reconciliation was performed.
Existing real evidence and its gaps remain owned by each Platform verification
document. Synthetic Adapter tests and this fake-backed UI exercise do not
establish live exchange correctness.

## Follow-up review on 2026-10-11

This review keeps the 26-table model, financial precision, scoped foreign keys,
and retention boundary. It renames the Manual comparison table to
`manual_balance_changes` and the report ownership key to
`uq_account_reports_account_report`. The existing Point timeline index now also
covers `calculation_version`; no summary table or additional index was added.

Manual audit events now contain the changed Asset and old/new quantity, together
with revision references, actor and time. Exclusion audit reads reconstruct the
historical state and reason from immutable rule versions in the same read
transaction. The existing Activity payload dialog renders both without an API
shape or frontend change. Tests remove an old Manual revision before reading its
audit and exercise exclusion edits across cursor pages and Portfolio boundaries.

Retention now checks report pins per 64-row page, deletes at most 1,000 children
per table per visit, and removes only empty parents. An unfinished large report
keeps the cursor from skipping it. Quote deletion shares a 64-row visit; Point
cleanup removes eligible children and advances its state in one transaction.
Tests cover oversized reports, page boundaries, stale-source pins, daily
anchors, active collection and restart continuity. A Manual revision search that
found no candidates now stays exhausted until the retention epoch changes.

### History service measurements

The fixture contained 350,400 Points, 20 Identities, 120 Accounts, 1,000 current
Balances, 400 current Futures Positions and 50 stable Assets, with quarterly
comparison events. Chart and calendar reads select compact headers first;
comparisons hydrate only event candidates. Calculation-version transitions still
include their predecessor, including A → B → A and equal-time Cycle ordering. A
600-Point database regression also covers expired detail, mapping transitions,
Manual changes, unknown boundary value and calendar gaps.

| Service                     |  Median | Maximum of five repeated samples |
| --------------------------- | ------: | -------------------------------: |
| Current Overview            |   24 ms |                            25 ms |
| Recent three-day series     |  1.3 ms |                           1.5 ms |
| Recent Point page           |  1.2 ms |                           1.2 ms |
| Ten-year custom comparison  | 1.170 s |                          1.185 s |
| Ten-year daily chart        | 0.252 s |                          0.254 s |
| Ten-year yearly closes      | 0.285 s |                          0.287 s |
| Ten-year yearly comparisons | 0.608 s |                          0.611 s |

Yearly comparisons initially took 5.240 s. Query plans showed Portfolio-wide
primary-key scans for prepared range reads; the covering timeline and explicit
index selection removed that cost. Measurements used local Darwin/arm64 with 10
logical CPUs and MySQL 8.4.11 with a 128 MiB buffer pool, after an initial pass.
They measure application service calls, not HTTP/network latency. Historic
holdings were omitted from this fixture; the retained-storage experiment below
is separate. These results do not certify a 4-core/8-GB host or cold-cache reads
under concurrent collection.

### Seed initialization

The complete reset measured 9.08 s before optimization and 7.191, 6.038 and
6.328 s afterward: a 6.328 s median, about 30% below the single baseline run.
The dataset still contains 650 Points. Approximately 13,019 SQL statements fell
to 5,514; 4,939 historical price selections became four bulk history reads.
Manual comparison reads fell from approximately 1,297 to 24. Source and Point
initialization takes about five seconds; the whole reset has not returned to
five seconds.
[Seed verification](../seeds.md#performance-measurement-2026-10-11) owns the
detailed parity evidence and measurement conditions.

### Agent runtime verification

A reset of the fake-backed `plutus_agent` applied the revised baseline. The
embedded SPA on :8081 displayed Overview, raw History, daily closes and daily
Value Changes. API checks created a disposable Manual Asset at 123, changed it
to 456, then cleared it; all three audits retained the corresponding quantities.
The Activity payload dialog displayed the 123 → 456 change. Excluding and
restoring BTC retained the prior reason in the restore audit and displayed it in
the same dialog. A custom comparison API request returned a complete result with
the expected seeded endpoints. Native datetime input automation did not submit
the custom form successfully; that interaction was not accepted as UI coverage
for custom comparison.

The agent server and child process group were stopped; :8081 had no listener and
health was unreachable. :8080 was not used. No live exchange requests,
recordings or exchange-app reconciliation were performed.

### Full retained capacity

A separate throwaway MySQL database loaded the full fixed workload: 20
Identities, six Accounts each, 50 Balances across Accounts and 20 combined
Positions per Identity. Each detailed Point contains 1,000 Balances and Balance
Valuations, 400 Positions and 120 Account Reports/Valuations. The physical
fixture uses 16 Futures, three Earn and one Dual Position per Identity, plus 244
prices per cycle across four Platforms. It also contains daily Manual change
markers, weekly calculated-Asset changes, weekly Manual revisions and ten audit
records per day. These are synthetic storage and retention inputs, not
reconstructed exchange history.

There are 350,400 permanent summaries, representing ten 365-day years. Before
cleanup, 3,619 daily snapshots plus 31 complete days retain 6,595 detailed
Points. The initial load occupied 7,176,142,848 bytes; the final restored-day
replay started and ended at 7,201,308,672 bytes. These are actual
application-table `FILE_SIZE` and `ALLOCATED_SIZE`, excluding fixture helper
tables and migration bookkeeping, not an extrapolation from a smaller database.
Data/index page statistics vary with `ANALYZE` estimates; physical files are the
disk-budget reference.

The full load exposed repeated empty Manual revision searches over 10,440
still-referenced revisions: three isolated reads took 244.4, 245.3 and 245.9 ms.
Remembering an exhausted search avoids repeating that work every phase rotation;
cutoff changes and released report pins reopen the search. A first completed run
stopped at its first idle result, but a later retention epoch can require
another sweep. The final timing therefore includes settlement through two
consecutive idle phase cycles.

The final run restored the complete expiring day before timing and executed the
real `retention.Service.Step`. It preserved 350,400 summaries and reduced detail
from 6,595 to 6,500 Points: 6.5 million Balances and Balance Valuations, 2.6
million combined Positions, 780,000 Account Reports and Account Valuations, and
1,586,000 prices. It removed 95 intraday snapshots while retaining one daily
snapshot and the last 30 complete days. It also traversed older pinned reports
and weekly Manual revisions.

Stable settlement required 165,084 Steps and **168.694 s of measured service
execution**. Median Step time was 0.691 ms, p95 1.433 ms, p99 1.785 ms and
maximum 270.819 ms. Applying the worker's 16-Step/250-ms round limit, one-second
polling interval and two intervening 60-second idle backoffs gives 10,388 worker
visits and **2 h 55 m 6 s of modeled paced duration**. The harness did not sleep
through that schedule; collection interruptions and target-host contention add
time. This is one expiring day's cleanup at the ten-year retained size,
including follow-up sweeps, not the cost of processing ten years of accumulated
backlog.

After cleanup, physical application files occupied 7,201,308,672 bytes (**7.201
GB / 6.707 GiB**). Reported data pages were 3,928,424,448 bytes and
secondary-index pages 2,967,191,552 bytes; 109,051,904 bytes were reported free
inside tablespaces. Files did not shrink. The measurement retains allocated
pages from earlier sweeps and restored days, and excludes fixture helper tables.
The largest application table file, `balance_valuations`, was 2,801,795,072
bytes; the whole-machine budget includes room for a single-table maintenance
copy.

The completed load also exposed report-query access paths. At a day-1,500
cursor, candidate selection scanned 180,064 index rows to return 64 (53 ms in
`EXPLAIN ANALYZE`); selecting the existing retention index used a 64-row range
(0.025 ms). Last-successful-report comparison scanned 1,502 rows per candidate;
selecting the existing fallback index with its complete reverse order read one
row per candidate. No new index was needed.

Position composition changes storage density. Using observed allocated bytes per
row and keeping other tables fixed projects these ten-year totals:

| All 400 Positions at each detailed Point | Projected physical application files |
| ---------------------------------------- | -----------------------------------: |
| Futures                                  |                             7.150 GB |
| Earn                                     |                             7.568 GB |
| Dual Investment                          |                             6.924 GB |

These are density projections from the mixed load, not three separately loaded
ten-year databases. Earn JSON occupied 384 bytes per row in the fixture; larger
product documents and longer strings change the result. Price history assumes
the same 244 observations per cycle and shared, mostly stable markets.

The archive was bulk-loaded in chronological order. It does not reproduce ten
years of allocation, deletion, page reuse or abrupt downtime. Earlier pruned
measurements showed greater per-anchor page cost; the
[whole-machine budget](../deployment.md#mysql-capacity) therefore reserves space
above this dense load. Stale-source and quote pins are covered by regressions,
not extra failed Accounts in this capacity fixture. The workload assumes mostly
stable Assets, occasional turnover and bounded product documents; it is not an
upper bound for unlimited product metadata, catalog creation or User activity.

### Final validation

The ordered format, web build, Go build, uncached Go/frontend tests and lint
sequence passed after the last code change. A Linux amd64 build also passed. All
193 frontend tests passed; golangci-lint reported zero issues, and
TypeScript/ESLint passed. Generation left all 906 tracked and untracked source
files byte-identical. The tests used throwaway MySQL databases and loopback
replay; no database tests were skipped for missing configuration.

The baseline remains editable only before the first production migration.
Existing disposable development databases must be reset to this baseline;
production upgrades must follow the
[migration safeguards](../deployment.md#upgrades-and-resets). Evidence and
reproducible temporary harnesses are under `/tmp/plutus/prelaunch-20261011/`
(`gates-final.log`, `runtime-check.json`, `history/`, `retention/`, and
`seeds/`).

## Rolling retention follow-up

The current policy supersedes the retention horizons used in the preceding
measurements. Every committed summary and comparison fact remains for ten
calendar years at a default collection interval of 30 minutes. Complete detail
remains for at least 14 days, then one complete daily snapshot. Summary reads do
not downsample after day 14. Additional manual collections retain their own
summaries.

The 26-table design gains one nullable Portfolio field, `history_before`, to
persist the supported history boundary independently of background deletion
progress. It prevents internal sparse predecessors from appearing as complete
older history and never moves backward. Current configuration, last-good
sources, the opening snapshot and required Asset/policy predecessors are
protected. Historical audits and unused Capital Flows now expire with the
window. The exact rules belong in
[domain retention](../domain-rules.md#historical-detail-retention).

### Rolling capacity and cleanup

A disposable MySQL 8.4.11 fixture loaded 3,660 days at 48 collections per day:
20 Identities, 120 Accounts, 1,000 Balances and 400 Positions per detailed
point. The Position mix was 320 Futures, 60 Earn and 20 Dual Investment, with
384-byte Earn JSON. It also included 244 shared price observations per retained
detailed point, weekly policy-state changes, daily Manual markers and ten audit
events per day. Old daily snapshots and the last 15 full days were loaded before
the actual retention service ran across the ten-year and 14-day boundaries.

| Result after cleanup                                 |         Observed count or size |
| ---------------------------------------------------- | -----------------------------: |
| Summaries inside the ten-year window                 |                        175,296 |
| Additional internal predecessor Points               |                              2 |
| Complete snapshots, including the opening dependency |                          4,311 |
| Account Reports / Account Valuations                 |                   517,320 each |
| Balances / Balance Valuations                        |                 4,311,000 each |
| Positions across all three products                  |                      1,724,400 |
| Price Observations                                   |                      1,051,880 |
| Physical application table files                     | 4,823,105,536 bytes (4.823 GB) |
| Allocated index pages / used leaf pages              |            4.556 GB / 4.313 GB |

Actual service execution took 267.28 seconds for 302,544 Step calls, including
candidate sweeps and two idle observations. The first idle observation occurred
at 168.93 seconds. This deliberately omitted worker pacing and is not a promise
for background completion time on the target machine. Candidate pages remain 64
rows and child deletes at most 1,000 rows per statement.

The application files occupied 4,823,089,152 bytes before cleanup. Deletion did
not shrink the table files: free pages remain available for reuse. Bounded
retention limits live history; it does not promise a constant filesystem size
without fragmentation or maintenance overhead. The
[whole-machine budget](../deployment.md#mysql-capacity) still reserves space for
the OS, bounded logs and a maintenance copy, with backups off-machine.

Using observed allocated page density to replace the mixed Position rows gives
approximately 4.80 GB for all Futures, 5.04 GB for all Earn and 4.55 GB for all
Dual Investment, keeping other tables fixed. These are projections, not three
separate full loads. Longer product metadata, more markets and additional
collections change storage use. The dense chronological load does not simulate
ten years of fragmentation or sustained concurrent production traffic.

### Query and runtime verification

A separate final-code fixture measured 175,296 half-hour summaries, 20
Identities, 120 Accounts, 1,000 current Balances, 400 current Futures and sparse
comparison events. It exercised the persisted history boundary. Historical
holdings were omitted from this latency fixture; the full archive above owns
storage evidence. The host exposed ten CPUs and a 128 MiB MySQL buffer pool; it
was not constrained to 4C8G.

| Application service call       | Median of five repeated calls | Slowest repeated call |
| ------------------------------ | ----------------------------: | --------------------: |
| Current Overview               |                      25.38 ms |              25.56 ms |
| Recent three-day raw summaries |                       1.13 ms |               1.18 ms |
| Recent summary page            |                       1.23 ms |               1.32 ms |
| Ten-year custom Value Change   |                     307.91 ms |             318.67 ms |
| Ten-year daily chart           |                     119.60 ms |             147.20 ms |
| Ten-year annual closes         |                     161.11 ms |             169.34 ms |
| Ten-year annual Value Changes  |                     477.55 ms |             744.15 ms |

These are local service timings, excluding HTTP serialization and user network
latency. They support the query design but do not establish the one-second /
five-second acceptance targets during normal collection on the target machine.

The ordered format, web, build, test and lint gates passed, including 193
frontend tests. Regressions cover restart, exact-cutoff flows, preserved
comparison results, audit-version references, applied Manual state, dormant
current Positions, and a retained cycle whose start precedes the window.
Regeneration left all 399 checked Go and API TypeScript files byte-identical.

The embedded SPA on :8081 showed the 1,800-second default in Settings and loaded
History. Final-code API checks confirmed old intraday summaries remain readable
without detail, while the daily snapshot retains its Accounts. All temporary
databases were dropped, and the session-started agent process group was stopped;
health was unreachable and :8081 had no listener. No live Platform verification
was performed. The earlier seed optimization and its timings remain unchanged.

Evidence and temporary harnesses are under
`/tmp/plutus/rolling-retention-20261011/`: `capacity-final.log`,
`history-final.log`, `runtime-check-final.log`, `lint.log`, and the capacity and
history Go overlays. The capacity load preceded the last dormant-position and
cross-cutoff-policy edge fixes; both fixes passed the final regression gates,
and the query fixture ran against the final code.

### Retention safety review

A second review traced every application table's lifetime, foreign-key and
semantic references, and the readers of comparison evidence. It found no
additional production defect in the rolling cleanup. The review added
`TestRollingHistoryReleasesReferencesAcrossThirtyYears` in
[`retention_history_test.go`](../../internal/integration/retention_history_test.go)
to exercise repeated release of old dependencies, rather than only pruning one
initial archive.

The sparse fixture advances annually from 2000 through 2034. Holding count stays
fixed while Assets and Futures Contracts rotate, one stable Asset alternates
exclusion rules, audit events refer to both rule versions, and Capital Flows
contribute nonzero adjustments. A failed report's daily snapshot continues to
use the preceding successful intraday report. The actual valuation service
commits each point and the actual retention service cleans each successive
window; the service restarts once midway through the scenario.

For every window, the regression checks that cleanup preserves comparison
amounts, reasons and Boundary Events, current and historical Positions remain
readable, and exclusion audits still resolve their version facts. Fourteen
history, source, catalog and policy tables have identical retained row counts
from 2013 through 2034. This is a logical lifetime test with sparse annual data,
not a second ten-year volume or hardware benchmark. Existing regressions cover
Manual revisions, Earn/Dual children, oversized batches, active-cycle guards,
partial-detail visibility and restart.

The lifetime audit distinguishes time-driven history from current configuration:

| Tables                                                                                                                             | Lifetime bound or deliberate exception                                                                                    |
| ---------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| `valuation_points`, `collection_cycles`, `boundary_events`, `manual_balance_changes`, `calculated_asset_changes`                   | Ten-year window plus opening/current state dependencies; sparse Asset predecessors can be older.                          |
| `account_reports`, `account_valuations`, `balance_valuations`, `balances`, `futures_positions`, `earn_positions`, `dual_positions` | Recent full detail, daily snapshots, and referenced or last-good sources.                                                 |
| `price_observations`, `futures_contracts`                                                                                          | Retention horizon plus adopted references; prices also preserve last-good source windows.                                 |
| `manual_revisions`, `manual_revision_balances`                                                                                     | Current revision and revisions referenced by retained Reports; the initial revision can remain.                           |
| `capital_flows`, `audit_events`                                                                                                    | Historical window, with flows after the opening point retained for the first comparison interval.                         |
| `asset_exclusion_versions`, `assets`                                                                                               | Required historical evidence and current configuration; default unused Assets expire, explicit mappings and rules remain. |
| `sessions`, `idempotency_records`                                                                                                  | Expiry/revocation cleanup in hourly bounded batches.                                                                      |
| `users`, `portfolios`, `identities`, `accounts`                                                                                    | User-created configuration; not aged out merely because it is old.                                                        |
| `goose_db_version`                                                                                                                 | Schema migration bookkeeping, independent of collection frequency.                                                        |

Under the agreed fixed configuration and continuing collection workload, the
review found no table that must keep accumulating rows solely because another
year passes. Unbounded creation of Identities or explicitly configured Assets is
still unbounded configuration growth. A long collection gap can keep opening
comparison dependencies older than ten years; deleting those references solely
by age would be incorrect. Row lifetime bounds do not guarantee a fixed table
file size or a hard disk quota.

The full ordered validation gates passed after the regression was added,
including all Go tests and 193 frontend tests. This review changes only tests
and this evidence record; it does not change runtime behavior or invalidate the
previous runtime verification. Logs are under
`/tmp/plutus/retention-review-20261011/`.
