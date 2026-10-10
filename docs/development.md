# Development

Use this guide for environment setup, runtime configuration, seeds, and the
embedded frontend. [Testing](./testing.md) owns validation procedures;
[deployment](./deployment.md) owns remote installation and upgrades.

## Environments

Plutus runs in three environments
([Environment and configuration boundaries](./adr/0010-environments-and-configuration.md)).
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

Pass `--config` and `--env-file` to runtime `plutus` commands to select an
environment. Explicit `adapter record` uses separate
[recording configuration](./platform/evidence-workflow.md#record-one-interface-prepare-a-group).
When `--env-file` is set, that file is the only env file loaded; it does not
merge with `config/local.env`. The MySQL DSN comes only from `PLUTUS_DSN` or
`PLUTUS_DSN_FILE`, never TOML. The credential encryption master key stays in
`PLUTUS_MASTER_KEY` or `PLUTUS_MASTER_KEY_FILE` and must not appear in TOML.
Without a key, credential writes, validation, and enabling return 503. Seed
commands require a master key.

The environment examples contain deployment differences and explicit environment
permissions, rather than every default. Keep their listener, port, cookie
suffix, fake Adapter selection, and reset permission explicit when comparing
files. Comments belong on separate lines above settings.

### Optional settings and defaults

Ordinary settings use code defaults followed by TOML only. Environment variables
and env files supply secrets only; process values take precedence over the
selected env file. Ordinary configuration variables, including removed names,
are silently ignored. Unknown TOML fields fail startup with their names;
explicit invalid values, including `port = 0` or an empty duration, fail rather
than reverting to defaults. Durations use Go notation such as `15s`, `2m`, and
`168h`; days are expressed in hours.

| TOML field                                 | Default     | Purpose                                                                                |
| ------------------------------------------ | ----------- | -------------------------------------------------------------------------------------- |
| `server.listen_address`                    | `"0.0.0.0"` | Bind address                                                                           |
| `server.port`                              | `8080`      | Instance HTTP or HTTPS port                                                            |
| `server.tls.cert_file`                     | `""`        | PEM server certificate; configure together with `key_file`                             |
| `server.tls.key_file`                      | `""`        | PEM private key matching the server certificate                                        |
| `server.tls.client_ca_file`                | `""`        | PEM trust bundle that enables required, verified client certificates                   |
| `logging.directory`                        | `"./logs"`  | Log directory relative to the process working directory                                |
| `logging.level`                            | `"info"`    | Application threshold: debug, info, warn, or error                                     |
| `logging.max_size_mb`                      | `100`       | Independent active-file size limit in MiB; must be positive                            |
| `logging.max_backups`                      | `5`         | Backup count per channel; zero removes the count limit                                 |
| `logging.max_age_days`                     | `0`         | Backup age limit in 24-hour days; zero removes the age limit                           |
| `logging.compress`                         | `true`      | gzip rotated files                                                                     |
| `logging.sql_enabled`                      | `true`      | Record all project SQL executions and transaction boundaries                           |
| `database.allow_reset`                     | `false`     | Permit the explicit destructive `db reset` command, subject to database-name checks    |
| `auth.cookie_suffix`                       | `""`        | Shared suffix for session and CSRF cookie names                                        |
| `auth.session_idle_lifetime`               | `"168h"`    | Session expiry after inactivity; 1 minute to 90 days                                   |
| `auth.session_absolute_lifetime`           | `"720h"`    | Maximum Session lifetime; 1 hour to 365 days, at least the idle lifetime               |
| `collection.connection_validation_timeout` | `"15s"`     | Total budget for one synchronous Connection validation; 1 second to 2 minutes          |
| `collection.fake_adapter`                  | `false`     | Register fake Adapters for dev rather than live Adapters                               |
| `collection.proxy.mode`                    | `"direct"`  | Runtime Platform proxy policy: direct or proxy                                         |
| `collection.proxy.url`                     | `""`        | HTTP/HTTPS/SOCKS5/SOCKS5H URL with host and explicit port; required only in proxy mode |

The three TLS settings are paths to PEM files and belong in TOML, including the
private-key path. Secret values remain outside TOML. Environment variables such
as `PLUTUS_SERVER_PORT` cannot change ordinary settings, even when the TOML
field is omitted.

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

Collection Cycles are permanent execution history: Account Reports and Valuation
Points reference them. Financial summaries are permanent; detailed snapshots
follow the [retention policy](./domain-rules.md#historical-detail-retention).

### Platform proxies

One `[collection.proxy]` policy applies to all live runtime Adapters, including
Connection validation, Collection, market data, and Kraken WebSocket reads.
Restart the process after changing it. Each Adapter retains its own request
budget; direct connections and proxy first hops use IPv4. An external proxy
chooses the address family and public IP used to reach the Platform.

```toml
[collection.proxy]
mode = "proxy"
url = "http://127.0.0.1:7890"
```

- `direct` is the default and connects without a proxy.
- `proxy` forces the configured URL for every runtime Platform request. An
  unavailable proxy fails the request; it never triggers automatic direct
  fallback. Temporary failures retain compatible earlier Account Reports as
  stale under the normal collection rules.

Neither mode reads `HTTP_PROXY`, `HTTPS_PROXY`, `ALL_PROXY`, or `NO_PROXY`,
including lowercase variants. The removed `environment` mode fails validation
with migration instructions.

The URL must be empty in `direct` mode. In `proxy` mode it must use `http`,
`https`, `socks5`, or `socks5h`, with a host and explicit port from 1 to 65535.
Only an empty or root path is allowed; usernames, passwords, queries, and
fragments are rejected. TLS certificate verification remains enabled.

For a Shadowsocks exit on another server, run sing-box separately. Plutus
connects to its loopback HTTP/SOCKS entry point, sing-box connects over SS to
the exit server, and that server reaches the Platform. Bind the local entry
point only to `127.0.0.1`, keep SS credentials in sing-box, and configure its
routes without direct fallback. Ensure the exit uses the IPv4 address
allowlisted for the Platform API keys. Plutus does not accept `ss://` or manage
the tunnel.

The deterministic fake stays offline and does not create proxy clients. Explicit
`adapter record` uses an independent TOML proxy policy with the same validation
and direct default. It reads neither runtime configuration nor proxy environment
variables. See the
[recording workflow](./platform/evidence-workflow.md#record-one-interface-prepare-a-group)
for file selection and credential isolation.

See the [runtime proxy delivery report](./platform/proxy-verification.md) for
synthetic and agent verification coverage and remaining deployment gaps.

### Logging

Runtime commands write `app.log`, `access.log`, and enabled `sql.log` under
`logging.directory`; application records also go to stderr. `logging.level`
affects only application records. Help, version, and Adapter recording do not
initialize logging; command results stay on stdout.

Access logs include request ID, method, path, status, and duration for normal,
rejected, and recovered requests, including health checks and static files.
Valid `X-Request-Id` values are reused; otherwise an ID is generated under the
[middleware rules](../internal/transport/http/middleware.go). Request context
correlates application and SQL records. Collection Cycles use Portfolio and
cycle IDs rather than retaining the originating request ID.

SQL logs include full parameter values, duration, errors, transactions,
migrations, seeds, and UTC initialization, but no result rows. Binary values use
hex; uncertain rendering records the template, arguments, and `render_error`.
Duration excludes result consumption. Set `logging.sql_enabled = false` to avoid
both the SQL file and formatting.

Use the TOML fields in [defaults](#optional-settings-and-defaults); examples
show only directory and level. [Deployment logs](./deployment.md#logs) owns
rotation, directory ownership, and failure handling;
[architecture](./architecture.md#operations-and-limits) owns integration.

### Configuration migration

Configuration changes are direct migrations without legacy aliases. Move all
ordinary environment overrides into TOML before starting the new binary; both
current and previously removed non-secret environment names are silently
ignored, even when empty or invalid. TOML retains strict field and value
validation.

Replace `collection.proxy.mode = "environment"` with `direct`, or with `proxy`
and an explicit URL. Existing proxy environment variables no longer route
runtime or recording traffic. For recording, configure `[proxy]` in its separate
TOML file. An omitted policy defaults to direct; an explicit `environment` mode
fails rather than silently switching routes. Secrets and stored data do not need
migration.

| Previous field                                        | Replacement                                                                            |
| ----------------------------------------------------- | -------------------------------------------------------------------------------------- |
| `server.tls.https_cert_path`                          | `server.tls.cert_file`                                                                 |
| `server.tls.https_key_path`                           | `server.tls.key_file`                                                                  |
| `server.tls.mtls_ca_path`                             | `server.tls.client_ca_file`                                                            |
| `server.tls.https_enabled`, `server.tls.mtls_enabled` | Remove; transport mode follows the configured certificate and CA files                 |
| `auth.session_cookie_name`, `auth.csrf_cookie_name`   | `auth.cookie_suffix`; use `local`, `agent`, `test`, or empty for the supplied examples |
| `jobs.completed_retention`, `jobs.finished_retention` | Remove; Collection Cycles remain as permanent history                                  |
| `collection.validate_timeout`                         | `collection.connection_validation_timeout`                                             |
| `database.dsn`                                        | `PLUTUS_DSN` or `PLUTUS_DSN_FILE` outside TOML                                         |
| `jobs.worker_poll_interval`                           | Remove; the serving worker polls an idle queue every 1 second                          |
| `jobs.lease_duration`                                 | Remove; the worker lease is 2 minutes and renews during long cycles                    |
| `collection.default_interval`                         | Remove; new Portfolios start at 15 minutes; edit each Portfolio's interval in settings |
| `collection.stagger`                                  | Remove; Collection Cycles wait 3 seconds between Identities                            |

Existing Portfolio intervals and financial history are unaffected. Both CLI User
creation and subsequent Portfolio creation use the same 15-minute initial
interval. Worker polling, leases, and the Identity delay are internal
mechanisms, not deployment policies.

## Seeds and collection

`db reset` drops every table and reapplies migrations. Without `--seed`, it
leaves an empty migrated database. It requires `[database].allow_reset = true`
in the selected TOML and a database name matching `plutus` or `plutus_<suffix>`
that does not contain `prod`. `serve` never migrates or seeds automatically.

The single Alice dataset covers healthy, edge-case, large-list, and empty
Portfolios. See [seeds](./seeds.md) for scenarios, history windows, expected
results, and measured reset times.

Seeding requires the fake Adapter and a master key. It writes source facts and
uses real valuation code to initialize immutable points. Continued fake
collection shares the original anchor. `db seed` requires an empty migrated
database; replace old dev data with the wipe script or `db reset --seed`. The
retired `--profile` flag is rejected. See
[ADR 0010](./adr/0010-environments-and-configuration.md).

Equivalent commands without dropping the database:

```sh
make web
go run ./cmd/plutus --config config/local.toml --env-file config/local.env migrate up
go run ./cmd/plutus --config config/local.toml --env-file config/local.env db seed
go run ./cmd/plutus --config config/local.toml --env-file config/local.env serve
go run ./cmd/plutus --config config/local.toml --env-file config/local.env collect --portfolio <uuid>
```

Sign in with the [seed login](./seeds.md).

Alice's `default` Portfolio is undeletable. Once `serve` runs, the scheduler
collects enabled Connections every 15 minutes. `Collect now` on Overview and
`plutus collect --portfolio <uuid>` trigger a cycle immediately. The empty
Portfolio has no enabled Connection and is not scheduled.

Public registration is not offered. Create Users with `plutus user create`.
Passwords must be 8 to 128 characters.

## Embedded frontend

`serve` hosts the SPA from `web/dist` via `go:embed`. The directory is generated
and ignored by Git. After installing frontend dependencies with `make deps`, run
`make web` to build the SPA before `make build`, `make test`, `make lint`, or
`make run`, and again after frontend changes. These targets are independent; the
caller controls when to rebuild the frontend. Direct `go build`, `go test`, and
`go run` commands also require the generated SPA.

Each `*-serve.sh` script runs `make web` once before starting the service. The
wipe-and-serve scripts reuse that build for both reset and serve commands.
Restart after frontend changes to rebuild the embedded SPA. Run `make help` for
targets or `make run ARGS=--help` for CLI help.
