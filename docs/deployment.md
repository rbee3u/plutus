# Deployment

Use this guide for remote test/prod installation, upgrades, and Platform
connectivity. See [development environments](./development.md#environments) for
database, port, Adapter, and credential boundaries. Agent sessions use only the
agent dev instance and throwaway test databases.

## Installation

Deployment is manual; read each command before running it.

1. Build the static binary for the server, including the SPA and Git version:

   ```sh
   make build GOOS=linux GOARCH=amd64
   ```

2. Copy `dist/plutus-linux-amd64` and the
   [test config](../config/test-example.toml) /
   [test env](../config/test-example.env) examples to the server, or use the
   [prod config](../config/prod-example.toml) /
   [prod env](../config/prod-example.env) pair. Rename them to `test.toml` /
   `test.env` or `prod.toml` / `prod.env`. Set the DSN, a fresh master key,
   listen address, and TLS. Back up the prod master key separately; without it,
   stored Platform credentials cannot be decrypted.

3. Create `plutus_test` or `plutus_prod` once on the server's MySQL, then
   migrate, create a User, and start the service. The example uses test
   configuration; substitute the prod pair for prod:

   ```sh
   ./plutus-linux-amd64 --config test.toml --env-file test.env migrate up
   ./plutus-linux-amd64 --config test.toml --env-file test.env user create
   ./plutus-linux-amd64 --config test.toml --env-file test.env serve
   ```

4. Sign in, create Identities with real read-only keys, select Accounts, and
   enable them. The scheduler uses each Portfolio's interval. Run `serve` under
   the server's supervisor, such as systemd, tmux, or nohup; the repository
   ships no unit file. `./plutus-linux-amd64 --version` identifies a binary's
   build, not an already-running process.

## Upgrades and resets

Stop the previous service before migrating and starting the new binary; only one
process may use a database. Run `migrate up` with the environment's config and
env file after every new migration. Check migration-specific prerequisites below
before stopping the old release.

After prod's first `migrate up`, the baseline is frozen. Append migrations that
work on populated databases; never reset prod. See
[ADR 0008](./adr/0008-environments-and-migrations.md).

Test can be reset and collected again without seed data:

```sh
./plutus-linux-amd64 --config test.toml --env-file test.env db reset
```

Reset requires `allow_reset = true`. Prod keeps it false, and the command also
refuses any database whose name contains `prod`.

### Dual Investment split upgrade

This procedure applies when upgrading from combined Earn history through
[20260929010000_dual_positions.sql](../db/migrations/20260929010000_dual_positions.sql)
and
[20260929010100_dual_accounts.sql](../db/migrations/20260929010100_dual_accounts.sql).

Drain running or partially collected cycles with the previous release, stop all
workers, run `migrate up`, then start only the new binary. Migration refuses
unfinished cycles. Missing Dual Accounts inherit existing Earn selections;
existing Dual configuration is retained. No database reset is needed.

History is preserved, and Value Changes spanning the split are incomplete. Until
ordinary Earn has a successful report in its new scope, an old combined report
cannot be carried forward. Scope semantics belong in the
[domain model](../CONTEXT.md#report-scope-compatibility).

## Platform connectivity

Live Adapters use IPv4 for HTTP and Kraken WebSocket connections. Allow the
server's public IPv4 address in each Platform API key. An HTTP proxy controls
which address family the Platform sees. The accepted connectivity choice is in
[ADR 0008](./adr/0008-environments-and-migrations.md).
