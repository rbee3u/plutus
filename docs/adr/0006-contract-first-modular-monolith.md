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

Recovery retains committed reports, including failures, and reuses an existing
Valuation Point. Each Identity's reports, mode audit, and outcome commit
together after Platform reads. See the
[collection lifecycle](../architecture.md#collection-and-job-lifecycle).

## Reason and trade-offs

One binary and one database fit a personal tool and keep deployment simple.
Generated contracts keep browser and server shapes aligned. Typed boundaries
contain exchange-specific parsing. Persistent jobs and leases permit recovery
but do not provide multi-instance coordination or exactly-once collection.
Uncommitted work uses the current boundary and Connections, so recovery does not
freeze a Platform snapshot. Package separation is not exclusive table ownership.

The [lint gates](../testing.md#validation-gates) prioritize correctness,
security, and resource lifetimes; formatting is separate. Style and speculative
micro-optimizations do not justify extra lint rules.

[Architecture](../architecture.md) describes the current runtime mechanisms.
Schema and API details are maintained in migrations and OpenAPI, rather than
parallel handwritten catalogs.
