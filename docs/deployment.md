# Deployment

Remote test/prod use live read-only keys; see
[environment boundaries](./development.md#environments). Examples below use
test. For prod, replace `test` with `prod` in paths and commands, and use port
8083 instead of 8082.

## Installation

1. Build the server binary with the SPA:

   ```sh
   make web && make build GOOS=linux GOARCH=amd64
   ```

2. Upload it as `/opt/plutus-test/plutus`. Copy the
   [test TOML](../config/test-example.toml) and
   [env](../config/test-example.env) examples as `test.toml` and `test.env` in
   that directory; prod uses the [prod TOML](../config/prod-example.toml) and
   [env](../config/prod-example.env) pair. Set the DSN, a fresh master key, and
   listen address. Back up the prod master key separately; losing it makes
   stored Platform credentials unreadable.

3. Prepare the [certificates](#certificates), create MySQL database
   `plutus_test`, then migrate and create a User:

   ```sh
   cd /opt/plutus-test
   ./plutus --config test.toml --env-file test.env migrate up
   ./plutus --config test.toml --env-file test.env user create
   ```

4. Start the [service](#systemd-services), sign in, create and enable Identities
   with read-only keys, and select their Accounts.

## Certificates

Both TOML examples require mTLS. Prepare these files in the deployment
directory:

- Obtain `server.pem` and `server-key.pem` from
  [Cloudflare Origin CA](https://developers.cloudflare.com/ssl/origin-configuration/origin-ca/).
  Save the matching CA root as `origin-ca.pem` for the curl readiness check.
- Run [test-certs.sh](../config/test-certs.sh) or
  [prod-certs.sh](../config/prod-certs.sh) there using OpenSSL with
  `req -addext` support. Plutus trusts the generated `client-ca.pem`; upload
  `client.pem` and `client-key.pem` to
  [Cloudflare Authenticated Origin Pulls](https://developers.cloudflare.com/ssl/origin-configuration/authenticated-origin-pull/set-up/zone-level/).

The scripts use unique working directories under `/tmp/plutus/`, clean them up
on exit, refuse existing files, and discard the CA private key. For renewal,
generate a new set separately, coordinate Cloudflare's client certificate and
Plutus's CA replacement, then restart Plutus.

## systemd services

Upload the [test unit](../config/test-example.service) or
[prod unit](../config/prod-example.service). They run as root and expect
`mysql.service`; adjust `Wants` and `After` for a different service name, or
remove both for remote MySQL.

Run as root on the server:

```sh
cd /opt/plutus-test
chown root:root plutus test.toml test.env server-key.pem client-key.pem
chmod 0755 plutus
chmod 0600 test.toml test.env server-key.pem client-key.pem
install -m 0644 test-example.service /etc/systemd/system/plutus-test.service
systemd-analyze verify /etc/systemd/system/plutus-test.service
systemctl daemon-reload
systemctl enable --now plutus-test.service
journalctl -u plutus-test.service -n 100 --no-pager
curl --fail --noproxy '*' \
  --cacert origin-ca.pem --cert client.pem --key client-key.pem \
  --resolve plutus-test.example.com:8082:127.0.0.1 \
  https://plutus-test.example.com:8082/health/ready
```

Replace the hostname in both curl arguments with one covered by `server.pem`.
The check connects directly to loopback and verifies the server with
`origin-ca.pem`. Require readiness success after each start; an active unit
alone is insufficient.

## Logs

The default directory is `./logs` relative to the unit's `WorkingDirectory`, so
test and prod files are isolated in their deployment directories. Runtime
commands create `app.log`, `access.log`, and, when enabled, `sql.log`;
application records also appear in the systemd journal. New directories use mode
`0750` and new files use `0600`. The service user needs write access to the
directory.

Exactly one process may use a log directory. Stop `serve` before running
maintenance or `collect` against its database, following the existing database
ownership rule. Separate instances need separate working directories or
`logging.directory` overrides. No process lock enforces this convention.

Each file rotates before a write would exceed 100 MiB by default. Backups are
named `app-<UTC timestamp>.log.gz` (and likewise for access/SQL), with five
backups per channel and no default age limit. Configure count, age, size, and
compression through the [logging settings](./development.md#logging).
Compression and cleanup are asynchronous, not a disk quota or an exit-time
guarantee; short commands may leave plain backups for the next run to process,
and maintenance errors are not returned by the rotation library.

Unusable active log files fail startup. During operation, write or rotation
failure reports the error and affected record to stderr; subsequent writes retry
the file. Records individually exceeding the size limit also go to stderr.
Normal shutdown closes active files after HTTP, worker, and database producers
stop. No per-record fsync is performed.

SQL logging is enabled by default and includes full parameter values,
migrations, and seed SQL. This is the accepted inspection policy. Disable it
with `sql_enabled = false` in the TOML `[logging]` section when it is not
needed. Log levels control application output independently of access/SQL.

## Upgrades and resets

Check migration prerequisites below and apply any
[configuration migration](./development.md#configuration-migration). Stop the
service, replace the binary, run `migrate up`, then start the service and check
readiness. Only one process may use a database. Run `systemctl daemon-reload`
when the unit changes; preserve file ownership and permissions.

After prod's first migration, freeze the baseline and append migrations that
work on populated databases. Never reset prod; see
[ADR 0008](./adr/0008-environments-and-migrations.md).

Test allows reset with `allow_reset = true`; it needs no seed data:

```sh
cd /opt/plutus-test
./plutus --config test.toml --env-file test.env db reset
```

### Dual Investment split upgrade

Before applying
[20260929010000_dual_positions.sql](../db/migrations/20260929010000_dual_positions.sql)
and
[20260929010100_dual_accounts.sql](../db/migrations/20260929010100_dual_accounts.sql),
drain running or partially collected cycles with the previous release, then stop
all workers. Migration refuses unfinished cycles. Missing Dual Accounts inherit
Earn selections; existing Dual selections and history remain. No reset is
needed. See [report scope rules](../CONTEXT.md#report-scope-compatibility).

## Platform connectivity

Allow the server's public IPv4 address in each Platform API key. Live Adapters
use IPv4 for HTTP and Kraken WebSocket connections; an external proxy determines
the address family seen by the Platform.

For an optional instance-wide HTTP/SOCKS proxy, configure
[`collection.proxy`](./development.md#platform-proxies) and restart Plutus. A
local sing-box entry point may forward through SS to an allowlisted exit; Plutus
does not implement SS or fall back to direct access. Verify the actual exit IPv4
and successful Account Reports separately from readiness. When moving an
existing database, retain its original credential encryption master key.

### Position presentation upgrade

Stop the service and apply
[20261003010000_position_presentation.sql](../db/migrations/20261003010000_position_presentation.sql)
with `migrate up` before running the new binary. This adds nullable provenance
fields on populated databases and preserves original quantities and amounts.
Exact canonical Position references restore one-way/hedged mode; noncanonical
references, contradictory directions, and unrecorded historical Report Margin
Modes remain unknown. New successful collection supplies the new provenance;
stale fallback keeps the old Report's known fields and observation time.

### Earn presentation upgrade

Stop the service and apply
[20261005010000_earn_positions.sql](../db/migrations/20261005010000_earn_positions.sql)
with `migrate up` before running the new binary. Existing populated reports
retain their financial values and original observation times. The new collection
flag starts false for history: the UI shows separate Balance summaries until a
new successful Earn report supplies normalized descriptions. No scope
reclassification, historical product reconstruction, or database reset is
required. See [ADR 0018](./adr/0018-earn-position-presentation.md).
