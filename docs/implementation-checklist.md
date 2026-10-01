# Implementation checklist

Living progress tracker for [technical-design.md](./technical-design.md). Check an item in the same commit that finishes it. Phases 1–3 were completed on the previous model and are kept as history. Phase 4 restructured the code to ADRs 0014–0021 and is complete. Phase 5 (live Adapters) is the current workstream.

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

- [x] Load process config from `config/local.toml` (or `--config`) with `PLUTUS_*` overrides
- [x] Accept the MySQL DSN from the selected gitignored env file, with an optional exclusive `PLUTUS_DSN` or `PLUTUS_DSN_FILE` override
- [x] Accept the master key from exactly one of `PLUTUS_MASTER_KEY` or `PLUTUS_MASTER_KEY_FILE` when provided
- [x] Validate the HTTP / HTTPS / mTLS startup matrix and refuse invalid combinations
- [x] Implement `plutus serve` with structured `log/slog` logging
- [x] Implement `GET /health/live`
- [x] Fail `GET /health/ready` when MySQL is unavailable (schema and worker checks come later)
- [x] Commit `config/*-example.toml` and `config/*-example.env` with no secrets
- [x] Gitignore `config/local.toml`, `config/local.env`, `config/agent.toml`, `config/agent.env`, and `config/test.env` (the `walk` lane was later removed and `config/gotest.env`, `test.*`, `prod.*` added; see Phase 5 stage 0)

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

- [x] Login, logout, logout-all, me, password change, session list and revocation
- [x] Session cookie, CSRF, and login rate limiting
- [x] Argon2id PHC password hashing with rehash-on-login when parameters are obsolete
- [x] `plutus user create`, `user disable`, and `user reset-password`
- [x] HTTP registration is absent; CLI user creation remains the only way to add Users

### Milestone 6 — Portfolios

- [x] List, create, get, patch, and cascade delete of additional Portfolios; the birth Portfolio cannot be deleted
- [x] CLI user creation creates an active undeletable Portfolio named `default`
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
- [x] Working `/login`, Portfolio switcher
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

- `users`, `sessions`
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
- [x] Deterministic `walkthrough` source-fact seed for local UI review
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

Completed. Stopped for review after every item in this section was checked.

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

- [x] Working Exposure, valuation history, and Value Change history screens

### Milestone 8 — Management pages

- [x] Data, Jobs, Connection, Catalog, Observation, and audit screens
- [x] Catalog mapping and tracking revision UI
- [x] Job diagnostics, Observation correction, and full Projection rebuild from the UI

### Milestone 9 — Phase 3 close-out

- [x] Complete accessibility and error-recovery coverage
- [x] Deterministic testing of missing, stale, coverage, price, and timezone cases
- [x] README describes implemented Phase 3 capabilities
- [x] `make format && make build && make test && make lint` pass

## Acceptance matrix

The Phase 1–3 matrix covered the previous model (Reported Equity scopes, projection generations, revisions). Those rows were retired with the restructure; the current matrix is technical-design §13, tracked here. Coverage labels: **test** (automated in `go test`), **seed** (asserted by the `edge-cases` integration test in `db/seeds`), **lane** (exercised on the Agent lane against the fake Adapter, through the SPA and `curl`), **build** (verified by the make gates and by `make generate` leaving the tree unchanged), **Phase 5** (needs a live Adapter).

### 13.1 Ownership

- [x] **test** — User A receives 404 when addressing User B's Portfolio or nested resource by a known UUID
- [x] **test** — Composite FKs reject linking an Identity, Account, Account Report, Capital Flow, or job to another Portfolio
- [x] **test** — The Portfolio created with a User cannot be deleted; additional Portfolios cascade-delete with their reports and points

### 13.2 Collection and valuation

