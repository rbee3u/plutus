# Architecture

Plutus is a contract-first Go monolith with an embedded React frontend and an
externally managed MySQL database. This document describes runtime boundaries
and core mechanisms. [Glossary](../GLOSSARY.md) defines terms,
[domain rules](./domain-rules.md) define business behavior,
[presentation](./presentation.md) defines display and interaction rules, and
[decisions](./adr/README.md) explain trade-offs.

## Deployment and dependencies

```text
Browser
  |
  | HTTP or HTTPS; JSON under /api/v1
  v
plutus serve
  +-- HTTP transport and embedded React SPA
  +-- application services --------------------> MySQL
  +-- scheduler and collection worker ---------> MySQL
  +-- typed platform adapters -----------------> Platforms or fake data
```

One application process serves the API and SPA, schedules collection, and runs
collection cycles. Only one process may be active for a database. Cycle leases
support restart recovery; they do not provide multi-instance coordination or
exactly-once collection. Plutus does not manage MySQL installation, backups, or
upgrades.

The backend uses Gin, `database/sql`, sqlc, and Goose. The frontend uses React,
TypeScript, React Router, TanStack Query, ECharts, CSS Modules, and semantic CSS
custom properties. Toolchain requirements and pinned versions live in
[go.mod](../go.mod), [web/package.json](../web/package.json),
[web/package-lock.json](../web/package-lock.json), and [Makefile](../Makefile).

## Module responsibilities

`internal/app` assembles shared services on one database handle; CLI owns the
HTTP server, worker, and process lifecycle. Services share sqlc queries and
transaction helpers without exclusive table ownership. HTTP maps generated wire
types to application types; services do not depend on Gin or HTTP types.

Depguard enforces the [backend boundaries](./adr/0007-backend-architecture.md):
application code uses the normalized adapter contract, `valuation` cannot import
`financial`, and HTTP cannot import SQL APIs. Runtime code cannot import
`internal/testsupport`. Within-layer dependencies remain reviewed choices.

| Module                                                                | Responsibility                                                         |
| --------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| `internal/app`                                                        | Composition root: every service built on one database handle           |
| `internal/cli`, `cmd/plutus`                                          | CLI commands; `cmd/plutus` only starts them                            |
| `internal/seeds`                                                      | Development seed fixtures for `db seed` and `db reset --seed`          |
| `internal/transport/http`                                             | API bindings, browser security, HTTP conventions, and SPA serving      |
| `internal/application/auth`                                           | Users, password verification, and sessions                             |
| `internal/application/portfolio`, `internal/application/identity`     | Ownership, portfolio settings, identity configuration, and connections |
| `internal/application/catalog`, `internal/application/assetexclusion` | Assets, futures contracts, prices, and exclusion policy                |
| `internal/application/collection`                                     | Adapter orchestration and normalized report persistence                |
| `internal/application/valuation`, `internal/application/capital`      | Immutable value writes and manual capital flows                        |
| `internal/application/financial`                                      | Overview, Holdings, history, periods, and financial presentation       |
| `internal/application/cycles`, `internal/worker`                      | Cycle queue, scheduling, leases, and portfolio deletion dispatch       |
| `internal/application/audit`                                          | User and system action records                                         |
| `internal/platform`                                                   | Typed platform contract, live implementations, and deterministic fake  |
| `internal/infra`, `internal/shared/decimal`, `internal/infra/secret`  | SQL infrastructure, exact arithmetic, and credential encryption        |

The financial service reads stored points and account valuations through the
valuation service, sharing decoding, account issues, and an operation-scoped
`PriceBook`. Readers aggregate stored decimals; writers retain unrounded
rationals privately. Internal source metadata is excluded from JSON. Product
filters, totals, and completeness stay separate. `application/ordering` uses the
offered platform catalog; `shared` has no business dependencies.

Collection loaders feed one valuation evaluator with prepared inputs, without
SQL, clocks, or query callbacks. The `app` package injects shared catalog,
manual, and adapter registry instances into their consumers.

Catalog commits each mapping and its audit event in one transaction. The next
valuation adopts it. No mapping write enqueues historical work.

