# Plutus

Plutus is a personal, read-only portfolio observation project. It is intended to periodically collect normalized account observations, persist their history, and present balances, positions, valuations, and basic directional exposure through a web frontend.

The domain hierarchy is `Platform -> Identity -> Account`: each main or subordinate identity on a financial Platform has its own funding or trading Accounts. A Connection configures access to one Identity. An authenticated Plutus User is a separate concept, owns one or more independent Portfolios, and directly manages the Identities selected into each Portfolio.

## Current status

Phase 3 (complete MVP UI and management) is implemented and waiting for review. The process is a Go CLI at `cmd/plutus` with an embedded React frontend:

- Configuration from TOML and `PLUTUS_*`, including `--config` / `--env-file` for isolated local lanes and the HTTP / HTTPS / mTLS startup matrix.
- Explicit Goose migrations and sqlc access. `serve` never migrates automatically.
- Session authentication (Argon2id, CSRF, rate limits), Portfolio CRUD, Identity CRUD, and write-only Connection credentials encrypted with AES-256-GCM. `/health/ready` fails when ciphertext exists without a master key.
- Development `plutus db seed` and `plutus db reset` with deterministic `minimal`, `edge-cases`, and `walkthrough` source-fact profiles. Seeds stay secret-free. Reset requires `[database].allow_reset = true` in the selected TOML and a database name matching `plutus` or `plutus_<suffix>`.
- Normalized Observations, catalog mapping and pin revisions, Observation correction, Capital Baseline / Flow revisions, and portfolio audit events.
- A durable Projection worker that rebuilds latest balances, positions, valuations, contributed capital, value changes, exposure, and period summaries, then atomically activates a ready generation. A failed rebuild leaves the previous generation readable. `POST /projection-rebuilds` enqueues a full rebuild.
- Working Overview, Balances, Positions, Exposure, Valuations, Value Changes, Capital, Identity, Connection, Catalog, Observation, Jobs, and Audit pages. Overview charts include a text summary and an accessible exact-value table. Query failures keep last-good data and show Retry plus a trace ID.

Not implemented yet: real Platform adapters, collection scheduling, Identity refresh, or Connection validate against a live Platform. Progress is tracked in the [implementation checklist](./docs/implementation-checklist.md).

## First-release scope

Initial integrations target these account modes:

| Platform | Account modes |
| --- | --- |
| Binance | Portfolio Margin (PM) |
| OKX | Multi-currency margin (MCM), Portfolio margin (PM) |
| Bybit | Unified Trading Account 2.0 (UTA 2.0) |

The product scope includes spot balances and stablecoin-margined (linear) and coin-margined (inverse) perpetual and delivery contracts. Availability and report coverage must be verified for each selected platform and mode.

Each enabled Identity has at most one enabled Connection. Its Accounts and Platform catalog entries are discovered automatically, while subordinate Identities require explicit enablement. Every reported Balance and Position is retained even before a newly discovered Asset Listing is mapped, and prices are tracked for current holdings, open Positions, and any additional entries pinned by the User within that Portfolio.

The planned frontend presents Balance and Position tables that can be filtered or grouped by Platform, Identity, Account, Asset, and Instrument. Platform-specific Account Metrics such as UniMMR remain visible at Account scope rather than being treated as cross-platform aggregates.

Portfolio Valuation uses Reported Equity for its declared coverage and sufficient Calculated Valuation for uncovered scopes. One editable Capital Baseline and manually entered external Capital Flows support absolute Value Changes for calendar days, ISO weeks, months, quarters, and years, including provisional period-to-date results. Observation times, failures, stale data, incomplete coverage, and incomparable periods remain visible. Display defaults to USD with a CNY option while preserving original quantities and denominations.

Plutus persists normalized Observations rather than raw Platform response payloads. Latest state, aggregates, valuations, exposure, contributed capital, value changes, and period summaries are rebuildable projections; correcting an Observation or manual source fact invalidates and rebuilds affected projections.

