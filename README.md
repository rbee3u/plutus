# Plutus

Plutus is a personal, read-only portfolio observation tool. It periodically collects the balances, positions, and account reports that configured exchange accounts disclose, values them in USD, and shows how the total changed over time once the owner's own transfers in and out are excluded.

The domain hierarchy is `Platform -> Identity -> Account`: each exchange login (main account or sub-account, configured independently) has the Accounts the User selects from the Account Types its Adapter supports. A Connection holds that Identity's read-only API credentials. A Plutus User owns one or more Portfolios; a Portfolio is one person's ledger.

## Current status

Phases 1–4 are implemented. The code runs the accepted model (ADRs [0014](./docs/adr/0014-store-one-valuation-point-per-collection-cycle.md)–[0021](./docs/adr/0021-report-each-account-with-one-aligned-account-report.md)) end to end against a deterministic **fake Adapter**; no live exchange Adapter exists yet.

- Configuration from TOML and `PLUTUS_*`, `--config` / `--env-file` lanes, the HTTP / HTTPS / mTLS startup matrix.
- One explicit Goose baseline migration (16 tables) and sqlc access. `serve` never migrates automatically.
- Session authentication (Argon2id, CSRF, rate limits), Portfolio CRUD with a per-Portfolio collection interval and timezone.
- Identities with User-selected Accounts, write-only credentials encrypted with AES-256-GCM, and a validate → enable → disable → archive Connection flow.
- A scheduler and durable job worker that run one Collection Cycle per Portfolio at its interval (or on demand), write one aligned Account Report per selected Account, and store one Valuation Point per cycle with stale carry-forward, incomplete marking, and derived Boundary Events.
- Read-only history: Valuation Point series, Period Closes, Value Changes by day / ISO week / month / quarter / year or a custom range, manual Capital Flows, Boundary Events.
- Shared catalog of Asset Listings (symbol mapping with manual override and recalculation) and Instruments; Platform prices.
- Overview, Holdings (tree, balances, positions), History, Capital, Identities, Catalog, Activity (cycles, jobs, audit), and settings pages in the embedded React app.
- Development `plutus db seed` / `db reset` with deterministic `minimal`, `edge-cases`, and `walkthrough` profiles generated from the same fake scenario the runtime Adapter uses, and `plutus collect` to run one cycle from the CLI.

Not implemented: the live Binance, OKX, and Bybit Adapters (Phase 5). Progress is tracked in the [implementation checklist](./docs/implementation-checklist.md).

## Product scope

Two questions, in priority order:

1. **How much is there now, and where.** A Platform → Identity → Account tree with each Account's report fields (equity, adjusted equity, available equity, margin ratios), its Balances and Positions, and USD totals per Account, Platform, and Portfolio. The numbers are meant to match each exchange app.
2. **How did the total change over a period, excluding my own transfers.** One Valuation Point per Collection Cycle (default every 15 minutes), Period Closes, and absolute USD Value Change by calendar day, ISO week, month, quarter, and year, after subtracting manual Capital Flows and derived Boundary Events.

Integrations target these Account Modes:

| Platform | Account modes | Account Types |
| --- | --- | --- |
| Binance | Portfolio Margin (PM) | portfolio margin, spot, funding, earn |
| OKX | Multi-currency margin (MCM), Portfolio margin (PM) | trading, funding, earn |
| Bybit | Unified Trading Account 2.0 (UTA 2.0) | unified, funding, earn |

Rules that shape the product:

- The User selects which Account Types to collect for each Identity; nothing is discovered or probed beyond that selection.
- An Account's value is the Platform's reported equity when it provides one, otherwise the sum of its Balances times the same Platform's prices. USD is the only valuation currency; USDT and USDC count as 1 USD.
- A failed Account collection keeps its last successful report, labeled stale with its age. A successful report is the complete set: a coin not listed is no longer held. A Balance without a price is shown but left out of the total, which is marked incomplete.
- Adding or removing an Account from the Portfolio is a Boundary Event treated as an implicit Capital Flow, so Value Change does not jump. Capital Flows are recorded manually; Plutus only ever needs read-only API keys.
- Value Change is not an investment return. Percentage returns, attribution, trade history, automatic deposit import, non-USD display, and Exposure aggregation are out of scope.

## Roadmap

1. Foundation (done): configuration, explicit MySQL migrations, OpenAPI generation, authentication, multiple Portfolios, embedded React shell, durable job worker.
2. End-to-end vertical slice (done, previous model): deterministic seeds, Observations, projections, first pages.
3. Complete MVP UI and management (done, previous model): history, management, audit, and correction workflows.
4. Restructure to the accepted model (done): the 16-table schema, write-once Valuation Points, Account selection, Boundary Events, the fake Adapter, and the Holdings / History / Capital / Identities / Catalog / Activity pages.
5. Integrations: Binance, OKX, and Bybit Adapters and live validation of the collection rules.

The accepted architecture is a contract-first modular Go monolith, a versioned JSON API, an embedded React frontend, and an externally managed MySQL database.

## Documentation

- [Domain glossary](./CONTEXT.md)
- [Technical design](./docs/technical-design.md)
- [Architecture decision records](./docs/adr/)
- [Implementation checklist](./docs/implementation-checklist.md)
- [Contributor and agent guidance](./AGENTS.md)

## Development

Use the Go toolchain required by `go.mod`, Node.js as required by `web/package.json`, and Make. Install the pinned formatting, lint, sqlc, Goose, and frontend tools with `make deps`, and ensure the Go tool installation directory is on `PATH`.

