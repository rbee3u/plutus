# Plutus

Plutus is a personal, read-only portfolio observation tool. It periodically collects the balances, positions, and account reports that configured exchange accounts disclose, values them in USD, and shows how the total changed over time once the owner's own transfers in and out are excluded.

The domain hierarchy is `Platform -> Identity -> Account`: each exchange login (main account or sub-account, configured independently) has the Accounts the User selects from the Account Types its Adapter supports. A Connection holds that Identity's read-only API credentials. A Plutus User owns one or more Portfolios; a Portfolio is one person's ledger.

## Current status

Phases 1–4 are implemented. The code runs the accepted model (ADRs [0014](./docs/adr/0014-store-one-valuation-point-per-collection-cycle.md)–[0021](./docs/adr/0021-report-each-account-with-one-aligned-account-report.md)) end to end against a deterministic **fake Adapter**. Phase 5 has live Binance, OKX, Bybit, and Kraken Adapters that still await validation against the exchange apps.

- Configuration from TOML and `PLUTUS_*`, `--config` / `--env-file` lanes, the HTTP / HTTPS / mTLS startup matrix.
- One explicit Goose baseline migration (16 tables) and sqlc access. `serve` never migrates automatically.
- Session authentication (Argon2id, CSRF, rate limits), Portfolio CRUD with a per-Portfolio collection interval and timezone.
- Identities with User-selected Accounts, write-only credentials encrypted with AES-256-GCM, and a validate → enable → disable → archive Connection flow. Each Identity declares its Account Structure at creation, which fixes its Account Types; the Margin Mode is read from the Platform at validation and in every Collection Run, shown on the Identity, and audited when it changes ([ADR 0028](./docs/adr/0028-separate-account-structure-from-margin-mode.md)). Validation refuses a key that reaches a different Platform account than the one the Identity is bound to.
- A scheduler and durable job worker that run one Collection Cycle per Portfolio at its interval (or on demand), write one aligned Account Report per selected Account, and store one Valuation Point per cycle with stale carry-forward, incomplete marking, and derived Boundary Events.
- Read-only history: Valuation Point series, Period Closes, Value Changes by day / ISO week / month / quarter / year or a custom range, manual Capital Flows, Boundary Events.
- Shared catalog of Asset Listings (symbol mapping with manual override and recalculation) and Instruments; Platform prices.
- Overview, Holdings (tree, balances, positions), History, Capital, Identities, Catalog, Activity (cycles, jobs, audit), and settings pages in the embedded React app.
- Development `plutus db seed` / `db reset` with deterministic `minimal`, `edge-cases`, and `walkthrough` profiles generated from the same fake scenario the runtime Adapter uses, and `plutus collect` to run one cycle from the CLI.

Every live Adapter is registered when `[collection] fake_adapter = false` and tested in three layers: recorded-fixture replay, documentation-derived fixtures for configurations no test key reaches ([ADR 0027](./docs/adr/0027-allow-documented-adapter-scenarios-without-live-coverage.md)), and a `live` smoke test; `plutus adapter record` records the fixtures. Earn on Binance, OKX, and Bybit includes dual investment and fails as `unsupported` when the Platform values Earn above what Plutus can itemize ([ADR 0029](./docs/adr/0029-count-dual-investment-in-earn-and-check-earn-completeness.md)). No Adapter has been validated against its exchange app on the test environment yet, so no integration is complete.

- Binance: Classic Trading (spot, funding, cross margin, isolated margin, USDⓈ-M futures, COIN-M futures, earn) and Portfolio Margin (portfolio margin, spot, funding, earn). Only Portfolio Margin has run against a real account. See [Binance mappings and coverage](./docs/adapters/binance.md).
- OKX: the unified account in Spot mode, Futures mode, Multi-currency margin, and Portfolio margin (trading, funding, earn), with a live check against OKX's own valuation of each Account. Only Portfolio margin has run against a real account. See [OKX mappings and coverage](./docs/adapters/okx.md).
- Bybit: global UTA 2.0 in Isolated Margin, Cross Margin, and Portfolio Margin (unified, funding, earn). Only Portfolio Margin has run against a real account. See [Bybit mappings and coverage](./docs/adapters/bybit.md).
- Kraken: Separate wallets and Unified Wallet with two credential groups and the structure read from a bounded WebSocket snapshot. Only Unified Wallet has run against a real account. See [Kraken mappings and coverage](./docs/adapters/kraken.md).

Remaining: exchange-app validation on the test environment and the unexercised structures, Margin Modes, and products listed in each adapter's coverage notes. Progress is tracked in the [implementation checklist](./docs/implementation-checklist.md).