- [x] **seed** — A cycle with all Connections succeeding writes one point whose total equals the sum of Account values
- [x] **seed** — A failed Account keeps its previous report, marked `stale` with the earlier `reportObservedAt`; the total does not drop
- [x] **lane** — A successful report that omits a previously held coin removes it from holdings and from the point
- [x] **seed** — An Account with no successful report is `missing` with `valueUsd: null` and makes the point `incomplete`
- [x] **seed** — A Balance with no price is excluded, named in `omitted`, and marks the Account and point `incomplete`
- [x] **test** — USDT and USDC value at exactly 1 USD without a price row
- [x] **seed** — Reported equity wins over the Balance sum when both exist

### 13.3 Boundary, flows, and periods

- [x] **seed** — Selecting a funded Account produces a contribution Boundary Event; deselecting produces a withdrawal; the spanning Value Change subtracts both
- [x] **seed** — A BTC Capital Flow is valued at the nearest Platform price; a flow without a price within 24 hours leaves the Value Change `incomplete` and names the flow
- [x] **seed** — A day without points has no Period Close; the week containing it is `incomplete`
- [x] **lane** — Changing the Portfolio timezone changes Period Close selection without any write

### 13.4 Connections and Adapters

- [x] **lane** — Enabling a Connection without a successful validation is refused with 422
- [x] **test** — An authentication error sets the Connection `invalid` and the scheduler skips that Identity (fake Adapter)
- [x] **test** — A rate-limit error is retried and does not set `invalid` (fake Adapter)
- [ ] **Phase 5** — A selected Account Type the Platform reports as not enabled yields a `failed` report with the Platform reason; sibling Accounts succeed
- [ ] **Phase 5** — No row, log, job detail, or response contains a raw Platform payload or credential material (verified against live Adapters)

### 13.5 Catalog

- [x] **test** — A new symbol on a second Platform maps to the existing Asset automatically
- [x] **seed** — A manual mapping override recalculates the affected Portfolios and stamps `recalculated_at`

### 13.6 HTTP, security, frontend

- [x] **test** — Decimal strings round-trip without floating-point loss; unknown values round-trip as `null`
- [x] **lane** — ETag mismatch returns 412; an exact `Idempotency-Key` retry replays the response and a changed body returns 409
- [x] **lane** — Domain incompleteness returns typed 2xx; malformed input and infrastructure failure return Problem Details with `traceId`
- [x] **build** — Generated Go and TypeScript artifacts are unchanged after contract regeneration
- [x] **test** — Loopback HTTP, HTTPS, and mTLS start or refuse exactly as the startup matrix specifies
- [x] **lane** — Themes, a phone-width layout without horizontal scrolling, keyboard activation, chart text summaries, and the 401 redirect to login work on the implemented routes; network-loss recovery is not exercised automatically

## Phase 4: Restructure to the accepted model

Completed. The previous schema was dropped, not migrated; every lane database was reset.

### Milestone 1 — Schema and generated code

- [x] Rewrite the baseline Goose migration to technical-design §5 (16 tables); remove every table, column, job kind, and price type listed there as removed
- [x] Regenerate sqlc models and the expected schema version
- [x] Rewrite OpenAPI to §6; regenerate Go and TypeScript artifacts; delete removed operations

### Milestone 2 — Modules

- [x] Replace `observation`, `projection`, and `access` with `collection` and `valuation`; delete generations, correction, coverage, Exposure, pins, baseline, and revision code
- [x] Merge the Connection into the Identity; remove `parent_identity_id`, `kind`, and `disabled`; add Account selection with `selected` toggling and audit
- [x] Adapter port per §3.5 with `Capabilities`, `Validate`, and `Collect`; deterministic `fake` Adapter for seeds, tests, and lanes
- [x] Asset Listings map by normalized symbol on first sight; manual override enqueues `valuation_recalc`

### Milestone 3 — Collection Cycle and valuation

