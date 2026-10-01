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

2. Copy `dist/plutus-linux-amd64` to the server as `/opt/plutus-test/plutus` or
   `/opt/plutus-prod/plutus`, alongside the
   [test config](../config/test-example.toml) /
   [test env](../config/test-example.env) examples, or use the
   [prod config](../config/prod-example.toml) /
   [prod env](../config/prod-example.env) pair. Rename them to `test.toml` /
   `test.env` or `prod.toml` / `prod.env` in the corresponding directory. Set
   the DSN, a fresh master key, listen address, and TLS. Back up the prod master
   key separately; without it, stored Platform credentials cannot be decrypted.

3. Create `plutus_test` or `plutus_prod` once on the server's MySQL, then
   migrate and create a User. The example uses test configuration; substitute
   the prod pair for prod:

   ```sh
   cd /opt/plutus-test
   ./plutus --config test.toml --env-file test.env migrate up
   ./plutus --config test.toml --env-file test.env user create
   ```

4. Start the [systemd service](#systemd-services), then sign in, create
   Identities with real read-only keys, select Accounts, and enable them. The
   scheduler uses each Portfolio's interval. `./plutus --version` identifies a
   binary's build, not an already-running process.

Existing installations must apply the
[configuration migration](./development.md#configuration-migration) before
starting a binary with the simplified configuration schema. Update renamed TOML
keys and environment overrides, remove internal tuning fields, and keep secrets
outside TOML; legacy names are not aliases.

## systemd services

The [test unit](../config/test-example.service) and
[prod unit](../config/prod-example.service) run as root by default, following
the operator's preference. They use separate deployment directories and
configuration files. The examples assume local MySQL named `mysql.service`;
change that name in both `Wants` and `After` if your distribution uses
`mysqld.service`. Remove it from both directives when using remote MySQL.

`WorkingDirectory` lets the configuration arguments use relative paths, such as
`--config test.toml --env-file test.env`. The executable in `ExecStart` must
still use its absolute path: systemd accepts an absolute path or a bare filename
resolved through its executable search path, but not `./plutus`. See the
[systemd command-line rules](https://github.com/systemd/systemd/blob/main/man/systemd.service.xml).

Run the following commands as root on the deployment server after copying and
configuring the test files. `test-example.service` is the uploaded unit example;
for prod, substitute `prod` for `test` in the directory, file, and unit names:

```sh
chown root:root /opt/plutus-test/plutus /opt/plutus-test/test.toml /opt/plutus-test/test.env
chmod 0755 /opt/plutus-test/plutus
chmod 0600 /opt/plutus-test/test.toml /opt/plutus-test/test.env
install -m 0644 test-example.service /etc/systemd/system/plutus-test.service
systemd-analyze verify /etc/systemd/system/plutus-test.service
systemctl daemon-reload
systemctl enable --now plutus-test.service
systemctl status plutus-test.service
journalctl -u plutus-test.service -n 100 --no-pager
curl --fail http://127.0.0.1:8082/health/ready
```

Use port 8083 for prod's readiness check. If application HTTPS is enabled, use
the configured HTTPS address and CA, plus a client certificate when mTLS is
enabled. Check readiness after starting: an active unit alone does not establish
that the database, schema, encryption key, and worker are ready.

`Wants=mysql.service` starts local MySQL without stopping Plutus when MySQL is
restarted. `After` orders startup and shutdown; it does not check database
readiness. `Restart=on-failure` retries failed processes after 30 seconds. The
units inherit systemd's logging, stop timeout, and start-rate limits. With the
usual manager defaults, logs go to the journal, the stop timeout is 90 seconds,
and the start limit is five starts per ten seconds; the 30-second retry interval
stays outside that window. Host configuration and unit drop-ins can override
these defaults.

Migrations and User creation remain explicit CLI operations before service
startup. Stopping the unit sends SIGTERM, which cancels the worker and shuts
down HTTP; it does not guarantee an in-flight Collection Cycle completes.

## Upgrades and resets

Stop the previous service before migrating and starting the new binary; only one
process may use a database. Run `migrate up` with the environment's config and
env file after every new migration. Check migration-specific prerequisites below
before stopping the old release.

With systemd, run `systemctl stop plutus-test.service` (or
`plutus-prod.service`), replace the binary, run the migration, then
`systemctl start` the same unit and check readiness. Run
`systemctl daemon-reload` before starting if the unit file changed. Preserve the
deployment file ownership and modes shown above.

After prod's first `migrate up`, the baseline is frozen. Append migrations that
work on populated databases; never reset prod. See
[ADR 0008](./adr/0008-environments-and-migrations.md).

Test can be reset and collected again without seed data:

```sh
cd /opt/plutus-test
./plutus --config test.toml --env-file test.env db reset
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
