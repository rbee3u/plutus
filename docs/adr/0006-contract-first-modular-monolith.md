---
status: accepted
---

# ADR 0006: Contract-first modular monolith

## Decision

Deploy one Go process that serves a versioned JSON API, embeds the React SPA,
and runs the scheduler and job worker, with externally managed MySQL. OpenAPI is
the transport contract. Generated HTTP types stay at the boundary; application
services remain independent of Gin and those wire types.

Separate application responsibilities into packages, sharing SQL infrastructure
and transaction-scoped helpers where needed. Normalize Platform data behind
Adapters; do not store raw runtime response payloads. Use exact arithmetic and
string-encoded financial decimals rather than floating-point account values.

Recover Collection Cycles by retaining committed Account Reports, including
failures, and collecting only selected Accounts without a report for that job.
An existing Valuation Point completes collection without another Adapter call or
valuation rewrite. Commit each Identity's reports, missing-report failures,
Margin Mode audit, and collection outcome together after Platform reads finish.
Report uniqueness remains enforced by the database.

## Reason and trade-offs

One binary and one database fit a personal tool and keep deployment simple.
Generated contracts keep browser and server shapes aligned. Typed boundaries
contain exchange-specific parsing. Persistent jobs and leases permit recovery
but do not provide multi-instance coordination or exactly-once collection.
Recovery preserves committed observations and their product scopes, while work
that has not committed uses the current boundary and Connections. It does not
freeze an atomic Platform snapshot or guarantee exactly-once Adapter calls.
Package separation is not exclusive table ownership. Lease-renewal failures
cancel the handler; completion checks that the worker still owns the job.

Static analysis prioritizes correctness, security, and resource lifetimes
through a reviewed linter allowlist. This accepted tooling choice keeps general
style, API-shape preferences, and speculative micro-optimizations out of lint
gates. Standard formatting remains a separate tooling step.

[Architecture](../architecture.md) describes the current runtime mechanisms.
Schema and API details are maintained in migrations and OpenAPI, rather than
parallel handwritten catalogs.