## Product scope

Two questions, in priority order:

1. **How much is there now, and where.** A Platform → Identity → Account tree with each Account's report fields (equity, adjusted equity, available equity, margin ratios), its Balances and Positions, and USD totals per Account, Platform, and Portfolio. The numbers are meant to match each exchange app.
2. **How did the total change over a period, excluding my own transfers.** One Valuation Point per Collection Cycle (default every 15 minutes), Period Closes, and absolute USD Value Change by calendar day, ISO week, month, quarter, and year, after subtracting manual Capital Flows and derived Boundary Events.

Integrations target these Account Structures and Margin Modes ([ADR 0028](./docs/adr/0028-separate-account-structure-from-margin-mode.md)):

| Platform | Account Structure (declared) | Margin Modes (observed) | Account Types |
| --- | --- | --- | --- |
| Binance | Classic Trading | none | spot, funding, cross margin, isolated margin, USDⓈ-M futures, COIN-M futures, earn |
| Binance | Portfolio Margin | none | portfolio margin, spot, funding, earn |
| OKX | Unified account | Spot mode, Futures mode, Multi-currency margin, Portfolio margin | trading, funding, earn |
| Bybit | Unified Trading Account 2.0 | Isolated Margin, Cross Margin, Portfolio Margin | unified, funding, earn |
| Kraken | Separate wallets | none | main, earn, multi-m, coin-m, derivatives holding |
| Kraken | Unified Wallet | none | main, earn, coin-m, derivatives holding |

Rules that shape the product:

- The User selects which Account Types to collect for each Identity; nothing is discovered or probed beyond that selection. The Account Structure cannot change: after switching it on the Platform, create a new Identity and archive the old one.
- An Account's value is the Platform's reported equity when it provides one, otherwise the sum of its Balances times the same Platform's prices. USD is the only valuation currency; USDT and USDC count as 1 USD.
- A failed Account collection keeps its last successful report, labeled stale with its age. A successful report is the complete set: a coin not listed is no longer held. A Balance without a price is shown but left out of the total, which is marked incomplete.
- Adding or removing an Account from the Portfolio is a Boundary Event treated as an implicit Capital Flow, so Value Change does not jump. Capital Flows are recorded manually; Plutus only ever needs read-only API keys.
- Value Change is not an investment return. Percentage returns, attribution, trade history, automatic deposit import, non-USD display, and Exposure aggregation are out of scope.

## Roadmap

1. Foundation (done): configuration, explicit MySQL migrations, OpenAPI generation, authentication, multiple Portfolios, embedded React shell, durable job worker.
2. End-to-end vertical slice (done, previous model): deterministic seeds, Observations, projections, first pages.
3. Complete MVP UI and management (done, previous model): history, management, audit, and correction workflows.
4. Restructure to the accepted model (done): the 16-table schema, write-once Valuation Points, Account selection, Boundary Events, the fake Adapter, and the Holdings / History / Capital / Identities / Catalog / Activity pages.
5. Integrations: Binance, OKX, Bybit, and then Kraken Adapters, each a separate delivery, with live validation of the collection rules on the test environment.

The accepted architecture is a contract-first modular Go monolith, a versioned JSON API, an embedded React frontend, and an externally managed MySQL database.

## Documentation

- [Domain glossary](./CONTEXT.md)
- [Technical design](./docs/technical-design.md)
- [Architecture decision records](./docs/adr/)
- [Implementation checklist](./docs/implementation-checklist.md)
- [Contributor and agent guidance](./AGENTS.md)

## Environments

Plutus runs in three environments ([ADR 0022](./docs/adr/0022-separate-dev-test-and-prod-environments.md)). Automated tests are not an environment; they create their own throwaway databases.

| Environment | Where | Adapter | Port and database | Data |
| --- | --- | --- | --- | --- |
| dev, `local` instance (you) | your machine, `go run` | deterministic fake, seeded | `0.0.0.0:8080`, `plutus_local` | reset whenever you like |
| dev, `agent` instance (the AI agent) | your machine, `go run` | deterministic fake, seeded | `127.0.0.1:8081`, `plutus_agent` | reset whenever the agent likes |
| test | a remote server, deployed binary | live Adapters with real read-only keys | `127.0.0.1:8082`, `plutus_test` | reset by hand, never seeded |
| prod | a remote server, deployed binary | live Adapters with real read-only keys | `127.0.0.1:8083`, `plutus_prod` | never reset |

Each environment has its own `PLUTUS_MASTER_KEY` (`openssl rand -base64 32`), its own database, and its own session cookie name. Dev instances never hold a real key; test and prod never register the fake. The test environment is where collected numbers are compared with the exchange apps by hand; the `local` instance is where the UI is walked through on fake data.