- [x] Scheduler enqueues one `collection_cycle` job per Portfolio at its interval; `POST /collection-cycles` merges into the same job
- [x] Worker runs Identities sequentially with stagger and writes Account Reports, Balances, Positions, and prices per job
- [x] Connection `invalid` on auth errors with scheduling stopped; bounded retry on rate limit and temporary errors
- [x] Valuation writer: reported-else-calculated, USDT/USDC at 1, stale fallback, `missing`, `omitted`, issues; one point per cycle in one transaction
- [x] Read-time Value Change, Period Close, custom range, and Boundary Event derivation

### Milestone 4 — HTTP and frontend

- [x] `GET /overview`, `/tree`, per-Account balances and positions, flat balances and positions with grouping
- [x] `GET /valuation-points`, `/period-closes`, `/value-changes`, `/value-changes/custom`, `/boundary-events`
- [x] Capital Flow CRUD without revisions; Identity create with `selectedAccountTypes`; credentials, validate, enable, disable, archive
- [x] Holdings tree, History, Capital, Identities (with Account checkboxes), Catalog, Activity pages; removed routes deleted

### Milestone 5 — Seeds and close-out

- [x] Rewrite `minimal`, `edge-cases`, and `walkthrough` seeds as Go generators over the fake scenario; `edge-cases` covers the §13.2–13.3 cases and is asserted by an integration test
- [x] `plutus collect --portfolio` runs one cycle with the configured Adapter; `plutus projection rebuild` removed
- [x] README describes the restructured capabilities; `CONTEXT.md` and ADRs re-verified against the code
- [x] `make format && make build && make test && make lint` pass; Agent lane exercised on `:8081`

## Phase 5: Integrations

Current workstream. Stage 0 (workflow infrastructure), Stages 1–4 (the Binance, OKX, Bybit, and Kraken Adapters), and Stage 5 (Account Structures, Margin Modes, and dual investment) are implemented; every Adapter awaits validation against the exchange apps on the test environment.

### Stage 0 — Environments and test workflow (ADRs 0022–0024)

- [x] dev / test / prod environments: `walk` lane removed, `test-example` and `prod-example` config pairs added, `scripts/agent-serve.sh` added, lane scripts renamed in their messages
- [x] `go test` reads `PLUTUS_GOTEST_*` from `config/gotest.env`; `mysql.RunTestMain` creates and drops one `plutus_gotest_<unix>_<hex>` database per package and sweeps stale ones; `db/seeds` no longer needs `plutus_seedcheck`
- [x] Independent recorded evidence and synthetic tests with single-request recording, replacing coupled sample promotion and `make test-live` (ADR 0037). Ordinary replay installs the exchange-network guard.
- [x] Separate `PLUTUS_RECORD_*` credentials in gitignored `config/record.env` from database-only `config/gotest.env`; ordinary tests use independent local synthetic inputs without recording keys (ADR 0038).
- [ ] Owner-prepared per-interface recording, review and freeze in Binance → OKX → Bybit → Kraken order; compatible holdings share preparation rounds and only critical differences require setting changes. Previous evidence and scratch directories were reset; all four Platforms await new reviewed recordings.
- [x] `make dist` cross-compiles a static Linux binary; `make build` and `make dist` inject `git describe` into `plutus --version`
- [x] `db reset` without `--profile` drops and migrates only
- [x] `kraken` accepted by the four `platform` CHECK constraints and the Platform constants, without any Account Structure; `OfferedPlatforms` hides it from the API and the fake
- [x] ADR 0022 (environments and manual deployment), ADR 0023 (adapter test layers and fixture recording), ADR 0024 (baseline freeze); ADR 0010 superseded; `AGENTS.md` and README rewritten

### Stage 1 — Binance

- [x] `plutus adapter record` subcommand writing trimmed, redacted evidence to `tmp/`
- [x] Binance Portfolio Margin Adapter with its declared Account Types, `httptest` fixture tests, offline failure and completeness tests, and `docs/adapters/binance.md` mapping table with the exchange-app comparison checklist
- [x] Register live Adapters when `[collection] fake_adapter = false`; keep the fake for tests and dev

### Stage 2 — OKX