Value Changes are not investment returns. Percentage returns, return attribution, a full transaction ledger, trading execution, and internal-transfer tracking are outside the first release; asynchronous observations around an internal transfer can therefore create a temporary discrepancy. Manual asset management, including manually tracked securities or DeFi assets, is deferred to a later product release.

## Roadmap

1. Foundation (done): configuration, explicit MySQL migrations, OpenAPI generation, authentication, multiple Portfolios, an embedded React shell, and a persistent Projection worker.
2. End-to-end vertical slice (done): deterministic source-fact seeds, normalized Observations, generation-based Projections, and working Overview, Balances, Positions, and Capital pages.
3. Complete MVP UI and management (done): Exposure, History, Data, Jobs, Connection and Catalog management, audit, and correction workflows.
4. Integrations: scheduling and real Binance, OKX, and Bybit adapters.

The accepted design uses a contract-first modular Go monolith, a versioned JSON API, an embedded React frontend, and an externally managed MySQL database.

## Documentation

- [Domain glossary](./CONTEXT.md)
- [MVP technical design](./docs/technical-design.md)
- [Architecture decision records](./docs/adr/)
- [Implementation checklist](./docs/implementation-checklist.md)
- [Contributor and agent guidance](./AGENTS.md)

## Development

Use the Go toolchain required by `go.mod`, Node.js as required by `web/package.json`, and Make. Install the pinned formatting, lint, sqlc, Goose, and frontend tools with `make deps`, and ensure the Go tool installation directory is on `PATH`.

Local work uses three isolated lanes. Files live together in [`config/`](./config/). They must not share a MySQL database, HTTP port, env file, or session cookie. Create the databases once (`CREATE DATABASE plutus_local; CREATE DATABASE plutus_agent; CREATE DATABASE plutus_walk; CREATE DATABASE plutus_test;`). Give each lane its own `PLUTUS_MASTER_KEY` (`openssl rand -base64 32`); do not copy keys between files.

- Local development (`127.0.0.1:8080`, database `plutus_local`): copy [config/local-example.toml](./config/local-example.toml) to gitignored `config/local.toml` and [config/local-example.env](./config/local-example.env) to gitignored `config/local.env`, then set `PLUTUS_DATABASE_DSN`. Run `./scripts/local-serve.sh`. This script does not wipe or rebuild the SPA. For Vite, run `npm run dev` in `web/`; it proxies `/api` and `/health` to `:8080`.
- Agent self-check (`127.0.0.1:8081`, database `plutus_agent`): copy [config/agent-example.toml](./config/agent-example.toml) to `config/agent.toml` and [config/agent-example.env](./config/agent-example.env) to `config/agent.env`. Run `./scripts/agent-wipe-and-serve.sh`. Every run rebuilds `web/dist`, resets `plutus_agent` with the walkthrough seed, and serves the embedded SPA. Open `http://127.0.0.1:8081`.
- Human walkthrough (`127.0.0.1:8082`, database `plutus_walk`): copy [config/walk-example.toml](./config/walk-example.toml) to `config/walk.toml` and [config/walk-example.env](./config/walk-example.env) to `config/walk.env`. Run `./scripts/walk-wipe-and-serve.sh`. Same wipe/serve behavior as the agent lane, on `:8082`.

`scripts/dev-wipe-and-serve.sh` is a compatibility alias for the walkthrough script so it no longer wipes the local database. `scripts/dev-serve.sh` is a compatibility alias for `scripts/local-serve.sh`. Pass `--config` and `--env-file` to any `plutus` command to select a lane. When `--env-file` is set, that file is the only env file loaded; it does not merge with `config/local.env`. `PLUTUS_DATABASE_DSN` or `PLUTUS_DATABASE_DSN_FILE` can still override a DSN. The Connection encryption master key stays in `PLUTUS_MASTER_KEY` or `PLUTUS_MASTER_KEY_FILE` and must not appear in TOML. The key is not required until an encrypted Connection exists. Integration tests read `PLUTUS_TEST_DATABASE_DSN` only from the process environment or gitignored `config/test.env` (copy [config/test-example.env](./config/test-example.env)) and must target `plutus_test`, not `plutus_local`, `plutus_agent`, or `plutus_walk`.

