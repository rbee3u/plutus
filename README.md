# Plutus

Plutus is a personal, read-only portfolio observation tool. It periodically collects the balances, positions, and account reports that configured exchange accounts disclose, values them in USD, and shows how the total changed over time once the owner's own transfers in and out are excluded.

The domain hierarchy is `Platform -> Identity -> Account`: each exchange login (main account or sub-account, configured independently) has the Accounts the User selects from the Account Types its Adapter supports. A Connection holds that Identity's read-only API credentials. A Plutus User owns one or more Portfolios; a Portfolio is one person's ledger.

## Current status

Phases 1–3 are implemented on the **previous** domain model and work end to end with synthetic data. On 2026-09-22 the design was revised (ADRs [0014](./docs/adr/0014-store-one-valuation-point-per-collection-cycle.md)–[0021](./docs/adr/0021-report-each-account-with-one-aligned-account-report.md)); Phase 4 restructures the code to the revised model before any real exchange integration. Until Phase 4 lands, the running code still has the old shape:

- Configuration from TOML and `PLUTUS_*`, `--config` / `--env-file` lanes, the HTTP / HTTPS / mTLS startup matrix.
- Explicit Goose migrations and sqlc access. `serve` never migrates automatically.
- Session authentication (Argon2id, CSRF, rate limits), Portfolio CRUD, Identity CRUD, write-only Connection credentials encrypted with AES-256-GCM.
- Development `plutus db seed` and `plutus db reset` with deterministic `minimal`, `edge-cases`, and `walkthrough` profiles.
- Old-model Observations, catalog mapping and pin revisions, Observation correction, Capital Baseline / Flow revisions, audit events, and a generation-based projection worker with `POST /projection-rebuilds`.
- Working Overview, Balances, Positions, Exposure, Valuations, Value Changes, Capital, Identity, Connection, Catalog, Observation, Jobs, and Audit pages.

Not implemented: the revised schema and modules, real Platform adapters, the Collection Cycle scheduler, Connection validation against a live Platform. Progress is tracked in the [implementation checklist](./docs/implementation-checklist.md).

## Product scope

Two questions, in priority order:

1. **How much is there now, and where.** A Platform → Identity → Account tree with each Account's report fields (equity, adjusted equity, available equity, margin ratios), its Balances and Positions, and USD totals per Account, Platform, and Portfolio. The numbers are meant to match each exchange app.
2. **How did the total change over a period, excluding my own transfers.** One Valuation Point per Collection Cycle (default every 15 minutes), Period Closes, and absolute USD Value Change by calendar day, ISO week, month, quarter, and year, after subtracting manual Capital Flows and derived Boundary Events.

Initial integrations target these Account Modes:

| Platform | Account modes |
| --- | --- |
| Binance | Portfolio Margin (PM) |
| OKX | Multi-currency margin (MCM), Portfolio margin (PM) |
| Bybit | Unified Trading Account 2.0 (UTA 2.0) |

Rules that shape the product:

- The User selects which Account Types to collect for each Identity; nothing is discovered or probed beyond that selection.
- An Account's value is the Platform's reported equity when it provides one, otherwise the sum of its Balances times the same Platform's prices. USD is the only valuation currency; USDT and USDC count as 1 USD.
- A failed Account collection keeps its last successful report, labeled stale with its age. A successful report is the complete set: a coin not listed is no longer held. A Balance without a price is shown but left out of the total, which is marked incomplete.
- Adding or removing an Account from the Portfolio is a Boundary Event treated as an implicit Capital Flow, so Value Change does not jump. Capital Flows are recorded manually; Plutus only ever needs read-only API keys.
- Value Change is not an investment return. Percentage returns, attribution, trade history, automatic deposit import, CNY display, and Exposure aggregation are out of scope.

## Roadmap

1. Foundation (done): configuration, explicit MySQL migrations, OpenAPI generation, authentication, multiple Portfolios, embedded React shell, durable job worker.
2. End-to-end vertical slice (done, previous model): deterministic seeds, Observations, projections, Overview / Balances / Positions / Capital pages.
3. Complete MVP UI and management (done, previous model): Exposure, History, Data, Jobs, Connection and Catalog management, audit, and correction workflows.
4. Restructure to the accepted model: rewrite the baseline migration, replace projections with write-once Valuation Points, Account selection, Boundary Events, fake Adapter, new Holdings / History / Capital / Identities pages.
5. Integrations: Binance, OKX, and Bybit Adapters and live validation of the collection rules.