- [x] OKX Adapter for `okx_mcm` and `okx_pm` on one code path, with the account level checked at validation and at the start of every Collection Run (ADR 0025), `httptest` fixture tests, a collection fixture test, offline per-Account completeness assertions, and `docs/adapters/okx.md`
- [x] `plutus adapter record --platform okx`; the recorders share the flat `fixture.File` contract, one request per command, with semantic filename variants
- [x] Inverse contracts: the USD notional of a Position is contracts × contract size, without the mark price. Covered by a valuation unit test and synthetic Binance COIN-M positions; the fake Adapter and seeds hold no inverse contract, so the dev environments never show this path
- [x] The credentials form requires the passphrase for OKX
- [ ] Validate the `okx_mcm` Margin Mode on the test environment with the MCM sub-account entered as its own Identity with its own read-only key
- [x] Futures mode (`acctLv = 2`) and Spot mode (`acctLv = 1`) as observed Margin Modes (ADR 0028; see Stage 5)
- [x] Binance: re-check the Account Structure in every Collection Run (ADR 0025)

### Stages 3–4 — Bybit, Kraken

- [x] Bybit global UTA 2.0 Adapter, flexible/fixed savings scope, synthetic interface replay, single-request recording (ADR 0037), and `docs/adapters/bybit.md`
- [x] Retain adapter-local signing, clock, and bounded transport logic: Bybit REST and Kraken REST/WebSocket protocols have different normalization and failure boundaries
- [x] Kraken ADR 0026, Separate wallets / Unified Wallet, dual credentials, appended migration (folded back into the baseline in Stage 5), synthetic interface replay, single-request recording (ADR 0037), and `docs/adapters/kraken.md`
- [ ] Owner review of the Bybit/Kraken recordings before promoting evidence
- [ ] Live nonempty Bybit fixed savings and Kraken Separate / Coin-M scenarios (documentation-derived tests cover these paths)
- [ ] Live validation of stale, complete-set, boundary, and no-price rules against real read-only keys on the test environment
- [ ] README marks integrations as implemented only after live validation

### Stage 5 — Account Structures, Margin Modes, and dual investment (ADRs 0028–0029)

- [x] Split the Account Mode into the declared, immutable Account Structure and the observed Margin Mode: catalog, baseline schema edited in place with the Kraken migration folded back, Identity service, API, UI, seeds; ADR 0025 narrowed to the structure and ADR 0005 rewritten
- [x] The Margin Mode is read at validation and in every Collection Run, stored on the Identity, and a change is audited with no actor; an unknown Margin Mode fails with `unsupported`
- [x] Validation refuses credentials that reach a different Platform account than the Identity's external reference
- [x] Fake Adapter: every Account Type of every structure, the Margin Mode picked by the key, `switch-` and `restructured-` key prefixes, figures a Margin Mode does not report left null
- [x] Shared per-Account batch assembly and field helpers in `internal/adapters`; the Bybit and Kraken Adapters follow the Binance/OKX conventions; a nonzero amount without a coin fails as `normalization` instead of vanishing
- [x] Every recorder records the Platform's authentication refusals, and the replay tests classify them from those recordings (ADR 0023)
- [x] Binance Classic Trading with eight Account Types, from independently constructed synthetic tests based on official contracts
- [x] OKX Spot mode and Futures mode, Bybit Isolated Margin and Cross Margin, from independently constructed synthetic tests based on official contracts
- [x] Dual investment in the Earn Account on Binance, OKX, and Bybit, and the Earn completeness check against each Platform's own Earn valuation
- [ ] Owner review of the new recorded evidence before promotion
- [ ] Reset every database (agent, local, test) and re-enter the test Identities with their Account Structures
- [ ] Live validation on the test environment: Binance Classic Trading, OKX Spot / Futures / Multi-currency margin, Bybit Isolated / Cross Margin, Kraken Separate wallets, and a running dual investment subscription on Binance, OKX, and Bybit
