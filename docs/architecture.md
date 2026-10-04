# Architecture

Plutus is a contract-first Go monolith with an embedded React frontend and an
externally managed MySQL database. This document describes runtime boundaries
and core mechanisms. Domain rules belong in [domain model](../CONTEXT.md);
design reasons belong in [decisions](./adr/README.md).

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

The CLI composes services, transport, and the worker. Application packages
separate responsibilities while sharing generated SQL queries and
transaction-scoped helpers. The HTTP layer maps generated wire types to
application types; services do not depend on Gin or generated HTTP types.
Package boundaries do not imply exclusive ownership of tables or queries.

| Module                                                  | Responsibility                                                         |
| ------------------------------------------------------- | ---------------------------------------------------------------------- |
| `cmd/plutus`                                            | Composition, configuration, and CLI commands                           |
| `internal/transport/http`                               | API bindings, browser security, HTTP conventions, and SPA serving      |
| `internal/auth`                                         | Users, password verification, and Sessions                             |
| `internal/portfolio`, `internal/identity`               | Ownership, Portfolio settings, Identity configuration, and Connections |
| `internal/catalog`, `internal/listingexclusion`         | Listings, Instruments, prices, and exclusion policy                    |
| `internal/collection`                                   | Adapter orchestration and normalized report persistence                |
| `internal/valuation`, `internal/capital`                | Derived values, history, and manual Capital Flows                      |
| `internal/jobs`, `internal/worker`                      | Persistent jobs, scheduling, leases, and dispatch                      |
| `internal/audit`                                        | User and system action records                                         |
| `internal/adapters`                                     | Typed Platform contract, live implementations, and deterministic fake  |
| `internal/infra`, `internal/decimal`, `internal/secret` | SQL infrastructure, exact arithmetic, and credential encryption        |

## Collection and job lifecycle

Scheduled and manual collection use the same persistent job and Portfolio
deduplication rule. The worker polls an idle queue every second, claims jobs
transactionally, renews its two-minute leases, and can reclaim expired jobs.
Handler failures have bounded retries with backoff. Partially persisted
collection is not an exactly-once operation; reclaiming a lease does not promise
seamless completion of every interrupted cycle.

A cycle establishes its selected Accounts, then collects each usable Connection
with a fixed three-second delay between Identities. Adapters perform the
applicable structure and Margin Mode checks. Successful normalized reports,
holdings, and prices are persisted; failed or omitted Accounts receive failure
reports. Valuation then selects compatible source reports, calculates Account
values and Boundary Events, and writes one Valuation Point for the cycle.

