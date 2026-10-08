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
  +-- scheduler and job worker ----------------> MySQL
  +-- typed Platform Adapters -----------------> Platforms or fake data
```

One application process serves the API and SPA, schedules collection, and runs
jobs. Only one process may be active for a database. Job leases support restart
recovery; they do not provide multi-instance coordination or exactly-once
collection. Plutus does not manage MySQL installation, backups, or upgrades.

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
application code uses the normalized Adapter contract, Valuation cannot import
Financial, and HTTP cannot import SQL APIs. Runtime code cannot import
`internal/testsupport`. Within-layer dependencies remain reviewed choices.

| Module                                                                  | Responsibility                                                         |
| ----------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| `internal/app`                                                          | Composition root: every service built on one database handle           |
| `internal/cli`, `cmd/plutus`                                            | CLI commands; `cmd/plutus` only starts them                            |
| `internal/seeds`                                                        | Development seed fixtures for `db seed` and `db reset --seed`          |
| `internal/transport/http`                                               | API bindings, browser security, HTTP conventions, and SPA serving      |
| `internal/application/auth`                                             | Users, password verification, and Sessions                             |
| `internal/application/portfolio`, `internal/application/identity`       | Ownership, Portfolio settings, Identity configuration, and Connections |
| `internal/application/catalog`, `internal/application/listingexclusion` | Listings, Instruments, prices, and exclusion policy                    |
| `internal/application/collection`                                       | Adapter orchestration and normalized report persistence                |
| `internal/application/valuation`, `internal/application/capital`        | Derived value writes/rebuilds and manual Capital Flows                 |
| `internal/application/financial`                                        | Overview, Holdings, history, periods, and financial presentation       |
| `internal/application/jobs`, `internal/worker`                          | Persistent jobs, scheduling, leases, and dispatch                      |
| `internal/application/audit`                                            | User and system action records                                         |
| `internal/platform`                                                     | Typed Platform contract, live implementations, and deterministic fake  |
| `internal/infra`, `internal/shared/decimal`, `internal/infra/secret`    | SQL infrastructure, exact arithmetic, and credential encryption        |

Financial reads stored Points and Account Valuations through Valuation, sharing
decoding, Account issues, and an operation-scoped PriceBook. Readers aggregate
stored decimals; writers retain unrounded rationals privately. Internal source
metadata is excluded from JSON. Product filters, totals, and completeness stay
separate. `application/ordering` uses the offered Platform catalog; `shared` has
no business dependencies.

Collection and rebuild loaders feed one valuation evaluator with prepared
inputs, without SQL, clocks, or query callbacks. App injects shared Catalog,
Manual, and Adapter registry instances into their consumers.

Catalog's mapping operation includes exclusion presentation and rebuild
enqueueing through Jobs. Mapping commits before enqueueing; an enqueue failure
can leave a committed mapping without its rebuild job.

Portfolio deletion uses explicit sqlc queries in dependency order with foreign
keys enabled. Account Report self/stale references are released only within the
same transaction that deletes those reports; all changes roll back on failure.
New owned tables require an explicit deletion decision. Shared catalogs and
User-scoped rows remain outside the deletion scope; unexpected foreign ownership
causes rollback.

See [test ownership](./testing.md#test-ownership) for cross-module workflows.

## Frontend structure

The [frontend boundaries](./adr/0013-frontend-architecture.md) separate `app`,
`features`, `shared`, and `api`, enforced by ESLint. Features expose explicit
entrypoints; API consumers use `api/index.ts`. Feature CSS stays with its owner;
compound selectors and cascade-sensitive rules stay within one CSS module.

QueryClient owns the authenticated User and User-scoped Portfolio list. The auth
boundary waits before mounting features; an auth-me 401 means no User, while
transport failures remain retryable. Login seeds the cache. Logout, expiry, and
password success cancel queries, clear cached data and CSRF state, and return to
login. Generation-bound expiry callbacks prevent a late 401 from ending a
replacement session, including a new login by the same User.

Shared writes acquire CSRF and handle expiry, except a password
`invalid_credentials` rejection. Features retain raw SDK results and their
validation, conflict, ETag, and idempotency policies. Cache keys and Collection
invalidation flags are shared; editors retain independent lifetimes. Session
lists use a User-scoped cache discarded when unused or logged out, with reads on
entry and revocation but no background retry, focus, or reconnect refresh.

Holdings shares request conversion, recovery, pagination, and Account Report
reads; pages own product semantics. Shared navigation prepares outgoing drafts,
while Holdings owns view-memory policy. Charts explicitly import common ECharts
registration. See [presentation](./presentation.md) for interaction rules and
[test ownership](./testing.md#test-ownership) for browser fixtures.

## Collection and job lifecycle

Scheduled and manual collection use the same persistent job and Portfolio
deduplication rule. The worker polls an idle queue every second, claims jobs
transactionally, renews its two-minute leases, and can reclaim expired jobs.
Infrastructure failures have bounded retries with backoff. Recovery retains
every committed Account Report for the job, including failures, and collects
only selected Accounts still missing reports. An existing Valuation Point is
reused without collection or rewriting its evaluation time and values; the
worker then completes the job status. Report uniqueness remains enforced.
Duplicate persistence errors fail the job without repeating the same conflict.
Lease-renewal failures cancel the handler, and completion checks lease
ownership. This remains a single-process design without exactly-once Platform
reads.

A cycle establishes its selected Accounts, then collects each usable Connection
with a fixed three-second delay between Identities. Adapters perform the
applicable structure and Margin Mode checks. Successful normalized reports,
holdings, and prices are persisted; failed or omitted Accounts receive failure
reports. Each Identity commits its reports, missing-report failures, Margin Mode
audit, and collection outcome in one transaction after its Platform reads.
Recovery keeps committed observations and scopes; uncommitted work uses the
current boundary and Connections. Valuation then selects compatible source
reports, calculates Account values and Boundary Events, and writes one Valuation
Point for the cycle.

Temporary and rate-limit Adapter failures have bounded retries within the cycle.
`unsupported` failures, including detected structure mismatches, are not retried
within that run. Enabled Connections remain scheduled for later cycles.
Authentication and authorization failures invalidate the Connection. Platform
failures can still produce a point with stale or missing values. Database or
other infrastructure failures can interrupt collection before a point exists.
The full stale, scope, completeness, and value rules are in
[domain rules](./domain-rules.md#collection-and-account-reports).

## Manual persistence and collection

`internal/application/manual` owns the signed ledger, immutable revisions, and
version-guarded audited mutations. Before Platform reads, the runner locks the
job and Manual boundary Accounts and atomically freezes inputs, including empty
ledgers, in `manual_cycles` and `manual_cycle_accounts`. Retries reload them;
ledger edits serialize on Account version. Reports and metadata commit together,
and job cleanup retains jobs referenced by frozen inputs.

The separate `MarketData` capability fetches public prices without credentials
or private preflight, grouped by Platform and required Assets. Failure leaves
quantities and eligible cached prices available. Manual reports bypass Adapter
Collect and private Identity success counters. Live valuation, reads, and
rebuilds all use the own-Platform lookup.

Writer and rebuild share quantity-signature adjustment detection; signatures
exclude timestamps and mapping. The ledger API exposes current/last-applied
quantities, versions, and pending state.
[Manual rules](./domain-rules.md#manual-balances) own continuity and
empty-Account semantics; [deployment](./deployment.md#upgrades-and-resets) owns
existing-data upgrades.

## Adapter boundary

The ordered `platform/adapters` catalog owns offered Platforms, Account
Structures, Account Types, Margin Modes, and finite browser credential metadata.
Lookups return copies of mutable descriptions. Runtime registration determines
availability separately; CLI keeps live/fake assembly explicit, and recording
has its own assembly. Browser forms do not replace server validation or expose
stored credentials.

Adapters accept typed credentials, declared structure, and selected Account
Types, returning normalized reports and market observations. Application code
does not depend on raw response shapes. Runtime calls use only read-only
account, balance, position, and market-data interfaces. Errors may name an
endpoint and Platform error code; raw bodies and credentials never enter logs or
storage.

The shared transport clones the default without global mutation, clears its
proxy callback, and applies the validated explicit instance policy. Adapter
request budgets and caller-supplied clients retain precedence. Kraken WebSocket
uses the same HTTP client; direct connections and proxy first hops use IPv4.
Proxy failure never falls back to direct, and proxy availability is not a
readiness dependency. External software owns tunnels. Recording reuses proxy
validation and the client factory but loads no runtime secrets, logging, or TLS
settings. [Development](./development.md#platform-proxies) owns configuration.

Dev registers the fake; remote test/prod register live Adapters. Automated tests
use independent synthetic responses and a network guard. Only explicit recording
writes sanitized responses under `/tmp/plutus/`; see the
[evidence workflow](./platform/evidence-workflow.md) and
[Platform mappings](../README.md#documentation).

## Development scenarios

Seed history and ongoing fake collection share a pure scenario generator.
Reserved `plutus-demo-v1` credentials carry a scenario ID and UTC anchor;
validation binds a stable external reference. Platform/Asset/time prices are
independent of login identity to avoid shared-price collisions.

Seeds insert source facts in one transaction, initialize Connections through
real services, then use the normal rebuild to derive values. Historical Plutus
events stop at the anchor; continued fake holdings cannot mutate flows,
selection, archival, or Catalog policy. No persistent simulator or background
application hook is needed. [Seeds](./seeds.md) owns scenarios, coverage, and
performance evidence.

## Persistence and rebuilds

MySQL stores configuration, normalized source facts, derived values, jobs, and
audits. Observations have no editing API. Rebuilds recalculate succeeded cycles
in order using retained report scopes, source times, and cycle-time exclusion
policy; evaluation times remain fixed. Financial history has no downsampling or
retention limit; job cleanup excludes referenced jobs.

Collection and rebuild share valuation and atomic Point/report writes. Rebuild
preloads bounded chunks, including compatible stale sources outside each chunk;
its price lookup preserves ordinary SQL selection and ambiguity behavior.
Sequential predecessor values use stored decimal precision. Each Point commits
independently: failure retains earlier commits and retries can recompute them
safely. Batch sizes and loading details live in
[the rebuild implementation](../internal/application/valuation/rebuild.go).

Reports preserve scope on success and failure: combined Earn uses scope 1;
separate Earn/Dual on Binance, OKX, and Bybit use scope 2. Stale selection
requires equality; [domain rules](./domain-rules.md#report-scope-compatibility)
own reclassification semantics.

`earn_positions` stores normalized typed JSON, never raw Platform responses.
`earn_details_collected` distinguishes missing descriptions from a successful
empty read. Reads follow the valuation's source report, including stale history,
and return missing-detail Balance summaries separately. Platform-specific
metadata reads belong in [mappings](../README.md#documentation).

[Migrations](../db/migrations) own schema; [SQL queries](../db/queries) and
[sqlc.yaml](../sqlc.yaml) generate access. The frozen baseline's
`technical-design.md` reference is obsolete; use the migration itself. `serve`
never migrates or seeds automatically. Follow
[populated-database upgrade safeguards](./deployment.md#upgrades-and-resets).

## API, numbers, and time

[OpenAPI](../api/openapi/openapi.yaml) owns routes and wire schemas; generated
Go/TypeScript types stop at transport boundaries. Generated files are committed
and must reproduce unchanged from unchanged sources. The API uses Problem
Details, version ETags, idempotency keys, and pagination as specified there.
`shared/etag` strictly parses validators; `shared/cursor` signs opaque cursors.
Reads and mutations remain scoped to the authenticated User and Portfolio.

`application/ordering` supplies catalog-based ordering before pagination; the
SPA preserves it. Holdings filters remain mounted independently of results. The
shared URL controller debounces search, defers during IME composition, and saves
outgoing drafts before navigation. Portal menus preserve internal scrolling and
keyboard input while dismissing on external scroll.
[Presentation](./presentation.md#filters) owns matching, filter recovery, and
view-memory rules.

Overview publishes summary and Tree together only when Point ID, total, and
completeness agree. Without a Point, both IDs must be absent. Account queries
are keyed by Point and Report and validate both against the visible Account;
latest-only races offer recovery rather than mixing reports. Explicit mutations
invalidate dependent queries; running Collection details poll until completion
and then refresh Overview and open details.
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

The financial read model derives Size, settlement-currency Margin, and issues
from retained Position facts and the source Report's observed mode. It exposes
reported/calculated margin provenance and unavailable reasons without changing
source amounts. [Domain rules](./domain-rules.md#contract-quantities-and-margin)
own common semantics; Platform mappings own source interpretation and formulas.

Historical Position reads resolve an optional `pointId` within the same
Portfolio and Account, then load its source Report. Shared React cells keep
global, Account, and historical views consistent. Disclosures render outside
table scroll containers. [Presentation](./presentation.md#futures) owns columns
and interactions; [deployment](./deployment.md#position-presentation-upgrade)
owns historical provenance restoration.

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

Users are provisioned through the CLI. Argon2id password hashes belong to Users.
Database-backed Sessions store hashes of random tokens, have idle and absolute
expiry, and can be revoked. The session cookie is HttpOnly with SameSite=Lax;
its Secure flag follows the application's HTTPS configuration. Authenticated
state changes validate a CSRF header against the Session secret; login uses a
pre-authentication cookie/header match. Login throttling is in memory.

Transport middleware applies security headers, request body limits, sensitive
response cache controls, and an Origin check for mutations. The server supports
HTTP, TLS 1.3 HTTPS, and HTTPS with verified client certificates using
configured PEM files. A loaded server certificate/key pair selects HTTPS; a
client CA bundle additionally requires verified client certificates. Transport
mode has no independent enable flags. Certificate automation and hot reload are
not implemented.

Connection credentials are write-only and sealed with AES-256-GCM under a
32-byte master key, bound to their Identity and Portfolio. The key stays outside
TOML and must be backed up separately. Without it, credential writes,
validation, and enabling are unavailable; readiness fails when encrypted
credentials exist without a key. Runtime, automated-test, and recording
credentials use separate configuration sources. See
[development](./development.md#configuration) and
[testing](./testing.md#test-configuration) for operational setup.

## History windows and tables

The `history-window` read endpoint resolves one UTC window from presets or
Portfolio-local inputs. All series, calendar, pagination, and recovery reads
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
Context propagates request/job correlation. See
[logging configuration](./development.md#logging),
[sink operation](./deployment.md#logs), and
[logging trade-offs](./adr/0012-process-logging.md).

Readiness checks database access, schema version, encryption-key availability,
and worker heartbeat. Liveness only indicates that the HTTP process responds.

The current worker updates its heartbeat between jobs. A normal job longer than
the three-second health threshold can temporarily make readiness report
`worker_unavailable` while work continues. This remains an unresolved health
check limitation; increasing configuration timeouts does not repair it.

Shutdown cancels the worker context, waits for its loop, and gives HTTP a
bounded shutdown window before closing the database. It does not promise that an
in-flight cycle finishes; an interrupted job may remain leased until expiry.
Drain-before-upgrade requirements are documented in
[deployment](./deployment.md).

High availability, distributed workers, a managed database lifecycle, and full
tracing or metrics infrastructure are outside the current architecture.
Implemented capabilities and integration gaps belong in
[README status](../README.md#current-status).

## Known implementation discrepancy

The [pricing decision](./adr/0003-valuation-and-asset-policy.md) requires
same-Platform prices. Exchange Account valuation currently prefers that Platform
but falls back to the latest eligible price across Platforms. Manual Account
valuation enforces own-Platform prices without fallback. Capital Flow valuation
queries prices across Platforms directly. Both enforce at-or-before time and the
24-hour age limit. See
[Account price lookup](../internal/application/valuation/prices.go) and
[Capital Flow valuation](../internal/application/capital/service.go).

This is an unresolved implementation gap, not an accepted change to the pricing
decision.
