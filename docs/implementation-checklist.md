# Implementation checklist

Living progress tracker for the MVP in [technical-design.md](./technical-design.md). Check an item in the same commit that finishes it. Phase 1 is the current workstream; phases 2–4 stay unchecked until their workstreams begin.

## Phase 1: Foundation

Current workstream. Stop for review when every item in this section is checked.

### Milestone 1 — Checklist and repository skeleton

- [x] Add this checklist covering phases 1–4 and the acceptance matrix
- [x] Move the CLI composition root to `cmd/plutus`
- [x] Remove the placeholder `demo` command
- [x] Create the planned directory layout without empty pass-through packages
- [x] Point Make `build` and `run` at `./cmd/plutus`
- [x] Keep README honest that application behavior is still unimplemented

### Milestone 2 — Configuration and serve skeleton

- [x] Load non-secret config from `plutus.toml` with `PLUTUS_*` overrides
- [x] Accept the MySQL DSN from exactly one of `PLUTUS_DATABASE_DSN` or `PLUTUS_DATABASE_DSN_FILE`
- [x] Accept the master key from exactly one of `PLUTUS_MASTER_KEY` or `PLUTUS_MASTER_KEY_FILE` when provided
- [x] Validate the HTTP / HTTPS / mTLS startup matrix and refuse invalid combinations
- [x] Reject non-loopback cleartext startup when public registration is enabled
- [x] Implement `plutus serve` with structured `log/slog` logging
- [x] Implement `GET /health/live`
- [x] Fail `GET /health/ready` when MySQL is unavailable (schema and worker checks come later)
- [x] Commit `configs/plutus.example.toml` with no secrets
- [x] Gitignore `.env` and a local `plutus.toml`

### Milestone 3 — Migrations and sqlc

- [ ] Pin Goose and sqlc; add `make generate` for sqlc
- [ ] Create Phase 1 tables only (see schema list below)
- [ ] Seed USD, CNY, and enabled Binance / OKX / Bybit platform rows with stable UUIDs
- [ ] Implement `plutus migrate status` and `plutus migrate up`
- [ ] Never migrate automatically on `serve`
- [ ] Include expected schema version in `/health/ready`

### Milestone 4 — OpenAPI and code generation

- [ ] Author Phase 1 OpenAPI 3.1 under `api/openapi/`
- [ ] Generate and commit the strict Gin server interface and Go wire models
- [ ] Generate and commit the TypeScript client and wire models
- [ ] Make regeneration reproducible with pinned commands
- [ ] Wire HTTP transport to the generated contract (handlers may still be incomplete)

### Milestone 5 — Authentication

- [ ] Registration toggle, login, logout, logout-all, me, password change, session list and revocation
- [ ] Session cookie, CSRF, and login/registration rate limiting
- [ ] Argon2id PHC password hashing with rehash-on-login when parameters are obsolete
- [ ] `plutus user create`, `user disable`, and `user reset-password`
- [ ] Disabled registration rejects HTTP creation while CLI user creation remains usable

### Milestone 6 — Portfolios

- [ ] List, create, get, patch, archive, restore, and empty-only delete
- [ ] Registration and CLI user creation create an active Portfolio named `Main`
- [ ] Timezone default `Asia/Shanghai`; display Asset USD by default, CNY supported
- [ ] Cross-user Portfolio access returns HTTP 404
- [ ] ETags / `If-Match` and `Idempotency-Key` on the relevant mutations
- [ ] Identity CRUD and credential-less Connection API (no validate, no credential fields)

### Milestone 7 — Jobs and projection worker

- [ ] Durable jobs with leases, attempts, dedupe, and `SKIP LOCKED` claims
- [ ] Empty Platform adapter registry
- [ ] Projection handler succeeds with a diagnostic empty result and does not switch `active_projection_generation_id`
- [ ] `GET` job list and job detail
- [ ] `plutus projection rebuild --portfolio` enqueues work only

### Milestone 8 — Embedded React shell

