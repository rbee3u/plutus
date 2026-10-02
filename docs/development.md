# Development

Use this guide for environment setup, runtime configuration, seeds, and the
embedded frontend. [Testing](./testing.md) owns validation procedures;
[deployment](./deployment.md) owns remote installation and upgrades.

## Environments

Plutus runs in three environments
([Isolated environments and populated-database migrations](./adr/0008-environments-and-migrations.md)).
Automated tests are not an environment; they create their own throwaway
databases.

| Environment                          | Where                            | Adapter                                | Port and database              | Data                           |
| ------------------------------------ | -------------------------------- | -------------------------------------- | ------------------------------ | ------------------------------ |
| dev, `local` instance (you)          | your machine, `go run`           | deterministic fake, seeded             | `0.0.0.0:8080`, `plutus_local` | reset whenever you like        |
| dev, `agent` instance (the AI agent) | your machine, `go run`           | deterministic fake, seeded             | `0.0.0.0:8081`, `plutus_agent` | reset whenever the agent likes |
| test                                 | a remote server, deployed binary | live Adapters with real read-only keys | `0.0.0.0:8082`, `plutus_test`  | reset by hand, never seeded    |
| prod                                 | a remote server, deployed binary | live Adapters with real read-only keys | `0.0.0.0:8083`, `plutus_prod`  | never reset                    |

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
`config/local.env`, then set `PLUTUS_DSN` and `PLUTUS_MASTER_KEY`.
`./scripts/local-serve.sh` rebuilds the SPA and serves the existing data;
`./scripts/local-wipe-and-serve.sh` also resets `plutus_local` with the
walkthrough seed. Other machines on the LAN can open `http://<lan-ip>:8080`.

### Agent setup

Agent sessions may connect only to `plutus_agent` and throwaway
`plutus_gotest_*` databases, never `plutus_local`, `plutus_test`, or
`plutus_prod`. Create the agent database once:

```sql
CREATE DATABASE plutus_agent;
```

For `agent` (`0.0.0.0:8081`, database `plutus_agent`), copy
[config/agent-example.toml](../config/agent-example.toml) to `config/agent.toml`
and [config/agent-example.env](../config/agent-example.env) to
`config/agent.env`, then set `PLUTUS_DSN` and a unique `PLUTUS_MASTER_KEY`.
`./scripts/agent-serve.sh` rebuilds the SPA and keeps the data;
`./scripts/agent-wipe-and-serve.sh` also resets `plutus_agent` with the
walkthrough seed.

## Configuration

Pass `--config` and `--env-file` to any `plutus` command to select an
environment. When `--env-file` is set, that file is the only env file loaded; it
does not merge with `config/local.env`. The MySQL DSN comes only from
`PLUTUS_DSN` or `PLUTUS_DSN_FILE`, never TOML. The credential encryption master
key stays in `PLUTUS_MASTER_KEY` or `PLUTUS_MASTER_KEY_FILE` and must not appear
in TOML. Without a key, credential writes, validation, and enabling return 503
and seeded Identities stay `unconfigured`.

The environment examples contain deployment differences and explicit environment
permissions, rather than every default. Keep their listener, port, cookie
suffix, fake Adapter selection, and reset permission explicit when comparing
files. Comments belong on separate lines above settings.

### Optional settings and defaults

Runtime loading applies code defaults, TOML, then environment overrides. Process
environment values take precedence over values in the selected env file. Unknown
TOML fields fail startup with their names; explicit invalid values, including
`port = 0` or an empty duration, fail rather than reverting to defaults.
Durations use Go notation such as `15s`, `2m`, and `168h`; days are expressed in
hours.

| TOML field                                 | Default     | Purpose                                                                             |
| ------------------------------------------ | ----------- | ----------------------------------------------------------------------------------- |
| `server.listen_address`                    | `"0.0.0.0"` | Bind address                                                                        |
| `server.port`                              | `8080`      | Instance HTTP or HTTPS port                                                         |
| `server.tls.cert_file`                     | `""`        | PEM server certificate; configure together with `key_file`                          |
| `server.tls.key_file`                      | `""`        | PEM private key matching the server certificate                                     |
| `server.tls.client_ca_file`                | `""`        | PEM trust bundle that enables required, verified client certificates                |
| `logging.directory`                        | `"./logs"`  | Log directory relative to the process working directory                             |
| `logging.level`                            | `"info"`    | Application threshold: debug, info, warn, or error                                  |
| `logging.max_size_mb`                      | `100`       | Independent active-file size limit in MiB; must be positive                         |
| `logging.max_backups`                      | `5`         | Backup count per channel; zero removes the count limit                              |
| `logging.max_age_days`                     | `0`         | Backup age limit in 24-hour days; zero removes the age limit                        |
| `logging.compress`                         | `true`      | gzip rotated files                                                                  |
| `logging.sql_enabled`                      | `true`      | Record all project SQL executions and transaction boundaries                        |
| `database.allow_reset`                     | `false`     | Permit the explicit destructive `db reset` command, subject to database-name checks |
| `auth.cookie_suffix`                       | `""`        | Shared suffix for session and CSRF cookie names                                     |
| `auth.session_idle_lifetime`               | `"168h"`    | Session expiry after inactivity; 1 minute to 90 days                                |
| `auth.session_absolute_lifetime`           | `"720h"`    | Maximum Session lifetime; 1 hour to 365 days, at least the idle lifetime            |
| `jobs.finished_retention`                  | `"2160h"`   | Retain finished jobs without report or point references; 1 to 365 days              |
| `collection.connection_validation_timeout` | `"15s"`     | Total budget for one synchronous Connection validation; 1 second to 2 minutes       |
| `collection.fake_adapter`                  | `false`     | Register fake Adapters for dev rather than live Adapters                            |

