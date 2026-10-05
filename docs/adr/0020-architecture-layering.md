---
status: accepted
---

# ADR 0020: Backend layering, composition root, and unified seams

## Decision

Group `internal/` by layer and keep the modular-monolith style of
[ADR 0006](./0006-contract-first-modular-monolith.md). Services own
`database/sql` handles and generated sqlc queries directly; there is no
repository layer and no dependency-injection framework.

- `internal/shared` holds dependency-free packages (`decimal`, `ordering`,
  `apperr`, `cursor`, `etag`).
- `internal/infra` holds infrastructure (`mysql`, `logging`, `sqljson`,
  `secret`, `idempotency`).
- `internal/platform` holds the typed Platform boundary and its live and fake
  implementations; it is a leaf that imports only `shared`.
- `internal/domain` holds one package per bounded responsibility (`identity`,
  `portfolio`, `catalog`, `capital`, `manual`, `valuation`, `collection`,
  `jobs`, `audit`, `auth`, `listingexclusion`).
- `internal/transport/http` holds the generated contract bindings, browser
  security, and SPA serving. It stays one package; conversion helpers are split
  by domain into `convert_<domain>.go`.
- `internal/app` is the composition root. It builds every service on one
  database handle and exposes them as `app.Services`.
- `internal/cli` holds the cobra commands; `cmd/plutus/main.go` only builds the
  root command from `internal/cli` and exits.
- `internal/seeds` holds the development seed fixtures behind `plutus db seed`
  and `plutus db reset --seed`.

Dependencies flow one way: `shared` <- `infra` <- `domain` <- `transport`. The
transport layer accepts the `app.Services` aggregate as a value; `app` never
imports `transport` or `cli`, so the graph stays acyclic while the wiring type
stays with the code that builds it.

Five cross-cutting seams keep repetition out of the domains:

- Transactions use `mysql.WithTx` and `mysql.WithReadTx` closures instead of
  repeated begin, defer-rollback, and commit bookkeeping. Transaction sites
  whose body does not fit the closure shape stay hand-written.
- Errors use `shared/apperr`: domains return a `Kind`-tagged error and the
  transport maps each `Kind` to one HTTP problem once.
- The composition root is the only place that knows which concrete services,
  adapters, and worker exist.
- Keyset cursors use `shared/cursor`: one HMAC-signed, Portfolio-bound wire
  format, with each list binding its own version byte and key.
- ETag validators use `shared/etag`: one strong-validator format and one strict
  parser, so a malformed value never matches.

ETag parsing is strict: `etag.Parse` accepts only the exact validator
`etag.Format` emits and never version zero. This unifies four previously lenient
`Sscanf` parses with two already-strict sites, so a malformed `If-Match` now
uniformly yields 412 instead of matching by prefix.

Layering is documented and reviewed rather than lint-enforced: no `depguard` or
import-graph rule is configured.

## Reason and trade-offs

One package per responsibility keeps ownership legible while sharing SQL
infrastructure, matching a personal tool that one process runs. A single
composition root makes process startup, the CLI, and tests read the same wiring,
and it makes the service set cheap to extend when features are added.

Accepting `app.Services` in the transport layer is a deliberate, narrow reversal
of the layer arrow: it replaces an eleven-field options struct that every caller
had to keep in step. It does not create a cycle and does not let transport reach
the composition root's construction logic.

Keeping transaction and error handling explicit rather than generated keeps the
domains ordinary Go that a reviewer can follow. The trade-off is that
heterogeneous transaction bodies keep their hand-written bookkeeping until a
second shape earns a second helper.

Sharing the cursor codec and the ETag format keeps one wire format per concern
instead of near-identical copies per list. The trade-off is one extra hop
through `shared` for behavior that used to be local, which is why the codec
stays a plain struct with two methods rather than a framework.

Documenting layering instead of enforcing it avoids a lint configuration that
fights ordinary refactors. The trade-off is real: drift is caught by review, not
by tooling, so this record is the reference when a new package is added.

The transport package stays whole because the generated strict server interface
is package-scoped; splitting handlers into subpackages would fight the generator
rather than the problem.

[Architecture](../architecture.md) describes the current runtime mechanisms.