- [ ] Copy AIBox tokens, shell, themes, and navigation density with no runtime dependency
- [ ] Working `/login`, `/register` (explains disabled registration), Portfolio switcher
- [ ] Working `/p/:portfolioId/settings/portfolio` and `/settings/security`
- [ ] Full information-architecture navigation with Phase placeholders for unimplemented routes
- [ ] Vite dev proxy for `/api`, `/health`, and cookies; production `go:embed` of the hashed build

### Milestone 9 — Phase 1 close-out

- [ ] README describes only implemented Phase 1 capabilities
- [ ] Mark credential ciphertext and nonce nullable in the technical design
- [ ] All Phase 1 checklist items checked
- [ ] `make format && make build && make test && make lint` pass, plus frontend typecheck/lint when `web/` exists

### Phase 1 schema (normative for this workstream)

Create these tables and no others:

- `users`, `user_credentials`, `sessions`
- `assets`, `platforms`
- `portfolios`
- `identities`, `connections` (nullable ciphertext and nonce), `accounts`
- `jobs`, `job_attempts`, `projection_job_details`, `projection_job_kinds`
- `projection_generations`
- `audit_events`, `idempotency_records`

Deferred schema: asset listings, instruments, mappings, pins, observations and subtypes, collection runs/failures, `collection_job_details`, capital tables, and projection row tables.

## Phase 2: End-to-end vertical slice

- [ ] Deterministic `minimal` and `edge-cases` source-fact seeds
- [ ] `plutus db seed` and development-only `db reset`
- [ ] Shared catalog, mappings, normalized Observations, failures, and correction primitives
- [ ] Capital Baseline and Capital Flow revisions
- [ ] Generation-based latest Balance and Position state, Portfolio Valuation, Contributed Capital, and Value Change
- [ ] Working Overview, Balances, Positions, and Capital pages backed by MySQL and the JSON API

## Phase 3: Complete MVP UI and management

- [ ] Exposure, valuation history, Value Change history, Data, Jobs, Connection, Catalog, Observation, and audit screens
- [ ] Connection credential CRUD with AES-256-GCM and no real Platform call
- [ ] Catalog mapping and tracking revision UI
- [ ] Job and issue diagnostics, Observation correction, full Projection rebuild
- [ ] HTTP `POST` projection-rebuilds
- [ ] Complete accessibility and error-recovery coverage
- [ ] Deterministic testing of missing, stale, coverage, price, and timezone cases

## Phase 4: Integrations

- [ ] Active scheduler and manual Identity refresh
- [ ] Binance Portfolio Margin adapter
- [ ] OKX Multi-currency margin and Portfolio margin adapters
- [ ] Bybit Unified Trading Account 2.0 adapter
- [ ] Platform validation against documented Account, coverage, catalog, price, and failure semantics

## Acceptance matrix

Labels: **Phase 1** means this workstream must cover it; **Later** waits for a later phase.

### 13.1 Ownership and identity

- [ ] **Phase 1** — User A receives 404 when addressing User B's Portfolio or nested resource by a known UUID
- [ ] **Phase 1** — One User can create and switch between at least two Portfolios without query-cache or URL leakage
- [ ] **Later** — A composite FK rejects linking an Identity, Account, Connection, Observation, Capital Flow Account, or job to another Portfolio (schema in Phase 1; Observation/Capital cases in Phase 2)
- [ ] **Phase 1** — An Identity can be edited before facts exist
- [ ] **Later** — An Identity cannot move to another Portfolio after any Observation exists
- [ ] **Phase 1** — An empty Portfolio can be deleted; a Portfolio with source facts can only be archived (Phase 1 has no source facts yet; enforce the empty-only delete rule)

### 13.2 Authentication and browser security

