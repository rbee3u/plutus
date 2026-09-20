# Implementation checklist

Living progress tracker for the MVP in [technical-design.md](./technical-design.md). Check an item in the same commit that finishes it. Phase 3 is the current workstream; Phase 4 stays unchecked until it begins. Stop for review when every item in Phase 3 is checked.

## Phase 1: Foundation

Completed. Stopped for review after every item in this section was checked.

### Milestone 1 — Checklist and repository skeleton

- [x] Add this checklist covering phases 1–4 and the acceptance matrix
- [x] Move the CLI composition root to `cmd/plutus`
- [x] Remove the placeholder `demo` command
- [x] Create the planned directory layout without empty pass-through packages
- [x] Point Make `build` and `run` at `./cmd/plutus`
- [x] Keep README honest that application behavior is still unimplemented

### Milestone 2 — Configuration and serve skeleton

- [x] Load process config from `plutus.toml` with `PLUTUS_*` overrides
- [x] Accept the MySQL DSN from gitignored `plutus.toml`, with an optional exclusive `PLUTUS_DATABASE_DSN` or `PLUTUS_DATABASE_DSN_FILE` override
- [x] Accept the master key from exactly one of `PLUTUS_MASTER_KEY` or `PLUTUS_MASTER_KEY_FILE` when provided
- [x] Validate the HTTP / HTTPS / mTLS startup matrix and refuse invalid combinations
- [x] Reject non-loopback cleartext startup when public registration is enabled
- [x] Implement `plutus serve` with structured `log/slog` logging
- [x] Implement `GET /health/live`
- [x] Fail `GET /health/ready` when MySQL is unavailable (schema and worker checks come later)
- [x] Commit `configs/plutus.example.toml` with no secrets
- [x] Gitignore `.env` and a local `plutus.toml`

### Milestone 3 — Migrations and sqlc

- [x] Pin Goose and sqlc; add `make generate` for sqlc
- [x] Create Phase 1 tables only (see schema list below)
- [x] Seed USD, CNY, and enabled Binance / OKX / Bybit platform rows with stable UUIDs
- [x] Implement `plutus migrate status` and `plutus migrate up`
- [x] Never migrate automatically on `serve`
- [x] Include expected schema version in `/health/ready`

### Milestone 4 — OpenAPI and code generation

- [x] Author Phase 1 OpenAPI 3.1 under `api/openapi/`
- [x] Generate and commit the strict Gin server interface and Go wire models
- [x] Generate and commit the TypeScript client and wire models
- [x] Make regeneration reproducible with pinned commands
- [x] Wire HTTP transport to the generated contract (handlers may still be incomplete)

### Milestone 5 — Authentication

- [x] Registration toggle, login, logout, logout-all, me, password change, session list and revocation
- [x] Session cookie, CSRF, and login/registration rate limiting
- [x] Argon2id PHC password hashing with rehash-on-login when parameters are obsolete
- [x] `plutus user create`, `user disable`, and `user reset-password`
- [x] Disabled registration rejects HTTP creation while CLI user creation remains usable

### Milestone 6 — Portfolios

- [x] List, create, get, patch, archive, restore, and empty-only delete
- [x] Registration and CLI user creation create an active Portfolio named `Main`
- [x] Timezone default `Asia/Shanghai`; display Asset USD by default, CNY supported
- [x] Cross-user Portfolio access returns HTTP 404
- [x] ETags / `If-Match` and `Idempotency-Key` on the relevant mutations
- [x] Identity CRUD and credential-less Connection API (no validate, no credential fields)

### Milestone 7 — Jobs and projection worker

- [x] Durable jobs with leases, attempts, dedupe, and `SKIP LOCKED` claims
- [x] Empty Platform adapter registry
- [x] Projection handler succeeds with a diagnostic empty result and does not switch `active_projection_generation_id`
- [x] `GET` job list and job detail
- [x] `plutus projection rebuild --portfolio` enqueues work only

### Milestone 8 — Embedded React shell

- [x] Copy AIBox tokens, shell, themes, and navigation density with no runtime dependency
- [x] Working `/login`, `/register` (explains disabled registration), Portfolio switcher
- [x] Working `/p/:portfolioId/settings/portfolio` and `/settings/security`
- [x] Full information-architecture navigation with Phase placeholders for unimplemented routes
- [x] Vite dev proxy for `/api`, `/health`, and cookies; production `go:embed` of the hashed build

### Milestone 9 — Phase 1 close-out

- [x] README describes only implemented Phase 1 capabilities
- [x] Mark credential ciphertext and nonce nullable in the technical design
- [x] All Phase 1 checklist items checked
- [x] `make format && make build && make test && make lint` pass, plus frontend typecheck/lint when `web/` exists

### Phase 1 schema (normative for this workstream)

Create these tables and no others:

- `users`, `user_credentials`, `sessions`
- `assets`, `platforms`
- `portfolios`
- `identities`, `connections` (nullable ciphertext and nonce), `accounts`
- `jobs`, `job_attempts`, `projection_job_details`, `projection_job_kinds`
- `projection_generations`
- `audit_events`, `idempotency_records`

Phase 1 deferred these tables; Phase 2 creates them.

## Phase 2: End-to-end vertical slice

Completed. Stopped for review after every item in this section was checked.

### Milestone 1 — Schema

- [x] Add Phase 2 catalog, collection, observation, capital, and projection-row tables
- [x] Update sqlc models and the expected Goose schema version
- [x] Treat Observation, Capital, mapping, pin, and collection-run rows as Portfolio delete blockers

### Milestone 2 — Seed and reset CLI

- [x] Deterministic `minimal` source-fact seed
- [x] `plutus db seed` and development-only `db reset`

### Milestone 3 — Facts and correction primitives

- [x] Shared catalog, mappings, normalized Observations, failures, and correction primitives
- [x] Freeze Identity identity fields after the first Observation exists
- [x] `GET /accounts`, `GET /catalog/assets`, and `PATCH /observations/{observationId}`

### Milestone 4 — Capital API

- [x] Capital Baseline and Capital Flow revisions over HTTP

### Milestone 5 — Projection engine

- [x] Generation-based latest Balance and Position state, Portfolio Valuation, Contributed Capital, Value Change, and Exposure rows
- [x] Activate a ready generation atomically; leave the previous generation readable on failure

### Milestone 6 — Read-model HTTP

- [x] `GET /overview`, `GET /balances`, and `GET /positions` with context, issues, and grouping

### Milestone 7 — Frontend pages

- [x] Working Overview, Balances, Positions, and Capital pages backed by MySQL and the JSON API

### Milestone 8 — Edge-case seed

- [x] Deterministic `edge-cases` source-fact seed

### Milestone 9 — Phase 2 close-out

- [x] README describes implemented Phase 2 capabilities
- [x] Check Phase 2 acceptance-matrix items that this workstream covers
- [x] `make format && make build && make test && make lint` pass

## Phase 3: Complete MVP UI and management

Current workstream. Stop for review when every item in this section is checked.

### Milestone 1 — OpenAPI growth

- [x] Mark Phase 3 as the current workstream
- [x] Grow OpenAPI for remaining Phase 3 endpoints
- [x] Generate Go and TypeScript artifacts
- [x] Add compiling transport stubs for new operations

### Milestone 2 — Connection credentials

- [x] Connection credential CRUD with AES-256-GCM and no real Platform call
- [x] Require the master key for readiness when ciphertext exists

### Milestone 3 — Catalog mapping and pins

- [x] Catalog listing and instrument search with mapping and pin status
- [x] Mapping and tracking revision HTTP that enqueues a Projection rebuild

### Milestone 4 — Observations and audit

- [x] Observation search and typed GET
- [x] Portfolio audit-event cursor listing

### Milestone 5 — Remaining read-model HTTP

- [x] `GET /exposures`, `GET /valuations`, `GET /value-changes`, and `GET /period-summaries`

### Milestone 6 — HTTP projection rebuilds

- [x] HTTP `POST` projection-rebuilds

### Milestone 7 — Exposure and history pages

- [ ] Working Exposure, valuation history, and Value Change history screens

### Milestone 8 — Management pages

- [ ] Data, Jobs, Connection, Catalog, Observation, and audit screens
- [ ] Catalog mapping and tracking revision UI
- [ ] Job diagnostics, Observation correction, and full Projection rebuild from the UI

### Milestone 9 — Phase 3 close-out

- [ ] Complete accessibility and error-recovery coverage
- [ ] Deterministic testing of missing, stale, coverage, price, and timezone cases
- [ ] README describes implemented Phase 3 capabilities
- [ ] `make format && make build && make test && make lint` pass

## Phase 4: Integrations

- [ ] Active scheduler and manual Identity refresh
- [ ] Binance Portfolio Margin adapter
- [ ] OKX Multi-currency margin and Portfolio margin adapters
- [ ] Bybit Unified Trading Account 2.0 adapter
- [ ] Platform validation against documented Account, coverage, catalog, price, and failure semantics

## Acceptance matrix

Labels: **Phase 1** and **Phase 2** mark workstreams that covered the item. **Later** waits for a later phase.

### 13.1 Ownership and identity

- [x] **Phase 1** — User A receives 404 when addressing User B's Portfolio or nested resource by a known UUID
- [x] **Phase 1** — One User can create and switch between at least two Portfolios without query-cache or URL leakage
- [x] **Phase 2** — A composite FK rejects linking an Identity, Account, Connection, Observation, Capital Flow Account, or job to another Portfolio
- [x] **Phase 1** — An Identity can be edited before facts exist
- [x] **Phase 2** — An Identity cannot move to another Portfolio after any Observation exists
- [x] **Phase 2** — An empty Portfolio can be deleted; a Portfolio with Observation, Capital, mapping, pin, or collection-run rows can only be archived

