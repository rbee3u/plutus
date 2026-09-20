---
status: accepted
---

# Use a contract-first modular web monolith

Build the first release as one Go application that serves a versioned JSON API, runs collection and projection workers, and embeds a React frontend while using an externally managed MySQL database. OpenAPI 3.1 is the transport contract, generated Go and TypeScript code stops at the transport boundary, and domain and application modules remain independent of Gin, MySQL, and generated wire types.

## Consequences

- The production artifact is one application binary plus MySQL; local frontend development uses Vite with API proxying.
- The first release supports one active application instance. Persistent jobs use leases for restart recovery but do not promise multi-instance coordination.
- MySQL stores normalized source facts and replaceable projection generations according to [ADR 0007](./0007-persist-normalized-observations-and-rebuild-derived-results.md); platform response payloads are never retained.
- Platform adapters normalize external data behind application ports. The framework and synthetic-data stages do not make real Platform network calls.
- HTTP is the default on the loopback interface. Static PEM server TLS and optional mutual TLS are application configuration, without ACME or certificate hot reload.