## Development

Use the Go toolchain required by `go.mod`, Node.js as required by `web/package.json`, and Make. Install the pinned formatting, lint, sqlc, Goose, and frontend tools with `make deps`, and ensure the Go tool installation directory is on `PATH`.

The two dev instances live in gitignored files under [`config/`](./config/). Create the databases once (`CREATE DATABASE plutus_local; CREATE DATABASE plutus_agent;`). Every dev example TOML sets `[collection] fake_adapter = true`, which registers the deterministic offline Adapter for every offered Platform.

- `local` (`0.0.0.0:8080`, database `plutus_local`): copy [config/local-example.toml](./config/local-example.toml) to `config/local.toml` and [config/local-example.env](./config/local-example.env) to `config/local.env`, then set `PLUTUS_DATABASE_DSN` and `PLUTUS_MASTER_KEY`. `make dev` runs `./scripts/local-serve.sh`, which rebuilds the SPA and serves the existing data; `./scripts/local-wipe-and-serve.sh` also resets `plutus_local` with the walkthrough seed. Other machines on the LAN can open `http://<lan-ip>:8080`.
- `agent` (`127.0.0.1:8081`, database `plutus_agent`): copy [config/agent-example.toml](./config/agent-example.toml) to `config/agent.toml` and [config/agent-example.env](./config/agent-example.env) to `config/agent.env`. `./scripts/agent-serve.sh` rebuilds the SPA and keeps the data; `./scripts/agent-wipe-and-serve.sh` also resets `plutus_agent` with the walkthrough seed.

Pass `--config` and `--env-file` to any `plutus` command to select an environment. When `--env-file` is set, that file is the only env file loaded; it does not merge with `config/local.env`. `PLUTUS_DATABASE_DSN` or `PLUTUS_DATABASE_DSN_FILE` can still override a DSN. The credential encryption master key stays in `PLUTUS_MASTER_KEY` or `PLUTUS_MASTER_KEY_FILE` and must not appear in TOML. Without a key, credential writes, validation, and enabling return 503 and seeded Identities stay `unconfigured`.

`db reset` drops every table in the connected database and reapplies the migration. With `--profile` it then inserts the chosen seed, seals and enables the profile's live Identities through the fake Adapter, and rebuilds every Valuation Point with the real valuation code; without `--profile` it leaves an empty migrated database (how the test environment is wiped). It requires `[database].allow_reset = true` in the selected TOML and a database name matching `plutus` or `plutus_<suffix>` that does not contain `prod`. `serve` never migrates or seeds automatically. Use `--profile walkthrough` for the rich UI dataset (forty days across Binance Portfolio Margin, OKX, and Bybit, twenty days of a Binance Classic Trading Identity, one rate-limited day, an archived Identity, Capital Flows), `--profile minimal` for the small happy path, and `--profile edge-cases` for the stale, missing, unpriced, boundary, gap, and unvalued-flow cases.

Equivalent commands without dropping the database:

```sh
make web-build
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

`serve` hosts the SPA from `web/dist` via `go:embed`. The directory is generated and ignored by Git. After installing frontend dependencies with `make deps`, Make targets that compile Go build the SPA first. Run `make web-build` before direct `go build`, `go test`, or `go run` commands, and again after frontend changes. `make dev` runs the complete local service on `:8080` and keeps existing data. Restart it after frontend changes to rebuild the embedded SPA.

### Validation gates

Local Go development follows four validation gates in order. `make format` also runs Prettier and `make lint` also runs the frontend typecheck and ESLint:

1. `make format`: rewrite imports and formatting before validating the code.
2. `make build`: build the SPA, confirm the formatted code compiles, and build `bin/plutus` with the git version baked in (`plutus --version`).
3. `make test`: run Go and frontend Vitest unit tests without cached results after the build passes. This gate is offline: no live keys, no network.
4. `make lint`: run the configured linters after tests pass.

Each gate must pass before proceeding to the next. Stop on failure, fix the issue, and repeat the sequence:

```sh
make format && make build && make test && make lint
```

`make style` is intended for future GitHub CI, where it replaces `make format` as the first gate. It checks formatting without rewriting files and fails when formatting corrections are needed; build, tests, and lint follow in the same order:

```sh
make style && make build && make test && make lint
```

The optional Chromium check uses a loopback-only Vite server and mocked API responses, so it needs no database. It covers mobile navigation behavior that DOM-only checks cannot verify. Install the bundled browser once, then run:

```sh
npm --prefix web exec playwright install chromium
npm --prefix web run test:chromium
```

It is separate from `make test`, which remains the routine offline gate.

`make generate` regenerates the sqlc queries, the Go strict server, and the TypeScript client; the tree must be unchanged afterwards.

### Tests and `config/gotest.env`

Every variable that `go test` reads starts with `PLUTUS_GOTEST_` and lives in gitignored `config/gotest.env` (copy [config/gotest-example.env](./config/gotest-example.env)) or the process environment, which wins. Outside tests, only `plutus adapter record` reads it (for the Platform keys); `serve`, the worker, and `collect` never do, and tests never read `config/local.env`.

- `PLUTUS_GOTEST_DSN` is a server-level MySQL DSN with no database name (`user:password@tcp(127.0.0.1:3306)/`). Each test package that needs MySQL creates its own `plutus_gotest_<unix>_<hex>` database in `TestMain`, migrates it, runs, and drops it. Nothing has to be created by hand, editing the migration never leaves stale tables behind, and packages run in parallel without sharing state. Databases older than an hour, left behind by a killed run, are dropped on the next run. Without the variable those tests skip and `make test` still passes.
- Platform Adapters are tested in three layers ([ADR 0023](./docs/adr/0023-test-platform-adapters-in-three-layers.md)): `httptest` replay of recorded fixtures in `internal/adapters/<platform>/testdata/` and the deterministic fake, both inside `make test`; and live smoke tests behind the `live` build tag, run only by `make test-live`. Live tests read `PLUTUS_GOTEST_BINANCE_APIKEY` / `_SECRET`, `PLUTUS_GOTEST_OKX_APIKEY` / `_SECRET` / `_PASSPHRASE`, `PLUTUS_GOTEST_BYBIT_APIKEY` / `_SECRET`, and `PLUTUS_GOTEST_KRAKEN_SPOT_APIKEY` / `_SPOT_SECRET` / `_FUTURES_APIKEY` / `_FUTURES_SECRET`, skip a Platform whose variables are missing, detect the account's Account Structure and Margin Mode instead of assuming them, and check structure rather than numbers. These are the same read-only keys the test environment uses; prod's keys never leave the server.
- Fixtures are recorded with `go run ./cmd/plutus adapter record --platform binance` (or `okx`, `bybit`, `kraken`) into gitignored `tmp/fixtures/<platform>/`, trimmed to a few symbols per list endpoint, redacted, and reviewed before they are moved into `testdata/` and committed. It reads the keys from `config/gotest.env` and needs no config file or database.

## Deployment

The test and prod environments run on a remote server from a binary built here. Nothing is automated on purpose: every step is a command you read before you run it.

1. Build a static Linux binary with the git version embedded (override the target with `DIST_GOOS` / `DIST_GOARCH`):

   ```sh
   make dist            # builds web/dist, then dist/plutus-linux-amd64
   ```

2. Copy `dist/plutus-linux-amd64` to the server along with [config/test-example.toml](./config/test-example.toml) and [config/test-example.env](./config/test-example.env) (or the `prod-example` pair). Rename them `test.toml` / `test.env` (or `prod.toml` / `prod.env`) and edit them by hand: DSN, a fresh master key, listen address, TLS. Back up the prod master key somewhere else; without it the stored Platform credentials are unreadable.

3. Create the database once on the server's MySQL (`CREATE DATABASE plutus_test;` or `CREATE DATABASE plutus_prod;`), then, on the server:

   ```sh
   ./plutus-linux-amd64 --config test.toml --env-file test.env migrate up      # first run and after every new migration
   ./plutus-linux-amd64 --config test.toml --env-file test.env user create
   ./plutus-linux-amd64 --config test.toml --env-file test.env serve
   ./plutus-linux-amd64 --version                                                 # which build is running
   ```

   Sign in, create Identities with real read-only keys, select their Accounts, and enable them; the scheduler collects at each Portfolio's interval. Run `serve` under whatever supervisor the server uses (systemd, tmux, nohup); the repository ships no unit file.

4. The test environment can be wiped and re-collected from scratch at any time:

   ```sh
   ./plutus-linux-amd64 --config test.toml --env-file test.env db reset          # drop, migrate, no seed
   ```

   prod cannot: `prod.toml` keeps `allow_reset = false`, and `db reset` refuses any database whose name contains `prod`. Once prod has run `migrate up` for the first time, schema changes only arrive as appended migrations ([ADR 0024](./docs/adr/0024-freeze-the-baseline-migration-once-prod-has-migrated.md)).

Use `make help` to list commands in their Makefile order. Run `make run` for local execution through `go run ./cmd/plutus`, without a separate build step. Pass CLI arguments with `ARGS`.