### 13.2 Authentication and browser security

- [x] **Phase 1** — Disabled registration rejects creation while login and CLI User creation remain usable
- [x] **Phase 1** — Login rate limiting works by username and client address without revealing existence
- [x] **Phase 1** — Correct and incorrect passwords exercise Argon2id; password change revokes other Sessions
- [x] **Phase 1** — Current-Session logout, all-Session logout, individual Session revocation, idle expiry, absolute expiry, User disable, and CLI password reset revoke access
- [x] **Phase 1** — Every state-changing browser request requires a valid CSRF token

### 13.3 Collection and normalized facts

- [x] **Phase 2** — Complete, partial, stale, failed, and missing collections remain distinguishable
- [x] **Phase 2** — A partial collection preserves successful Observations and records failed scopes without zero replacements
- [x] **Phase 2** — Different source observation times within one Identity Snapshot are preserved
- [x] **Phase 2** — Observation correction audits before/after, increments ETag version, and merges a Projection job
- [x] **Phase 1** — No database row, log, audit event, job failure, or API response contains a raw Platform payload

### 13.4 Coverage, prices, and calculations

- [x] **Phase 2** — Disjoint Reported Equity scopes combine without double counting
- [x] **Phase 2** — A broad complete Reported Equity scope supersedes strictly contained reported values
- [x] **Phase 2** — Partially overlapping Reported Equity scopes yield ambiguity and incomplete valuation
- [x] **Phase 2** — Calculated Valuation fills only uncovered scopes and is incomplete when mapping or Price is missing
- [x] **Phase 2** — A stablecoin requires an observed exchange rate
- [x] **Phase 2** — Price selection respects Platform, instrument or asset, price type, and cutoff; no cross-Platform fallback
- [x] **Phase 2** — Opposing Positions preserve long and short Exposure while net Exposure is zero
- [x] **Phase 2** — Missing, stale, and incomplete facts remain `null` or issue-bearing rather than zero

### 13.5 Capital, generations, and time

- [x] **Phase 2** — Editing a Capital Flow appends a revision, audits, and creates a Projection job
- [x] **Phase 2** — A rebuild writes a new generation without changing readers, then atomically switches the Portfolio pointer only after validation
- [x] **Phase 2** — A failed rebuild leaves the previous active generation fully readable
- [x] **Phase 2** — Day, ISO-week, month, quarter, leap-date, and year boundaries convert correctly to UTC
- [x] **Phase 2** — Changing Portfolio timezone triggers a full rebuild without mixing timezones
- [x] **Phase 2** — A coverage change makes its spanning Value Change incomparable

### 13.6 HTTP contracts

- [x] **Phase 1** — Decimal strings round-trip without binary floating-point loss when present; unknown values round-trip as `null`
- [x] **Phase 1** — ETag mismatch returns 412 and never overwrites the newer revision
- [x] **Phase 1** — An exact Idempotency-Key retry returns the original response; a changed body returns 409
- [x] **Phase 1** — Offset pagination remains stable; cursor pagination neither duplicates nor skips under its contract (jobs/sessions as applicable)
- [x] **Phase 1** — Domain incompleteness returns typed 2xx; malformed input and infrastructure failure return Problem Details
- [x] **Phase 1** — Generated Go and TypeScript artifacts are unchanged after contract regeneration
- [x] **Phase 2** — `GET /overview`, `GET /balances`, `GET /positions`, and Capital endpoints return decimal strings or `null`, typed 2xx incompleteness, and 404 across Users

### 13.7 Transport security

- [x] **Phase 1** — Loopback HTTP starts with the default configuration
- [x] **Phase 1** — HTTPS starts with a valid certificate/key pair and negotiates TLS 1.3 or newer
- [x] **Phase 1** — mTLS accepts a valid client certificate and rejects absent, expired, or untrusted certificates on UI, API, and health routes
- [x] **Phase 1** — Every invalid combination in the startup matrix refuses startup with a safe actionable error
- [x] **Phase 1** — Non-loopback cleartext startup with public registration enabled is rejected

### 13.8 Frontend

- [x] **Phase 1** — System, light, and dark themes render implemented routes with semantic colors
- [x] **Phase 1** — Desktop, tablet, and narrow mobile layouts preserve usable navigation
- [x] **Phase 1** — Implemented interactive flows work by keyboard with visible focus
- [x] **Phase 2** — Overview charts provide a text summary and accessible exact-value table
- [x] **Phase 1** — Network failures and 401 Session loss present a specific recovery path on implemented screens
- [x] **Phase 2** — Capital 412 conflicts reload the latest server values