Portfolio deletion sets `portfolios.deletion_requested_at`; that state is the
only deletion record. All business writers lock the active root, including
in-flight collection persistence. Scheduling excludes marked roots. When no
cycle is claimable, the worker locks the oldest marked root with `SKIP LOCKED`
and removes at most 1,000 rows from one dependency table per transaction, with
foreign keys enabled; the last transaction removes the root. A failed batch is
retried on the next tick, and restarts resume from the remaining rows.
User-scoped replay records retain their independent lifetime.

See [test ownership](./testing.md#test-ownership) for cross-module workflows.

### Persisted asset references

Catalog objects own `symbol`; references name their business role. Contract
terms live in `futures_contracts`, positions in `futures_positions`, and dual
principal currency in `principal_asset_symbol`. Reported currencies remain
source codes, distinct from canonical asset pricing mappings. Only record IDs
have Catalog foreign keys; a quote or settlement currency need not be held.

Symbols use trimmed NFC uppercase text, at most 64 UTF-8 bytes. Opaque platform
references retain their exact bytes, with a 255-byte limit and no truncation.
Usernames and display names retain NFC spelling while dedicated NFC case-folded
keys enforce uniqueness. Binary collation preserves accents and trailing
characters outside that explicit normalization. Account labels come from the
capability catalog; accounts have no independently editable display name.

## Frontend structure

The [frontend boundaries](./adr/0013-frontend-architecture.md) separate `app`,
`features`, `shared`, and `api`, enforced by ESLint. Features expose explicit
entrypoints; API consumers use `api/index.ts`. Feature CSS stays with its owner;
compound selectors and cascade-sensitive rules stay within one CSS module.

QueryClient owns the authenticated user and user-scoped portfolio list. The auth
boundary waits before mounting features; an auth-me 401 means no user, while
transport failures remain retryable. Login seeds the cache. Logout, expiry, and
password success cancel queries, clear cached data and CSRF state, and return to
login. Generation-bound expiry callbacks prevent a late 401 from ending a
replacement session, including a new login by the same user.

Shared writes acquire CSRF and handle expiry, except a password
`invalid_credentials` rejection. Features retain raw SDK results and their
validation, conflict, ETag, and idempotency policies. Cache keys and collection
invalidation flags are shared; editors retain independent lifetimes. Session
lists use a user-scoped cache discarded when unused or logged out, with reads on
entry and revocation but no background retry, focus, or reconnect refresh.

Holdings shares request conversion, recovery, pagination, and account report
reads; pages own product semantics. Shared navigation prepares outgoing drafts,
while Holdings owns view-memory policy. Charts explicitly import common ECharts
registration. See [presentation](./presentation.md) for interaction rules and
[test ownership](./testing.md#test-ownership) for browser fixtures.

## Collection cycle lifecycle

One durable `collection_cycles` row is both the financial history record and its
queue entry: trigger, status, attempt count, availability, lease, and the last
attempt's error. The stored `active_portfolio_id` column is non-null only while
queued or running, and its unique key allows one active cycle per portfolio;
`Enqueue` locks the portfolio and returns that cycle instead of adding another.
The worker polls each second, claims one available queued cycle or expired
running lease with `SKIP LOCKED`, and renews two-minute leases. Interrupted
attempts requeue with backoff; a fourth claim fails the cycle. Recovery keeps
committed reports, including failures, and collects missing accounts. Financial
completion commits the point, account valuations, and cycle success in one
transaction after checking the running lease; success clears the error fields.
Lease loss cancels unfinished work. This remains a single-process design;
platform calls are not exactly once.

A cycle establishes its selected accounts, then collects each usable connection
with a fixed three-second delay between identities. Adapters perform the
applicable structure and margin mode checks. Successful normalized reports,
holdings, and prices are persisted; failed or omitted accounts receive failure
reports. Each attempt stores at most one price per platform and canonical asset;
later identities on that platform reuse it. Each identity commits its reports,
missing-report failures, margin mode audit, and collection outcome in one
transaction after its platform reads. Recovery keeps committed observations;
uncommitted work uses the current boundary and connections. Valuation then
selects compatible source reports, calculates account values and boundary
events, and writes one valuation point for the cycle.

Temporary and rate-limit adapter failures have bounded retries within the cycle.
`unsupported` failures, including detected structure mismatches, are not retried
within that run. Enabled connections remain scheduled for later cycles.
Authentication and authorization failures invalidate the connection. Platform
failures can still produce a point with stale or missing values. Database or
other infrastructure failures can interrupt collection before a point exists.
The full stale, completeness, and value rules are in
[domain rules](./domain-rules.md#collection-and-account-reports).

## Manual persistence and collection

`manual_revisions` is the only editable-ledger history: mutations append a
revision while locking its account and checking the revision number. Initial
revision 1 is empty. Identity versions guard account selection changes.
`manual_revision_balances` retains signed nonzero quantities by asset ID.

Before platform reads, the runner reads the current revision of every boundary
manual account and fetches public prices outside any lock; revisions are
immutable, so pricing cannot diverge from the stored reference. One transaction
then locks the portfolio and the cycle's `manual_reported_at`, records prices,
inserts each manual report with its `manual_revision_number`, and sets the
marker, even when the boundary contains no manual accounts. A set marker skips
this step, so retries cannot capture later additions. Mappings are not frozen.

The separate `MarketData` capability fetches public prices without credentials
or private preflight, grouped by platform and required canonical asset symbols.
Failure leaves quantities and eligible cached prices available. Manual reports
bypass adapter `Collect` and private identity success counters. Live valuation,
reads all use the own-platform lookup.

Point commit compares the revision quantities adopted by consecutive points and
stores changed manual accounts in `manual_balance_changes`; revision numbers,
timestamps, and mappings do not establish an adjustment. The ledger API exposes
current/last-applied quantities, versions, and pending state. Financial reads
merge current manual records with the selected report before filtering, sorting,
and pagination. Added, updated, and deleted row metadata is derived from
quantities, never persisted; report values and historical totals are unchanged.
Manual writes invalidate balance and Overview reads without starting collection.
[manual rules](./domain-rules.md#manual-balances) own continuity and
empty-account semantics; [deployment](./deployment.md#upgrades-and-resets) owns
existing-data upgrades.

## Adapter boundary

The ordered `platform/adapters` catalog owns offered platforms, account
structures, account types, margin modes, and finite browser credential metadata.
Lookups return copies of mutable descriptions. Runtime registration determines
availability separately; CLI keeps live/fake assembly explicit, and recording
has its own assembly. Browser forms do not replace server validation or expose
stored credentials.

Adapters accept typed credentials, declared structure, and selected account
types, returning normalized reports and market observations. Application code
does not depend on raw response shapes. Runtime calls use only read-only
account, balance, position, and market-data interfaces. Errors may name an
endpoint and platform error code; raw bodies and credentials never enter logs or
storage.

The shared transport clones the default without global mutation, clears its
proxy callback, and applies the validated explicit instance policy. Adapter
request budgets and caller-supplied clients retain precedence. Kraken WebSocket
uses the same HTTP client; direct connections and proxy first hops use IPv4.
Proxy failure never falls back to direct, and proxy availability is not a
readiness dependency. External software owns tunnels. Recording reuses proxy
validation and the client factory but loads no runtime secrets, logging, or TLS
settings. [Development](./development.md#platform-proxies) owns configuration.

Dev registers the fake; remote test/prod register live adapters. Automated tests
use independent synthetic responses and a network guard. Only explicit recording
writes sanitized responses under `/tmp/plutus/`; see the
[evidence workflow](./platform/evidence-workflow.md) and
[platform mappings](../README.md#documentation).

## Development scenarios

Seed history and ongoing fake collection share a pure scenario generator.
Reserved `plutus-demo-v1` credentials carry a scenario ID and UTC anchor;
validation binds a stable external reference. Prices keyed by portfolio,
platform, canonical asset symbol, and time remain independent of login identity
within that portfolio.

Seeds insert source facts and initialize their points in one transaction, then
initialize connections through real services. A seed-only history initializer
shares the collection writer's calculation but loads sources in cycle batches,
carries the previous point in memory, and skips per-account evidence rereads.
Its finite, immutable price and exclusion histories are loaded once per
portfolio; price selection preserves source-time ordering, retrieval cutoffs,
and the 24-hour window. Manual change detection runs per cycle batch. Seed tests
compare its edge-case rows and price selections with the collection writer.
Historical Plutus events stop at the anchor; continued fake holdings cannot
mutate flows, selection, archival, or Catalog policy. No persistent simulator or
background application hook is needed. [Seeds](./seeds.md) owns scenarios,
coverage, and performance evidence.

## Persistence and retention

The [baseline](../db/migrations/20261010160000_schema.sql) is the authoritative
field, key, and constraint definition for 26 tables:

| Responsibility                   | Tables                                                                                 |
| -------------------------------- | -------------------------------------------------------------------------------------- |
| Access and request replay        | `users`, `sessions`, `idempotency_records`                                             |
| Ownership and configuration      | `portfolios`, `identities`, `accounts`                                                 |
| Portfolio market catalog         | `assets`, `futures_contracts`, `price_observations`                                    |
| Collection and execution         | `collection_cycles`                                                                    |
| Immutable account facts          | `account_reports`, `balances`, `futures_positions`, `earn_positions`, `dual_positions` |
| Derived financial results        | `valuation_points`, `account_valuations`, `balance_valuations`, `boundary_events`      |
| Durable comparison evidence      | `manual_balance_changes`, `calculated_asset_changes`                                   |
| Manual and policy versions       | `manual_revisions`, `manual_revision_balances`, `asset_exclusion_versions`             |
| User financial records and audit | `capital_flows`, `audit_events`                                                        |

Every portfolio-owned primary key starts with `portfolio_id`: surrogate-key
tables use `(portfolio_id, id)` and child tables extend their parent's key.
Scoped composite foreign keys prevent references across portfolios and accounts,
and key prefixes cover them without separate indexes. Every lookup and join
therefore names the portfolio; omitting it turns a key seek into a scan.

Portfolio deletion locks its root and removes its entire owned graph in
dependency order, including catalogs, prices, and audit. Idempotency responses
belong to users and remain replayable for 24 hours even after a created
portfolio is deleted. Collection cycles remain with their retained points.
Hourly maintenance removes expired/revoked sessions and expired idempotency
responses in bounded batches.

Reports and holdings contain source facts only. `collected_at` is local report
completion; nullable `source_observed_at` is an actual source timestamp, never a
local substitute. `recorded_at` is the persistence/knowledge cutoff. Failed
reports have neither collection nor source observation time or financial
measures. Display freshness uses source time when known, otherwise collection
time; source clock skew is not rejected by a local-time ordering constraint.
Only platform-supplied margin ratios are stored as reported ratios; the read
model derives missing ratios from retained requirements and adjusted equity.
Points, account valuations, balance valuations, boundary events, manual
adjustments, and sparse calculated asset changes commit atomically with cycle
success. Financial reads share a repeatable-read transaction. The portfolio root
serializes first commit, policy edits, retention, and deletion. A retry reads an
existing point; there is no result update, revision counter, recalculation time,
or rebuild. First commits cannot insert a point before already committed
history; exceptional manual reconciliation belongs to the operator. Boundary
events freeze completeness along with method and amount, referencing the
retained source point and account rather than an expiring valuation row. Points
store no boundary totals; reads sum their boundary events.

`valuation_points.cycle_id` is its primary key and only public identifier.
`valuationRevision` is an opaque consistency token equal to that cycle ID.
Summary, cycle point, and `ReadContext` expose it. Browser parent/child reads
refresh their read family when cycle tokens disagree. `calculation_version`
identifies the algorithm adopted at first commit.

`detail_state` distinguishes full, daily, pruning, and expired detail. API
readers expose pruning as expired before any holdings are removed. A reader's
repeatable-read snapshot either sees all earlier rows or the unavailable state.
Daily point states and the `price_anchors_before` cursor prevent timezone
changes from replacing existing anchors. Detail cleanup, report/price sweeps,
and manual revision cleanup are resumable bounded work while no cycle is
claimable. Price and report sweeps traverse bounded time/ID pages; they preserve
adopted references and last-good source price windows. A finished sweep restarts
only when the cutoff day moves or retention work may have released pins, so a
release caused by collection waits at most one local day. Maintenance pauses for
a minute after a round finds nothing to change or scan. Superseded manual
revisions go once no retained report references them. Referenced futures
contracts and policy predecessors remain for retained endpoint comparisons.
Unreferenced historical catalog entries can expire.

Report sweeps check pins for a page of 64 candidates together. One visit removes
at most 1,000 rows from each holdings table and then deletes empty, unpinned
reports; its cursor stops before any unfinished report. Price sweeps delete
eligible observations in 64-candidate batches. These bounds preserve resumable
cleanup without requiring a worker round per report or per holdings table. An
exhausted manual revision search is also remembered for the current retention
epoch; cutoff changes and released pins make it eligible again.

Long-range comparisons hydrate only points with possible boundary, manual,
mapping, or calculation-version evidence. Calendar coverage and chart sampling
read compact timeline headers; period closes hydrate only their selected
endpoints. These reads preserve the original endpoint and completeness rules
without permanent rollup tables. The timeline index also covers calculation
versions; predecessor and version-window queries name that index because the
measured optimizer otherwise scanned a whole portfolio's clustered records.

Manual audit payloads retain only the edited asset's old and new quantity, so
revision cleanup does not erase the edit. Exclusion audit reads resolve their
referenced immutable policy versions in one query per page, keeping the policy
table as the single source of rules and reasons.

InnoDB reuses freed pages but never shrinks its files;
[deployment](./deployment.md#mysql-capacity) owns disk and binlog settings. The
planning scale is a fixed total of 20 identities, each with about 50 balances
across its accounts, 20 combined futures/earn/dual positions, and its manual
account. Assets are mostly stable holdings, with occasional turnover. Collection
every 30 minutes for ten 365-day years produces 175,200 cycles and points, plus
calendar leap days. Fourteen full days plus daily anchors keep about 4,308
detailed points before boundary/source pins: roughly 4.31 million balances and
balance valuations, 1.72 million combined positions and 0.52 million account
reports and account valuations. With 244 shared market quotes per detailed
cycle, about 1.05 million price observations remain. These are row-count
planning estimates, not physical byte guarantees.

The
[rolling-retention verification](./platform/schema-provenance-delivery.md#rolling-retention-follow-up)
records the updated policy and evidence. Earlier measurements in the same report
used the former 15-minute/30-day policy and do not describe current retention.
[Deployment](./deployment.md#mysql-capacity) owns the whole-machine disk budget.

`portfolios.history_before` is a monotonic cleanup watermark. Public history
queries omit older sparse dependencies; comparison endpoint reads allow an
opening predecessor only when the requested instant is within the supported
window. Retention keeps that snapshot and the last calculated state per asset
before it. Exclusion policy predecessors use the opening cycle's creation time,
not its later evaluation time. Expired children leave before their point;
reports, prices and cycles are swept again after pins release. Current manual
application reads successful cycles, so deleting a historical point does not
turn an already applied revision into a pending edit.

Price cleanup merges two bounded candidate pages from the existing retention
index: non-anchors before the detail cutoff, and daily anchors before the
historical cutoff. The merge sorts at most 128 rows. Cleanup also expires audits
and unused capital flows, then unreferenced policy versions and catalog entries.
A failed transaction invalidates disposable cursors so rolled-back work is not
skipped.

`collection_finished_at` freezes all cycle reports before valuation. A retry
with that marker skips platform reads, account selection, and manual reports; it
retries only the point transaction.

JSON is limited to bounded, typed documents that are read with their owner:

- Earn `details_json` contains `assets`, `rates`, `rewards`, `details`, and
  `issues`. Scalar identity, lifecycle, and product fields stay in columns.
- Audit owns before/after/metadata documents. Idempotency owns typed success
  response JSON and response headers.

`manual_balance_changes` freezes manual markers at first commit, comparing
adopted quantity sets before details can expire. `calculated_asset_changes`
appends only changes of a nonzero calculated asset's treatment: `included` with
its adopted canonical asset, `excluded`, or `absent`. Endpoint comparison uses
the latest change in `(evaluated_at, cycle_id)` order; an included change whose
previous change adopted another canonical asset is a mapping change. Only a
mapping update changes an asset's canonical asset, and it advances the mapping
version, so mapping-change reads consider only remapped assets. Reads derive
calculation-version markers from consecutive points and account issues and named
omissions from retained account valuations, so expired points keep only counts.
Value change consumes these small durable facts and boundary events without
reading historical holdings. Summary-only charts do not hydrate detail. Price
selection batches requested symbols at each source cutoff; adopted-price reads
batch IDs and cache them within the read operation. Source earn diagnostics
remain typed stored data.

Core financial references use relational children. `balance_valuations` owns the
adopted mapping, pricing method, and optional price/value; the `excluded` method
marks an asset excluded by policy. Quantities remain in source balances.
Treatment and missing-price reason are derived. `boundary_events` snapshots
direction, method, and signed value; account labels come from current
identities. Scoped foreign keys enforce ownership, while the point transaction
validates successful source reports, complete balance coverage, price
eligibility, policy interpretation, and values.

There are no raw platform payloads, generic queue payloads, JSON alias readers,
legacy scope versions, or duplicate current manual/exclusion tables. See
[ADR 0015](./adr/0015-prelaunch-schema.md) for the trade-offs.

[Migrations](../db/migrations) own schema; [SQL queries](../db/queries) and
[sqlc.yaml](../sqlc.yaml) generate access. `serve` never migrates or seeds
automatically. Follow
[populated-database upgrade safeguards](./deployment.md#upgrades-and-resets).

## API, numbers, and time

[OpenAPI](../api/openapi/openapi.yaml) owns routes and wire schemas; generated
Go/TypeScript types stop at transport boundaries. Generated files are committed
and must reproduce unchanged from unchanged sources. The API uses Problem
Details, version ETags, idempotency keys, and pagination as specified there.
`shared/etag` strictly parses validators; `shared/cursor` signs opaque cursors.
Reads and mutations remain scoped to the authenticated user and portfolio.

The four idempotent creation routes authenticate and check CSRF before replay. A
user row lock serializes request lookup, business writes, audit, and successful
response insertion in one transaction. Different bodies conflict; failures roll
back the business mutation and key. Resource lookup happens only for a fresh
request, allowing an authorized replay after resource deletion.

Browser writes construct the canonical resource ETag from the JSON version
paired with the edited data or deletion confirmation. Response ETag headers are
not a source of write preconditions: intermediaries can weaken them when
compressing representations. Each resource owns its formatter, including the
separate exclusion version and manual revision number. Backend parsing remains
strict, and conditional SQL updates compare the expected version and increment
it atomically. See [editing behavior](./presentation.md#editing-and-conflicts).

`application/ordering` supplies catalog-based ordering before pagination; the
SPA preserves it. Holdings filters remain mounted independently of results. The
shared URL controller debounces search, defers during IME composition, and saves
outgoing drafts before navigation. Portal menus preserve internal scrolling and
keyboard input while dismissing on external scroll.
[Presentation](./presentation.md#filters) owns matching, filter recovery, and
view-memory rules.

Overview publishes summary and tree together only when cycle ID, valuation
revision, total, and completeness agree. Without a point, both cycle references
must be absent. Account queries include the cycle, report, and revision in their
keys; mismatches refresh the related reads before publishing details. Explicit
mutations invalidate dependent queries; running collection details poll until
completion and then refresh Overview and open details.
[Browser refresh](./presentation.md#browser-refresh-and-scrolling) owns other
display-read behavior.

Financial arithmetic uses exact rationals in `shared/decimal`; storage uses
`DECIMAL(65,30)`, and JSON financial values are strings with at most 30
fractional digits. Null means unknown or inapplicable. The browser uses
`decimal.js`, converting only plot coordinates to JavaScript numbers. Shared
formatting retains source strings for chart tooltips and returns ordinary
rounded decimals for screen readers.
[Numeric presentation](./presentation.md#numeric-presentation) owns rounding and
notation.

Application-generated UUIDv7 IDs are stored in binary form. Database sessions
use UTC; stored instants have microsecond precision and JSON uses RFC 3339.
Portfolio timezones affect queries, not storage.

### Contract position presentation

The financial read model derives size, settlement-currency margin, and issues
from retained position facts and the source report's observed mode. It exposes
reported/calculated margin provenance and unavailable reasons without changing
source amounts. [Domain rules](./domain-rules.md#contract-quantities-and-margin)
own common semantics; platform mappings own source interpretation and formulas.

Historical position reads resolve an optional `cycleId` within the same
portfolio and account, then load its source report. Shared React cells keep
global, account, and historical views consistent. Disclosures render outside
table scroll containers. [Presentation](./presentation.md#futures) owns columns
and interactions. Each position references its immutable observed contract
version.

## Mobile browser scrolling

At the 900px mobile breakpoint, the document owns vertical scrolling and native
overscroll. The sticky header and overlay layers sit above charts; tables scroll
horizontally. The main pane clips horizontal overflow without introducing a
vertical scroll container. Desktop keeps its fixed-height shell and inner pane.

Drawers and dialogs use a reference-counted document lock before moving focus.
The first lock saves scroll position/styles and fixes the body; the last release
restores them. Effect cleanup and breakpoint changes release locks, and focus
restoration cannot scroll the document. Overlay scrollers contain overscroll.
See [browser refresh](./presentation.md#browser-refresh-and-scrolling) for
user-facing behavior and device-coverage limits.

## Authentication and secrets

Users are provisioned through the CLI. Argon2id password hashes belong to users.
Database-backed sessions store hashes of random tokens, have idle and absolute
expiry, and can be revoked. The session cookie is HttpOnly with SameSite=Lax;
its Secure flag follows the application's HTTPS configuration. Authenticated
state changes validate a CSRF header against the session secret; login uses a
pre-authentication cookie/header match. Login throttling is in memory.

Transport middleware applies security headers, request body limits, sensitive
response cache controls, and an Origin check for mutations. The server supports
HTTP, TLS 1.3 HTTPS, and HTTPS with verified client certificates using
configured PEM files. A loaded server certificate/key pair selects HTTPS; a
client CA bundle additionally requires verified client certificates. Transport
mode has no independent enable flags. Certificate automation and hot reload are
not implemented.

Connection credentials are write-only and sealed with AES-256-GCM under a
32-byte master key, bound to their identity and portfolio. The key stays outside
TOML and must be backed up separately. Without it, credential writes,
validation, and enabling are unavailable; readiness fails when encrypted
credentials exist without a key. Runtime, automated-test, and recording
credentials use separate configuration sources. See
[development](./development.md#configuration) and
[testing](./testing.md#test-configuration) for operational setup.

## History windows and tables

The `history-window` read endpoint resolves one UTC window from presets or
portfolio-local inputs. All series, calendar, pagination, and recovery reads
reuse it; ambiguous or nonexistent manual wall times fail explicitly.

Raw point pages read scalar summaries and an indexed count in one transaction.
Aggregated series scan IDs/times, select final actual points by timezone bucket,
then fetch summaries in batches. The scan still grows with the selected range;
sampling does not reduce stored history. Calendar results feed full charts and
paginate independently in the browser.
[Presentation](./presentation.md#history-windows-and-tables) owns windows,
periods, row limits, and URL state.

## Operations and limits

`internal/infra/logging` owns application, access, and SQL sinks. CLI opens logs
before the database and closes them after producers stop. HTTP access logging
surrounds recovery and early rejection for one completion record per request.
MySQL driver wrappers preserve conversion/lifecycle interfaces without changing
services or generated sqlc code; prepared fallbacks avoid duplicate records.
Context propagates request/cycle correlation. See
[logging configuration](./development.md#logging),
[sink operation](./deployment.md#logs), and
[logging trade-offs](./adr/0012-process-logging.md).

Readiness checks database access, schema version, encryption-key availability,
and worker heartbeat. Liveness only indicates that the HTTP process responds.

The current worker updates its heartbeat between work units. A normal cycle
attempt longer than the three-second health threshold can temporarily make
readiness report `worker_unavailable` while work continues. This remains an
unresolved health check limitation; increasing configuration timeouts does not
repair it.

Shutdown cancels the worker context, waits for its loop, and gives HTTP a
bounded shutdown window before closing the database. It does not promise that an
in-flight cycle finishes; an interrupted cycle may remain leased until expiry.
Drain-before-upgrade requirements are documented in
[deployment](./deployment.md).

High availability, distributed workers, a managed database lifecycle, and full
tracing or metrics infrastructure are outside the current architecture.
Implemented capabilities and integration gaps belong in
[README status](../README.md#current-status).
