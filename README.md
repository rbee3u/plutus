# Plutus

Plutus is a personal, read-only portfolio observation project. It is intended to periodically collect normalized account observations, persist their history, and present balances, positions, valuations, and basic directional exposure through a web frontend.

The domain hierarchy is `Platform -> Identity -> Account`: each main or subordinate identity on a financial Platform has its own funding or trading Accounts. A Connection configures access to one Identity. An authenticated Plutus User is a separate concept, owns one or more independent Portfolios, and directly manages the Identities selected into each Portfolio.

## Current status

Phase 3 (complete MVP UI and management) is implemented and waiting for review. The process is a Go CLI at `cmd/plutus` with an embedded React frontend:

- Configuration from `plutus.toml` and `PLUTUS_*`, including the HTTP / HTTPS / mTLS startup matrix.
- Explicit Goose migrations and sqlc access. `serve` never migrates automatically.
- Session authentication (Argon2id, CSRF, rate limits), Portfolio CRUD, Identity CRUD, and write-only Connection credentials encrypted with AES-256-GCM. `/health/ready` fails when ciphertext exists without a master key.
- Development `plutus db seed` and `plutus db reset` with deterministic `minimal` and `edge-cases` source-fact profiles. Seeds stay secret-free. Reset requires `[database].allow_reset = true` in the gitignored `plutus.toml` and a database name matching `plutus` or `plutus_<suffix>`.
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

Copy [configs/plutus.example.toml](./configs/plutus.example.toml) to a gitignored `plutus.toml` in the working directory and set `[database].dsn`. `PLUTUS_DATABASE_DSN` or `PLUTUS_DATABASE_DSN_FILE` can still override that value. The Connection encryption master key stays in `PLUTUS_MASTER_KEY` or `PLUTUS_MASTER_KEY_FILE` and is not required until an encrypted Connection exists.

For a full local wipe of an allowed development database, also set `[database].allow_reset = true` (the example file defaults to `false`). Then reset, seed, and serve:

```sh
cp configs/plutus.example.toml plutus.toml   # then edit [database].dsn and allow_reset
go run ./cmd/plutus db reset --profile minimal
go run ./cmd/plutus serve
```

`db reset` drops every table in the connected database, reapplies migrations, inserts the chosen source-fact profile, and enqueues a projection rebuild for each Portfolio. `serve` never migrates or seeds automatically. Use `--profile edge-cases` for the incomplete-coverage dataset.

Equivalent commands without dropping the database:

```sh
go run ./cmd/plutus migrate up
go run ./cmd/plutus db seed --profile minimal
go run ./cmd/plutus serve
```

Scratch logins from both seed profiles:

| Username | Password |
| --- | --- |
| `alice` | `Alice=123456` |
| `bob` | `Bob=123456` |

Alice owns Portfolios `Main` and `Trading`. Bob owns `Main`. After `serve` starts, the worker activates generations so Overview, Balances, Positions, Exposure, Valuations, Value Changes, and Capital read live JSON. Full rebuilds are also available from the Jobs page or with `plutus projection rebuild --portfolio <uuid>`.

`scripts/dev-wipe-and-serve.sh` runs `db reset --profile minimal` and then `serve`. It prints the seed logins above and does not create a separate `admin` user.

Public registration defaults to off. The Register page explains that additional Users can be created with `plutus user create`. Passwords must be 8 to 128 characters.

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

Integration tests that need MySQL skip unless `PLUTUS_TEST_DATABASE_DSN` is set. Do not point that variable at a development database you care about.

Use `make help` to list commands in their Makefile order. Run `make run` for local execution through `go run ./cmd/plutus`, without a separate build step. Pass CLI arguments with `ARGS`.