Temporary and rate-limit Adapter failures have bounded retries within the cycle.
Authentication and authorization failures invalidate the Connection. Platform
failures can still produce a point with stale or missing values. Database or
other infrastructure failures can interrupt collection before a point exists.
The full stale, scope, completeness, and value rules are in
[domain model](../CONTEXT.md#collection-and-account-reports).

## Adapter boundary

Adapters accept typed credentials, Account Structure, and selected Account
Types, and return normalized reports and market observations. They expose
supported structures and Account Types and report observed Margin Modes. The
application does not retain or depend on raw Platform response shapes.

Runtime calls use only read-only account, balance, position, and market-data
interfaces. Normalized error details may name an endpoint and Platform error
code; raw bodies and credentials must not enter logs or storage. Live HTTP and
Kraken WebSocket connections use IPv4 for direct access and proxy first hops; an
external proxy controls the address family it presents to the Platform.

Validated instance configuration selects environment proxies, forced direct
access, or one forced HTTP/HTTPS/SOCKS proxy for all runtime Adapters. Explicit
proxy mode ignores environment bypass rules and never falls back to direct. The
shared transport clones the default transport without global mutation; Adapter
factories preserve individual timeouts and caller-supplied HTTP clients retain
precedence. Kraken WebSocket uses the same configured HTTP client. Proxy
availability is not a readiness dependency. External software such as sing-box
owns SS protocols, credentials, and tunnel lifecycle. Explicit Adapter recording
retains its separate process-environment proxy policy. See
[proxy configuration](./development.md#platform-proxies).

Development registers the deterministic fake for offered Platforms. Remote test
and prod register live Adapters. Automated tests use independent synthetic
responses and a network guard. Only explicit `adapter record` can publish
sanitized responses to `/tmp/plutus/` for review. See the
[evidence workflow](./adapters/evidence-workflow.md) and
[Platform mapping guides](../README.md#documentation).

## Persistence and rebuilds

MySQL holds configuration, normalized observed facts, derived values, jobs, and
audit records. Observed holdings have no user editing API; new cycles produce
new reports. Derived valuation fields on reports and Valuation Point results can
be rebuilt in place. Boundary Events are calculated and stored with points, then
reused by reads.

Mapping corrections enqueue affected Portfolio rebuilds. Rebuilds retain point
evaluation times, use stored report scopes and cycle-time exclusion policy, and
recalculate succeeded cycles in order. Source facts, historical policy events,
and point results remain distinguishable.

Financial observations and Valuation Points have no implemented retention or
downsampling policy. Completed jobs without report or point ownership are pruned
according to configuration. Configuration versions support optimistic
concurrency.

Report scope is stored with both successful and failed Account Reports. Legacy
combined Earn uses scope 1; separate Earn and Dual on Binance, OKX, and Bybit
use scope 2. Stale selection requires equal scope. Rebuilds preserve the
reclassification and incomplete-comparison rules in the
[domain model](../CONTEXT.md#report-scope-compatibility).

The cumulative schema is defined by [migrations](../db/migrations); generated
query access comes from [SQL queries](../db/queries) and
[sqlc.yaml](../sqlc.yaml). There is no separate handwritten field catalog. The
baseline migration's comment still refers to the retired `technical-design.md`
field catalog; the migration files are the current schema source. The baseline
is preserved during documentation edits.

Migrations are explicit CLI operations; `serve` never migrates or seeds
automatically. After prod's first migration, the baseline is frozen and changes
must work on populated databases. Upgrade procedures are in
[deployment](./deployment.md).

## API, numbers, and time

[OpenAPI](../api/openapi/openapi.yaml) is the API contract. Generated Go server
bindings and TypeScript client types stop at the transport boundary. Generated
files are committed; regeneration must leave them unchanged when their sources
have not changed. Routes and payload schemas are not duplicated here.

The API uses versioned routes, Problem Details errors, version-based ETags,
idempotency keys for supported mutations, and cursor pagination where specified
by OpenAPI. Browser ownership checks scope access to the authenticated User and
their Portfolios.

Display ordering is shared by application reads through `internal/ordering`.
Platform ranks and Account Type ranks come from the Adapter capability catalog.
Valuation reads sort Accounts first, then order flat holdings by those Account
ranks before grouping or pagination; Account-detail futures use the same
contract comparator. Catalog queries receive the shared Platform order and apply
it in SQL before `LIMIT` and `OFFSET`. The SPA preserves API ordering in lists
and selectors. See [ADR 0011](./adr/0011-display-ordering.md).

The global Holdings tables keep their filter headers mounted independently of
query results. Platform and Identity candidates derive from configured
non-archived Identities; successful reads normalize obsolete URL choices.
Interactive header content supplements a plain column name in `DataColumn`.
Portal popovers measure their available viewport space and dismiss on external
scroll, while preserving internal list scrolling and keyboard input. A shared
URL controller debounces search by 250 ms, defers during IME composition, and
flushes pending input on dismissal. Before internal navigation, the shared Shell
controller saves pending input in the outgoing Holdings view without updating
the destination URL. Server reads apply literal case-insensitive substring
matching before pagination. Flat Account filters and Balance grouping are
retired. Balances, Futures, and Dual Investment use separate top-level routes
and session-memory state keyed by User, Portfolio, and view. Internal links
restore the target view's filters, search, and page; explicit URLs, reloads, and
browser history remain authoritative. See
[ADR 0014](./adr/0014-holdings-header-filters.md).

Overview reads its summary and ownership Tree in one frontend query. The result
is published only when the Valuation Point, total, and completeness agree; query
retries and recovery retain the last successful pair. With no Valuation Point,
only matching absent point IDs are required. Expanded Account queries are keyed
by point and report, and validate both against the visible Account. A newer
latest-only response produces a recovery notice instead of mixing reports.

The ownership Tree uses nested lists and native disclosure buttons rather than a
table or an ARIA tree widget. Buttons span each node row and expose expanded
state. Names and USD values reflow inside the row; child containers add
indentation and a vertical guide. Account detail grids use shrinkable columns
and local table wrappers, so their width never expands the ancestor Tree. Report
measures render only when present. Provenance lives in a native Report details
disclosure; exception feedback remains inside Account details, with page-level
Issues independent of expansion.

Overview and the Activity Cycles/Jobs lists have no periodic display polling.
Page navigation and browser reload read existing data. Explicit mutations still
invalidate dependent queries, and running Collection details retain status
polling until completion. Collect now updates the paired Overview read and open
Account details on completion. Browser refresh does not request collection. See
[ADR 0016](./adr/0016-overview-and-holdings-navigation.md).

Financial arithmetic uses exact rationals behind `internal/decimal`; storage
uses `DECIMAL(65,30)`. Storage and JSON boundaries format decimal values with at
most 30 fractional digits. JSON financial values are strings; null means unknown
or inapplicable. The frontend uses `decimal.js`, converting to JavaScript
numbers only for chart plotting.

The shared numeric formatter selects a semantic type and explicit half-up
rounding, then returns display text and its rounded ordinary decimal equivalent.
Quantity and price wrappers share precision; USD, ratio, percentage, leverage,
and count wrappers select their own policy. Percentage scaling uses a separate
Decimal constructor with enough precision for stored values, without changing
global arithmetic. Unicode subscript digits encode long fractional zero runs.
`NumericText` exposes the rounded ordinary decimal to screen readers without
adding a control; numeric table strings use it automatically. Pure numeric
source-value overlays are removed, while business detail overlays remain.

Chart series keep their source decimal strings in index-based tooltip closures.
Plotted JavaScript numbers are not used for tooltip rounding. Axis formatting
uses compact or scientific notation independently of tooltip precision. Inputs,
API requests, diagnostic JSON, calculations, and ordering retain their existing
precision. [ADR 0013](./adr/0013-numeric-presentation.md) owns the display
policy.

Application-generated UUIDv7 identifiers are stored in binary form. Database
sessions use UTC; stored instants use microsecond precision and JSON uses
RFC 3339. Portfolio timezones affect calendar queries, not stored instants.

### Contract position presentation

Account Reports retain the observed account Margin Mode; stale reads use the
successful source Report rather than the Identity's latest mode. Positions
retain direction, source quantity, position mode, margin setting, source margin
currency, and initial/allocated basis. Exact decimal read calculations add Size,
settlement-currency Margin, Portfolio context, and row issues to the API.
[ADR 0012](./adr/0012-position-presentation.md) owns the display policy.

Margin reads prefer a verified source amount and expose `marginSource` as
`reported` or `calculated`, plus `marginUnavailableReason` for unknown values.
Binance Cross initial requirements can be calculated from the source Position's
Size, Mark, and initial leverage, including supported retail PM. The immutable
Binance PM structure establishes Cross for old Reports without margin metadata;
no source amount or observation time changes. Other Platforms use their own
reported measures rather than this fallback.

OKX Cross reads retain the reported margin currency and verify non-PM inverse
`imr` against both native and USD requirements using exact rational arithmetic
and a relative `1e-8` tolerance. This also corrects earlier typed-USD reads when
only the native candidate matches. Missing provenance, unsupported currencies,
and ambiguous or mismatched candidates remain unknown. The source Report's
regime determines eligibility; current Identity settings never establish it.

The account Positions endpoint accepts an optional `pointId` scoped to the same
Portfolio and Account. Historical expansion reads that point's source Report,
never the latest report. Shared React cells provide the same columns and units
in global, Account, and historical tables. Detail overlays render outside table
scroll containers and support hover, focus, click, and Escape.

Margin uses the numeric cell's right alignment, with no mode tag or reserved
track. Its cell receives a content-sizing width hint so automatic table layout
does not assign it spare space. The amount disclosure is one focusable control;
mode and leverage share a short line in this overlay. Cross uses source
leverage; Isolated divides absolute USD notional by allocated margin valued in
USD. The shared display helper uses source decimals before formatting, with 1:1
USD-equivalent conversion or compatible Base Settle times positive Mark.
Incomplete inputs show the known mode alone. This derived display never changes
source leverage or margin. Portfolio accounts show `portfolio margin` unless the
Position has an actual Isolated setting. Symbol overlays contain contract
specifications without leverage; margin diagnostics remain in the API.

The appended Position presentation migration restores only byte-exact canonical
references with compatible stored direction. Legacy Bybit margin is marked as an
initial requirement; missing Report modes and other absent margin provenance
remain null. The baseline and source financial amounts are retained.

## Mobile browser scrolling

At the existing 900px layout breakpoint, the document owns vertical scrolling.
The root, shell, workspace, and main content grow with their contents rather
than clipping a viewport-sized inner pane. Vertical overscroll uses the browser
default, allowing native pull-to-refresh in ordinary mobile browser tabs. The
mobile top bar is sticky below navigation overlays and portal menus; table
wrappers retain horizontal scrolling. Sign-in and startup screens use viewport
minimum heights. Desktop retains its fixed-height shell and scrolling main pane.
The mobile main pane clips only horizontal overflow, containing chart tooltips
left at an old position after a viewport resize without adding a vertical scroll
container. Its stacking context keeps chart content below the sticky header and
navigation overlays, while body-portal menus retain their own overlay layer.

Navigation drawers and shared Dialogs acquire a reference-counted mobile
document lock before moving focus. The first lock records the scroll offset and
affected inline styles, then fixes the body at the saved offset and locks root
scrolling. The last release restores those styles and the scroll offset. Effect
cleanup and layout-breakpoint changes release locks; focus restoration does not
scroll the document. Drawer and dialog scrollers contain overscroll, while their
backgrounds remain fixed. Shared menus dismiss on external scroll and preserve
internal menu scrolling.

Reload keeps the current URL, rereads existing data, and resets transient React
state. It does not enqueue a Collection Cycle. The accepted user-facing behavior
and browser coverage boundary are recorded in
[ADR 0015](./adr/0015-mobile-native-refresh.md).

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

History shares a resolved UTC window across series, raw point pages, and
calendar reads. The read-only `history-window` endpoint resolves shortcuts or
Portfolio-local wall times; invalid, ambiguous, and nonexistent manual times
fail explicitly. Browser URLs retain relative presets or absolute custom
endpoints. Pagination and recovery reuse the applied window.

The `valuation-points/page` endpoint reads scalar summaries in descending order
with SQL LIMIT/OFFSET and an indexed range count in one transaction. It reuses
OffsetPage metadata. The existing ascending series interface remains compatible.
Raw series read scalar summaries; Hour/Day scan only IDs and evaluation times,
apply the existing timezone sampler, and fetch selected summaries in batches.
Series reads avoid detail JSON, but the header scan still grows with the
selected range. No migration, preaggregation, or retention policy is introduced.

Period closes and Value Changes retain complete intersecting calendar periods
and the 400-period ceiling. Their full results feed charts while table display
reverses and paginates independently. Separate URL pagination keys prevent point
expansion or an unrelated period selector from resetting a table. See
[ADR 0017](./adr/0017-history-windows-and-pagination.md).

## Operations and limits

The framework-independent `internal/infra/logging` component owns three `slog`
loggers and independent rolling text files for application, HTTP access, and SQL
events. The CLI loads configuration and initializes sinks before opening the
database; it stops producers and closes the database before closing logs.
Application logs also go to stderr. Gin framework writers and server diagnostics
are adapted at the HTTP boundary. HTTP recovery records exceptions and stacks,
and access logging surrounds recovery and early rejection so each request has
one completion record.

MySQL Connector, connection, statement, and transaction wrappers capture SQL
execution without changing service constructors or generated sqlc code. They
preserve driver conversion and connection lifecycle interfaces. Prepared
fallbacks do not emit duplicate records. Parameter substitution is diagnostic;
its uncertainty falls back to the original statement and arguments. Context
fields associate SQL and application events with requests and claimed jobs. SQL
duration ends at driver return, excluding row iteration. See
[ADR 0010](./adr/0010-process-logging.md) for accepted visibility and retention
trade-offs and [configuration](./development.md#logging) for settings.

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

The [pricing decision](./adr/0003-valuation-and-catalog.md) requires
same-Platform prices. Account valuation currently prefers that Platform but
falls back to the latest eligible price across Platforms. Capital Flow valuation
queries prices across Platforms directly. Both enforce at-or-before time and the
24-hour age limit. See [Account price lookup](../internal/valuation/write.go)
and [Capital Flow valuation](../internal/capital/service.go).

This is an unresolved implementation gap, not an accepted change to the pricing
decision.