`db reset` drops every table in the connected database, reapplies migrations, inserts the chosen source-fact profile, and enqueues a projection rebuild for each Portfolio. It requires `[database].allow_reset = true` in the selected TOML and a database name matching `plutus` or `plutus_<suffix>`. `serve` never migrates or seeds automatically. Use `--profile walkthrough` for the rich local UI dataset (daily history on Binance, OKX, and Bybit, extra assets, incomplete coverage kept off the latest cutoff, plus catalog/audit rows), `--profile minimal` for the small happy path, and `--profile edge-cases` for the focused incomplete-coverage dataset.

Equivalent commands without dropping the database:

```sh
go run ./cmd/plutus --config config/local.toml --env-file config/local.env migrate up
go run ./cmd/plutus --config config/local.toml --env-file config/local.env db seed --profile walkthrough
go run ./cmd/plutus --config config/local.toml --env-file config/local.env serve
```

Scratch logins from every seed profile:

| Username | Password |
| --- | --- |
| `alice` | `Alice=123456` |
| `bob` | `Bob=123456` |

Alice owns Portfolios `default` (USD) and `trading` (CNY). Bob owns `default` in UTC. After `serve` starts, the worker activates generations so Overview, Balances, Positions, Exposure, Valuations, Value Changes, and Capital read live JSON. Full rebuilds are also available from the Jobs page or with `plutus projection rebuild --portfolio <uuid>`.

`scripts/walk-wipe-and-serve.sh` rebuilds `web/dist`, resets `plutus_walk`, and then `serve`s on `:8082`. It prints the seed logins above and does not create a separate `admin` user. Valuations and Value Changes open a 45-day From/To window so the first view is populated. Agent self-check uses `scripts/agent-wipe-and-serve.sh` on `:8081`. Local `scripts/local-serve.sh` does not wipe.

Public registration is not offered. Create Users with `plutus user create`. Passwords must be 8 to 128 characters.

Production `serve` hosts the hashed SPA from `web/dist` via `go:embed`. Refresh that build with `make web-build` after frontend changes. For Vite development, run `npm run dev` in `web/`; it proxies `/api` and `/health` to `127.0.0.1:8080` and sends cookies to that origin.

Local Go development follows four validation gates in order. After `web/` exists, `make format` also runs Prettier and `make lint` also runs the frontend typecheck and ESLint:

1. `make format`: rewrite imports and formatting before validating the code.
2. `make build`: confirm the formatted code compiles and build `bin/plutus`.
3. `make test`: verify unit tests without cached results after the build passes.
4. `make lint`: run the configured linters after tests pass.

Each gate must pass before proceeding to the next. Stop on failure, fix the issue, and repeat the sequence:

```sh
make format && make build && make test && make lint
```

`make style` is intended for future GitHub CI, where it replaces `make format` as the first gate. It checks formatting without rewriting files and fails when formatting corrections are needed; build, tests, and lint follow in the same order:

```sh
make style && make build && make test && make lint
```

Integration tests that need MySQL skip unless `PLUTUS_TEST_DATABASE_DSN` is set in the process environment or `config/test.env` in the working directory or Go module root (process environment wins). `go test` runs with the package directory as the working directory, so a repository-root `config/test.env` is still found. Do not point that variable at a development database you care about.

Use `make help` to list commands in their Makefile order. Run `make run` for local execution through `go run ./cmd/plutus`, without a separate build step. Pass CLI arguments with `ARGS`.