Local work uses three isolated lanes. Files live together in [`config/`](./config/). They must not share a MySQL database, HTTP port, env file, or session cookie. Create the databases once (`CREATE DATABASE plutus_local; CREATE DATABASE plutus_agent; CREATE DATABASE plutus_walk; CREATE DATABASE plutus_test;`). Give each lane its own `PLUTUS_MASTER_KEY` (`openssl rand -base64 32`); do not copy keys between files. Every example TOML sets `[collection] fake_adapter = true`, which registers the deterministic offline Adapter for Binance, OKX, and Bybit.

- Local development (`0.0.0.0:8080`, database `plutus_local`): copy [config/local-example.toml](./config/local-example.toml) to gitignored `config/local.toml` and [config/local-example.env](./config/local-example.env) to gitignored `config/local.env`, then set `PLUTUS_DATABASE_DSN` and `PLUTUS_MASTER_KEY`. Run `./scripts/local-serve.sh` to keep existing data. This script does not wipe or rebuild the SPA. Run `./scripts/local-wipe-and-serve.sh` to rebuild `web/dist`, reset `plutus_local` with the walkthrough seed, and serve. Other machines on the LAN can open `http://<lan-ip>:8080`. For Vite, run `npm run dev` in `web/`; it proxies `/api` and `/health` to `:8080`.
- Agent self-check (`127.0.0.1:8081`, database `plutus_agent`): copy [config/agent-example.toml](./config/agent-example.toml) to `config/agent.toml` and [config/agent-example.env](./config/agent-example.env) to `config/agent.env`. Run `./scripts/agent-wipe-and-serve.sh`. Every run rebuilds `web/dist`, resets `plutus_agent` with the walkthrough seed, and serves the embedded SPA. Open `http://127.0.0.1:8081`.
- Human walkthrough (`127.0.0.1:8082`, database `plutus_walk`): copy [config/walk-example.toml](./config/walk-example.toml) to `config/walk.toml` and [config/walk-example.env](./config/walk-example.env) to `config/walk.env`. Run `./scripts/walk-wipe-and-serve.sh`. Same wipe/serve behavior as the agent lane, on `:8082`.

Pass `--config` and `--env-file` to any `plutus` command to select a lane. When `--env-file` is set, that file is the only env file loaded; it does not merge with `config/local.env`. `PLUTUS_DATABASE_DSN` or `PLUTUS_DATABASE_DSN_FILE` can still override a DSN. The credential encryption master key stays in `PLUTUS_MASTER_KEY` or `PLUTUS_MASTER_KEY_FILE` and must not appear in TOML. Without a key, credential writes, validation, and enabling return 503 and seeded Identities stay `unconfigured`. Integration tests read `PLUTUS_TEST_DATABASE_DSN` only from the process environment or gitignored `config/test.env` (copy [config/test-example.env](./config/test-example.env)) and must target `plutus_test`, not `plutus_local`, `plutus_agent`, or `plutus_walk`; the seed test additionally creates and drops a scratch `plutus_seedcheck` database on the same server.

`db reset` drops every table in the connected database, reapplies the migration, inserts the chosen profile, seals and enables the profile's live Identities through the fake Adapter, and rebuilds every Valuation Point with the real valuation code. It requires `[database].allow_reset = true` in the selected TOML and a database name matching `plutus` or `plutus_<suffix>`. `serve` never migrates or seeds automatically. Use `--profile walkthrough` for the rich local UI dataset (forty days across Binance, OKX, and Bybit, one rate-limited day, an archived Identity, Capital Flows), `--profile minimal` for the small happy path, and `--profile edge-cases` for the stale, missing, unpriced, boundary, gap, and unvalued-flow cases.

Equivalent commands without dropping the database:

```sh
go run ./cmd/plutus --config config/local.toml --env-file config/local.env migrate up
go run ./cmd/plutus --config config/local.toml --env-file config/local.env db seed --profile walkthrough
go run ./cmd/plutus --config config/local.toml --env-file config/local.env serve
go run ./cmd/plutus --config config/local.toml --env-file config/local.env collect --portfolio <uuid>
```

Scratch logins from every seed profile:

| Username | Password |
| --- | --- |
| `alice` | `Alice=123456` |
| `bob` | `Bob=123456` |

Alice owns Portfolios `default` (Asia/Shanghai, undeletable) and `trading`. Bob owns `default` in Europe/Berlin. Once `serve` runs, the scheduler keeps collecting every enabled Identity at the Portfolio interval; the fake Adapter continues the seeded series, so Overview, Holdings, History, and Capital stay live. `Collect now` on the Overview page and `plutus collect --portfolio <uuid>` trigger one cycle immediately.

Public registration is not offered. Create Users with `plutus user create`. Passwords must be 8 to 128 characters.

Production `serve` hosts the hashed SPA from `web/dist` via `go:embed`. Refresh that build with `make web-build` after frontend changes. For Vite development, run `npm run dev` in `web/`; it proxies `/api` and `/health` to `127.0.0.1:8080` and sends cookies to that origin.

Local Go development follows four validation gates in order. `make format` also runs Prettier and `make lint` also runs the frontend typecheck and ESLint:

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

`make generate` regenerates the sqlc queries, the Go strict server, and the TypeScript client; the tree must be unchanged afterwards.

Integration tests that need MySQL skip unless `PLUTUS_TEST_DATABASE_DSN` is set in the process environment or `config/test.env` in the working directory or Go module root (process environment wins). `go test` runs with the package directory as the working directory, so a repository-root `config/test.env` is still found. Do not point that variable at a development database you care about.

Use `make help` to list commands in their Makefile order. Run `make run` for local execution through `go run ./cmd/plutus`, without a separate build step. Pass CLI arguments with `ARGS`.