Fields outside `[logging]` have matching environment overrides: prefix
`PLUTUS_`, uppercase the field path, and replace dots with underscores. For
example, `collection.connection_validation_timeout` becomes
`PLUTUS_COLLECTION_CONNECTION_VALIDATION_TIMEOUT`. The three TLS file settings
are paths to PEM files; their environment values are paths too.

The cookie suffix generates both names. `cookie_suffix = "agent"` produces
`plutus_session_agent` and `plutus_csrf_agent`; an empty suffix produces
`plutus_session` and `plutus_csrf`. Nonempty suffixes may contain letters,
digits, `_`, and `-`. Browser cookies are shared across ports on the same host,
so use a distinct suffix for each instance.

With no certificate or key, Plutus serves HTTP. A complete certificate/key pair
enables HTTPS; an incomplete, unreadable, invalid, or expired pair fails
startup. Adding `client_ca_file` requires HTTPS and enables mTLS with mandatory
verified client certificates. There are no separate TLS enable flags. An
optional HTTPS configuration is:

```toml
[server.tls]
cert_file = "/etc/plutus/server.pem"
key_file = "/etc/plutus/server-key.pem"
# Add this only when every client must present a trusted certificate.
# client_ca_file = "/etc/plutus/client-ca.pem"
```

Finished-job retention includes succeeded, failed, and cancelled jobs, but never
deletes jobs referenced by Account Reports or Valuation Points. It does not set
a retention period for financial history.

### Logging

Runtime commands write single-line text to `app.log`, `access.log`, and
`sql.log` in `logging.directory`. Application records also go to stderr;
`logging.level` controls only application records. Access and enabled SQL
records are independent of that threshold. Help, version, and Adapter recording
do not initialize the runtime logging system. Command results remain on stdout.

Access records contain request ID, method, path, status, and duration, including
health checks, static resources, rejected requests, and recovered panics.
Request IDs accepted from `X-Request-Id` must contain 1–128 ASCII letters,
digits, dots, underscores, or hyphens; otherwise a new ID is generated. That ID
also accompanies application and SQL records issued with the request context.
Claimed background jobs carry job, Portfolio, and, for Collection Cycles, cycle
IDs. Correlation does not persist a request ID across asynchronous job
execution.

SQL logs show statements with parameter values substituted for inspection,
execution duration, and errors. They include transactions, migrations, seeds,
and UTC initialization. Values are not redacted; binary values use hexadecimal
literals. Result rows are not recorded. If rendering is uncertain, the original
statement, arguments, and `render_error` are recorded instead. SQL rendering
never changes database execution. Query duration excludes result consumption.
Turning off `logging.sql_enabled` avoids both the SQL file and SQL formatting.

Logging settings come only from the TOML `[logging]` section and code defaults;
environment variables do not override them. Examples show only `directory` and
`level`; add other fields when changing their documented defaults. Rotation
limits apply separately to each file. Writes are synchronous, without per-record
fsync. Backup compression and cleanup run in the rotation library's background
maintenance; short commands do not wait, and subsequent runs resume maintenance.
See [deployment logs](./deployment.md#logs) for directory ownership and
write-failure behavior.

### Configuration migration

Configuration changes are direct migrations without legacy aliases. Update
existing TOML and environment overrides together before starting the new binary.
Known removed environment names fail with migration instructions, including
those loaded from an env file.

| Previous field                                        | Replacement                                                                            |
| ----------------------------------------------------- | -------------------------------------------------------------------------------------- |
| `server.tls.https_cert_path`                          | `server.tls.cert_file`                                                                 |
| `server.tls.https_key_path`                           | `server.tls.key_file`                                                                  |
| `server.tls.mtls_ca_path`                             | `server.tls.client_ca_file`                                                            |
| `server.tls.https_enabled`, `server.tls.mtls_enabled` | Remove; transport mode follows the configured certificate and CA files                 |
| `auth.session_cookie_name`, `auth.csrf_cookie_name`   | `auth.cookie_suffix`; use `local`, `agent`, `test`, or empty for the supplied examples |
| `jobs.completed_retention`                            | `jobs.finished_retention`                                                              |
| `collection.validate_timeout`                         | `collection.connection_validation_timeout`                                             |
| `database.dsn`                                        | `PLUTUS_DSN` or `PLUTUS_DSN_FILE` outside TOML                                         |
| `jobs.worker_poll_interval`                           | Remove; the serving worker polls an idle queue every 1 second                          |
| `jobs.lease_duration`                                 | Remove; the worker lease is 2 minutes and renews during long jobs                      |
| `collection.default_interval`                         | Remove; new Portfolios start at 15 minutes; edit each Portfolio's interval in settings |
| `collection.stagger`                                  | Remove; Collection Cycles wait 3 seconds between Identities                            |

Existing Portfolio intervals and financial history are unaffected. Both CLI User
creation and subsequent Portfolio creation use the same 15-minute initial
interval. Worker polling, leases, and the Identity delay are internal
mechanisms, not deployment policies.

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
npm --prefix web run build
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
Make targets that compile Go build the SPA first. Run
`npm --prefix web run build` before direct `go build`, `go test`, or `go run`
commands, and again after frontend changes. `./scripts/local-serve.sh` runs the
complete local service on `:8080` and keeps existing data. Restart it after
frontend changes to rebuild the embedded SPA.

See [testing and verification](./testing.md) for validation gates, temporary
MySQL setup, agent runtime verification, and Adapter delivery reports. For
remote environments and upgrades, see [deployment](./deployment.md). Run
`make help` to list targets, or `make run ARGS=--help` for CLI help.
