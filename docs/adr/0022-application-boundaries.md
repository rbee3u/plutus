---
status: accepted
---

# ADR 0022: Application assembly and financial read boundaries

## Decision

SQL-backed services live in `internal/application`; there is no parallel domain
or repository layer. App supplies the same collaborators to all consumers, and
HTTP neither constructs nor mutates them. This refines
[ADR 0020](./0020-architecture-layering.md) and
[ADR 0021](./0021-frontend-structure.md).

Financial owns presentation reads; Valuation owns calculation, writes, rebuilds,
and narrow stored Point/Account reads. They share stored decoding and scoped
pricing, while exact writer arithmetic stays private. Query and rebuild loaders
feed the same evaluator without SQL, clocks, or query callbacks.

One ordered Platform catalog defines offered products and finite browser
credential metadata. Runtime registration determines availability; server
validation and recording assembly remain independent.

Catalog owns mapping and rebuild enqueueing. Mapping commits first, so an
enqueue failure does not undo it. Portfolio deletion uses explicit ordered sqlc
queries with foreign keys enabled; source-reference release and deletion share
one transaction. Foreign ownership causes rollback; shared catalog and
User-scoped data survive.

QueryClient owns the authenticated User and Portfolio list. Shared browser
mechanics handle authentication, CSRF, cache keys, and invalidation; features
retain raw SDK results and their conflict, validation, and idempotency policies.
A password business rejection does not end the session. Session lists load on
entry and revocation without background refresh. Shared navigation owns
mechanics, Holdings owns route memory, and charts explicitly import shared
initialization.

## Reason and trade-offs

Explicit injection prevents inconsistent service state without a DI framework.
Separating Financial lets presentation evolve without broad access to valuation
internals, at the cost of narrow shared read interfaces. Prepared inputs isolate
financial rules from SQL and rebuild batching without adding a valuation
framework; lazy reads and per-Point transactions remain intact.

A finite catalog removes repeated Platform branching without a general form
engine. Browser and server validation retain their existing differences.
Application-owned mapping removes HTTP orchestration but deliberately leaves
atomic mapping/enqueueing for a separate behavior decision.

Explicit deletion makes table ownership reviewable and keeps foreign-key
protection. New owned tables require updating the deletion order; unexpected
cross-Portfolio references fail instead of deleting foreign data.

Shared browser mechanics remove repeated infrastructure without centralizing
product decisions or editor state. Limited feature exports keep business logic
out of shared infrastructure.

[Architecture](../architecture.md) owns runtime mechanisms and remaining limits.