- [ ] **Phase 1** — Disabled registration rejects creation while login and CLI User creation remain usable
- [ ] **Phase 1** — Login rate limiting works by username and client address without revealing existence
- [ ] **Phase 1** — Correct and incorrect passwords exercise Argon2id; password change revokes other Sessions
- [ ] **Phase 1** — Current-Session logout, all-Session logout, individual Session revocation, idle expiry, absolute expiry, User disable, and CLI password reset revoke access
- [ ] **Phase 1** — Every state-changing browser request requires a valid CSRF token

### 13.3 Collection and normalized facts

- [ ] **Later** — Complete, partial, stale, failed, and missing collections remain distinguishable
- [ ] **Later** — A partial collection preserves successful Observations and records failed scopes without zero replacements
- [ ] **Later** — Different source observation times within one Identity Snapshot are preserved
- [ ] **Later** — Observation correction audits before/after, increments ETag version, and merges a Projection job
- [ ] **Phase 1** — No database row, log, audit event, job failure, or API response contains a raw Platform payload

### 13.4 Coverage, prices, and calculations

- [ ] **Later** — Disjoint Reported Equity scopes combine without double counting
- [ ] **Later** — A broad complete Reported Equity scope supersedes strictly contained reported values
- [ ] **Later** — Partially overlapping Reported Equity scopes yield ambiguity and incomplete valuation
- [ ] **Later** — Calculated Valuation fills only uncovered scopes and is incomplete when mapping or Price is missing
- [ ] **Later** — A stablecoin requires an observed exchange rate
- [ ] **Later** — Price selection respects Platform, instrument or asset, price type, and cutoff; no cross-Platform fallback
- [ ] **Later** — Opposing Positions preserve long and short Exposure while net Exposure is zero
- [ ] **Later** — Missing, stale, and incomplete facts remain `null` or issue-bearing rather than zero

### 13.5 Capital, generations, and time

- [ ] **Later** — Editing a Capital Flow appends a revision, audits, and creates a Projection job
- [ ] **Phase 1** — A rebuild writes work without changing readers; Phase 1 must not switch the active generation
- [ ] **Later** — A failed rebuild leaves the previous active generation fully readable
- [ ] **Later** — Day, ISO-week, month, quarter, leap-date, and year boundaries convert correctly to UTC
- [ ] **Later** — Changing Portfolio timezone triggers a full rebuild without mixing timezones
- [ ] **Later** — A coverage change makes its spanning Value Change incomparable

### 13.6 HTTP contracts

- [ ] **Phase 1** — Decimal strings round-trip without binary floating-point loss when present; unknown values round-trip as `null`
- [ ] **Phase 1** — ETag mismatch returns 412 and never overwrites the newer revision
- [ ] **Phase 1** — An exact Idempotency-Key retry returns the original response; a changed body returns 409
- [ ] **Phase 1** — Offset pagination remains stable; cursor pagination neither duplicates nor skips under its contract (jobs/sessions as applicable)
- [ ] **Phase 1** — Domain incompleteness returns typed 2xx; malformed input and infrastructure failure return Problem Details
- [ ] **Phase 1** — Generated Go and TypeScript artifacts are unchanged after contract regeneration

### 13.7 Transport security

- [ ] **Phase 1** — Loopback HTTP starts with the default configuration
- [ ] **Phase 1** — HTTPS starts with a valid certificate/key pair and negotiates TLS 1.3 or newer
- [ ] **Phase 1** — mTLS accepts a valid client certificate and rejects absent, expired, or untrusted certificates on UI, API, and health routes
- [ ] **Phase 1** — Every invalid combination in the startup matrix refuses startup with a safe actionable error
- [ ] **Phase 1** — Non-loopback cleartext startup with public registration enabled is rejected

### 13.8 Frontend

- [ ] **Phase 1** — System, light, and dark themes render implemented routes with semantic colors
- [ ] **Phase 1** — Desktop, tablet, and narrow mobile layouts preserve usable navigation
- [ ] **Phase 1** — Implemented interactive flows work by keyboard with visible focus
- [ ] **Later** — Charts provide text summaries and accessible exact-value tables
- [ ] **Phase 1** — Network failures and 401 Session loss present a specific recovery path on implemented screens
