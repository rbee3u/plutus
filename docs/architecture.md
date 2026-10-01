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
Kraken WebSocket connections use IPv4; an external proxy controls the address
family it presents to the Platform.

Development registers the deterministic fake for offered Platforms. Remote test
and prod register live Adapters. Automated tests use independent synthetic
responses and a network guard. Only explicit `adapter record` can publish
sanitized responses to gitignored `tmp/` for review. See the
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

Financial arithmetic uses exact rationals behind `internal/decimal`; storage
uses `DECIMAL(65,30)`. Storage and JSON boundaries format decimal values with at
most 30 fractional digits. JSON financial values are strings; null means unknown
or inapplicable. The frontend uses `decimal.js`, converting to JavaScript
numbers only for charts.

Application-generated UUIDv7 identifiers are stored in binary form. Database
sessions use UTC; stored instants use microsecond precision and JSON uses
RFC 3339. Portfolio timezones affect calendar queries, not stored instants.

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

## Operations and limits

Structured logs carry request and job identifiers and operational outcomes.
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
