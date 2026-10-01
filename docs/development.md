# Development

Use this guide for environment setup, runtime configuration, seeds, and the
embedded frontend. [Testing](./testing.md) owns validation procedures;
[deployment](./deployment.md) owns remote installation and upgrades.

## Environments

Plutus runs in three environments
([Isolated environments and populated-database migrations](./adr/0008-environments-and-migrations.md)).
Automated tests are not an environment; they create their own throwaway
databases.

| Environment                          | Where                            | Adapter                                | Port and database                | Data                           |
| ------------------------------------ | -------------------------------- | -------------------------------------- | -------------------------------- | ------------------------------ |
| dev, `local` instance (you)          | your machine, `go run`           | deterministic fake, seeded             | `0.0.0.0:8080`, `plutus_local`   | reset whenever you like        |
| dev, `agent` instance (the AI agent) | your machine, `go run`           | deterministic fake, seeded             | `127.0.0.1:8081`, `plutus_agent` | reset whenever the agent likes |
| test                                 | a remote server, deployed binary | live Adapters with real read-only keys | `127.0.0.1:8082`, `plutus_test`  | reset by hand, never seeded    |
| prod                                 | a remote server, deployed binary | live Adapters with real read-only keys | `127.0.0.1:8083`, `plutus_prod`  | never reset                    |

Each environment has its own `PLUTUS_MASTER_KEY` (`openssl rand -base64 32`),
its own database, and its own session cookie name. Dev instances never hold a
real Platform key; test and prod never register the fake. The test environment
is where collected numbers are compared with the exchange apps by hand; the
`local` instance is where the UI is walked through on fake data.

## Setup

Use the Go toolchain required by [go.mod](../go.mod), Node.js as required by
[web/package.json](../web/package.json), Make, and MySQL 8.4 LTS. Install the
pinned formatting, lint, sqlc, Goose, and frontend tools with `make deps`, and
ensure the Go tool installation directory is on `PATH`.

The two dev instances live in gitignored files under [`config/`](../config).
Create only the database for the instance you are setting up. Every dev example
TOML sets `[collection] fake_adapter = true`, which registers the deterministic
offline Adapter for every offered Platform. With `fake_adapter = false`, all
four live Adapters are registered instead.

### Local setup

For the human's instance, create `plutus_local` once:

```sql
CREATE DATABASE plutus_local;
```

For `local` (`0.0.0.0:8080`, database `plutus_local`), copy
[config/local-example.toml](../config/local-example.toml) to `config/local.toml`
and [config/local-example.env](../config/local-example.env) to
`config/local.env`, then set `PLUTUS_DSN` and `PLUTUS_MASTER_KEY`. `make dev`
runs `./scripts/local-serve.sh`, which rebuilds the SPA and serves the existing
data; `./scripts/local-wipe-and-serve.sh` also resets `plutus_local` with the
walkthrough seed. Other machines on the LAN can open `http://<lan-ip>:8080`.

### Agent setup

Agent sessions may connect only to `plutus_agent` and throwaway
`plutus_gotest_*` databases, never `plutus_local`, `plutus_test`, or
`plutus_prod`. Create the agent database once:

```sql
CREATE DATABASE plutus_agent;
```

For `agent` (`127.0.0.1:8081`, database `plutus_agent`), copy
[config/agent-example.toml](../config/agent-example.toml) to `config/agent.toml`
and [config/agent-example.env](../config/agent-example.env) to
`config/agent.env`, then set `PLUTUS_DSN` and a unique `PLUTUS_MASTER_KEY`.
`./scripts/agent-serve.sh` rebuilds the SPA and keeps the data;
`./scripts/agent-wipe-and-serve.sh` also resets `plutus_agent` with the
walkthrough seed.

## Configuration

Pass `--config` and `--env-file` to any `plutus` command to select an
environment. When `--env-file` is set, that file is the only env file loaded; it
does not merge with `config/local.env`. `PLUTUS_DSN` or `PLUTUS_DSN_FILE` can
still override a DSN. The credential encryption master key stays in
`PLUTUS_MASTER_KEY` or `PLUTUS_MASTER_KEY_FILE` and must not appear in TOML.
Without a key, credential writes, validation, and enabling return 503 and seeded
Identities stay `unconfigured`.

## Seeds and collection

`db reset` drops every table in the connected database and reapplies the
migration. With `--profile` it then inserts the chosen seed, seals and enables
the profile's live Identities through the fake Adapter, and rebuilds every
Valuation Point with the real valuation code; without `--profile` it leaves an
empty migrated database (how the test environment is wiped). It requires
`[database].allow_reset = true` in the selected TOML and a database name
matching `plutus` or `plutus_<suffix>` that does not contain `prod`. `serve`
never migrates or seeds automatically. Use `--profile walkthrough` for the rich
UI dataset (forty days across Binance Portfolio Margin, OKX, and Bybit, twenty
days of a Binance Classic Trading Identity, one rate-limited day, an archived
Identity, Capital Flows), `--profile minimal` for the small happy path, and
`--profile edge-cases` for the stale, missing, unpriced, boundary, gap, and
unvalued-flow cases.

Equivalent commands without dropping the database:

```sh
make web-build
go run ./cmd/plutus --config config/local.toml --env-file config/local.env migrate up
go run ./cmd/plutus --config config/local.toml --env-file config/local.env db seed --profile walkthrough
go run ./cmd/plutus --config config/local.toml --env-file config/local.env serve
go run ./cmd/plutus --config config/local.toml --env-file config/local.env collect --portfolio <uuid>
```

Scratch logins from every seed profile:

| Username | Password       |
| -------- | -------------- |
| `alice`  | `Alice=123456` |
| `bob`    | `Bob=123456`   |

Alice owns Portfolios `default` (Asia/Shanghai, undeletable) and `trading`. Bob
owns `default` in Europe/Berlin. Once `serve` runs, the scheduler keeps
collecting every enabled Identity at the Portfolio interval; the fake Adapter
continues the seeded series, so Overview, Holdings, History, and Capital stay
live. `Collect now` on the Overview page and `plutus collect --portfolio <uuid>`
trigger one cycle immediately.

Public registration is not offered. Create Users with `plutus user create`.
Passwords must be 8 to 128 characters.

## Embedded frontend

`serve` hosts the SPA from `web/dist` via `go:embed`. The directory is generated
and ignored by Git. After installing frontend dependencies with `make deps`,
Make targets that compile Go build the SPA first. Run `make web-build` before
direct `go build`, `go test`, or `go run` commands, and again after frontend
changes. `make dev` runs the complete local service on `:8080` and keeps
existing data. Restart it after frontend changes to rebuild the embedded SPA.

See [testing and verification](./testing.md) for validation gates, temporary
MySQL setup, agent runtime verification, and Adapter delivery reports. For
remote environments and upgrades, see [deployment](./deployment.md). Run
`make help` to list targets, or `make run ARGS=--help` for CLI help.
