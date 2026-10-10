# Prelaunch schema and history delivery

The baseline defines 26 tables. Collection Cycles carry their own attempts and
leases, Portfolio deletion is driven by the root's request, and every
Portfolio-owned primary key starts with `portfolio_id`. Other responsibilities
remain separate; `manual_adjustments` and sparse `calculated_asset_changes`
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
derivation, leaving `manual_adjustments` and sparse `calculated_asset_changes`
with an explicit treatment. Capital Flows name their pricing method and pricing
attempt, Assets record observation bounds, zero-equity pricing and
user-requested cycles have their own values, retained daily prices are anchors,
and the shortest collection interval is 900 seconds. The capacity load then
showed correlated latest-change lookups sorting each Asset's whole history,
mapping-change reads walking every Asset's later changes, and the applied Manual
revision scanning the Portfolio's reports; all three were rewritten.

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
detail; retention then pruned 30 of them to daily anchors. Per-Point costs came
from `innodb_index_stats`. Holdings tables were scaled tenfold to 20 Identities
× (50 Balances + 20 Positions) with their Manual Accounts. Prices were not
scaled because they are shared per Platform and Canonical Asset; this assumes
about 61 priced Assets per Platform. The
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
