---
status: accepted
---

# ADR 0020: Backend layering, composition root, and unified seams

## Decision

Keep the modular monolith of
[ADR 0006](./0006-contract-first-modular-monolith.md), grouped into `shared`,
`infra`, `platform`, `application`, and `transport`. Services use SQL and sqlc
directly; no repository layer or dependency-injection framework is needed.
`internal/app` assembles shared service instances; CLI owns process lifecycle.

Dependencies point toward lower layers. Application uses only the normalized
Adapter contract, not concrete implementations; that contract depends only on
shared internal mechanisms. Valuation cannot import Financial, and HTTP cannot
import SQL APIs. Transport accepts `app.Services` as a wiring value; App does
not import Transport or CLI. Depguard enforces these coarse boundaries; choices
within a layer remain subject to review.

Share transaction helpers, application error kinds, Portfolio-bound signed
cursors, and strict ETags. Keep exceptional transaction bodies explicit. ETags
accept only the exact formatted validator and reject version zero; malformed
`If-Match` yields 412. Each paginated list binds its own cursor version and key.

Reusable test infrastructure belongs in `internal/testsupport`, which runtime
code cannot import. Cross-module public scenarios belong in
`internal/integration`; private recovery and clock tests stay with their
package. Tests may assemble across layers without exposing private production
methods. CLI-used fake, fixture, and recording packages remain under Platform.

## Reason and trade-offs

One composition root prevents divergent wiring. Direct SQL avoids a storage
abstraction without a substitution requirement. Shared codecs and error mapping
remove duplicate contracts; exceptional transactions remain ordinary Go rather
than requiring a universal helper.

Accepting `app.Services` in HTTP avoids a second service-options structure at
the cost of a narrow dependency on assembly's type. The generated strict server
interface keeps handlers in one package. Coarse lint checks replace the earlier
review-only preference without maintaining a per-package dependency matrix.

Test separation makes ownership explicit while preserving local control over
private seams. A universal test factory would hide those differences.

[ADR 0022](./0022-application-boundaries.md) refines collaborator, financial,
and persistence ownership.
[Architecture](../architecture.md#module-responsibilities) owns the package map;
[testing](../testing.md#test-ownership) owns test setup.
