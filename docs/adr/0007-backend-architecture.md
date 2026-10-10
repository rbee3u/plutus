---
status: accepted
---

# Contract-first monolith and explicit backend boundaries

Use one Go process for the versioned JSON API, embedded React SPA, scheduler,
and worker, with externally managed MySQL. OpenAPI owns transport contracts;
generated HTTP types stay at the boundary. Application services use SQL and sqlc
directly, without a parallel repository layer or DI framework.

Group responsibilities into shared mechanisms, infrastructure, Platform,
application, and transport. A composition root supplies shared service
instances; CLI owns process lifetime and HTTP neither constructs nor mutates
collaborators. Normalized Adapter contracts isolate Platform parsing; raw
runtime payloads do not reach storage or logs. Exact arithmetic and
string-encoded financial values preserve financial meaning.

Financial owns presentation reads; Valuation owns calculation, writes, and
narrow stored Point/Account reads. Shared decoding and scoped pricing do not
expose writer arithmetic. Collection loaders feed the same evaluator with
prepared inputs, without SQL, clocks, or query callbacks. Catalog owns atomic
mapping and audit writes. Portfolio deletion uses a durable root marker and
bounded ordered transactions with foreign keys enabled; foreign ownership causes
rollback.

One ordered Platform catalog defines capabilities and finite credential
metadata; runtime registration determines availability. Coarse lint boundaries
enforce layer direction. Shared transactions, errors, cursors, and ETags remove
duplicate contracts while exceptional flows stay explicit. Test support remains
separate from runtime code.

A monolith and direct SQL fit a personal tool without substitution layers.
Explicit collaborators prevent divergent assembly; narrower financial boundaries
let presentation evolve independently. Costs include shared database ownership,
manual deletion-order maintenance and single-instance operation. Leased
Collection Cycles do not promise exactly-once collection or multi-instance
coordination.

[Architecture](../architecture.md) owns dependency direction, mechanisms, and
limits; OpenAPI and migrations own wire and schema details.
[Testing](../testing.md) owns enforcement and test boundaries.