The accepted architecture is a contract-first modular Go monolith, a versioned JSON API, an embedded React frontend, and an externally managed MySQL database.

## Documentation

- [Domain glossary](./CONTEXT.md)
- [Technical design](./docs/technical-design.md)
- [Architecture decision records](./docs/adr/)
- [Implementation checklist](./docs/implementation-checklist.md)
- [Contributor and agent guidance](./AGENTS.md)

## Development

The commands and seed behavior below describe the code as it runs today (previous model). Phase 4 replaces the projection worker, `plutus projection rebuild`, and the CNY Portfolio option; the checklist tracks that switch.

Use the Go toolchain required by `go.mod`, Node.js as required by `web/package.json`, and Make. Install the pinned formatting, lint, sqlc, Goose, and frontend tools with `make deps`, and ensure the Go tool installation directory is on `PATH`.

Local work uses three isolated lanes. Files live together in [`config/`](./config/). They must not share a MySQL database, HTTP port, env file, or session cookie. Create the databases once (`CREATE DATABASE plutus_local; CREATE DATABASE plutus_agent; CREATE DATABASE plutus_walk; CREATE DATABASE plutus_test;`). Give each lane its own `PLUTUS_MASTER_KEY` (`openssl rand -base64 32`); do not copy keys between files.

- Local development (`0.0.0.0:8080`, database `plutus_local`): copy [config/local-example.toml](./config/local-example.toml) to gitignored `config/local.toml` and [config/local-example.env](./config/local-example.env) to gitignored `config/local.env`, then set `PLUTUS_DATABASE_DSN`. Run `./scripts/local-serve.sh` to keep existing data. This script does not wipe or rebuild the SPA. Run `./scripts/local-wipe-and-serve.sh` to rebuild `web/dist`, reset `plutus_local` with the walkthrough seed, and serve. Other machines on the LAN can open `http://<lan-ip>:8080`. For Vite, run `npm run dev` in `web/`; it proxies `/api` and `/health` to `:8080`.
- Agent self-check (`127.0.0.1:8081`, database `plutus_agent`): copy [config/agent-example.toml](./config/agent-example.toml) to `config/agent.toml` and [config/agent-example.env](./config/agent-example.env) to `config/agent.env`. Run `./scripts/agent-wipe-and-serve.sh`. Every run rebuilds `web/dist`, resets `plutus_agent` with the walkthrough seed, and serves the embedded SPA. Open `http://127.0.0.1:8081`.
- Human walkthrough (`127.0.0.1:8082`, database `plutus_walk`): copy [config/walk-example.toml](./config/walk-example.toml) to `config/walk.toml` and [config/walk-example.env](./config/walk-example.env) to `config/walk.env`. Run `./scripts/walk-wipe-and-serve.sh`. Same wipe/serve behavior as the agent lane, on `:8082`.

Pass `--config` and `--env-file` to any `plutus` command to select a lane. When `--env-file` is set, that file is the only env file loaded; it does not merge with `config/local.env`. `PLUTUS_DATABASE_DSN` or `PLUTUS_DATABASE_DSN_FILE` can still override a DSN. The Connection encryption master key stays in `PLUTUS_MASTER_KEY` or `PLUTUS_MASTER_KEY_FILE` and must not appear in TOML. The key is not required until an encrypted Connection exists. Integration tests read `PLUTUS_TEST_DATABASE_DSN` only from the process environment or gitignored `config/test.env` (copy [config/test-example.env](./config/test-example.env)) and must target `plutus_test`, not `plutus_local`, `plutus_agent`, or `plutus_walk`.

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

`scripts/walk-wipe-and-serve.sh` rebuilds `web/dist`, resets `plutus_walk`, and then `serve`s on `:8082`. It prints the seed logins above and does not create a separate `admin` user. Valuations and Value Changes open a 45-day From/To window so the first view is populated. Agent self-check uses `scripts/agent-wipe-and-serve.sh` on `:8081`. Local `scripts/local-wipe-and-serve.sh` does the same for `plutus_local` on `:8080`; `scripts/local-serve.sh` does not wipe.

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
