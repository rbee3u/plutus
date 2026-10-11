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

## MySQL capacity

The [planning scale](./architecture.md#persistence-and-retention) is a fixed
20-Identity workload. The
[capacity verification](./platform/schema-provenance-delivery.md#full-retained-capacity)
records physical retained-data measurements and their limits. The target host
has four cores, 8 GB RAM, and about 30 GB of total disk. Keep long-term backups
off-host. Use this decimal-GB planning budget for the whole machine:

| Allocation                      | Budget | Purpose                                                                     |
| ------------------------------- | -----: | --------------------------------------------------------------------------- |
| Application table files         |  16 GB | Data, indexes, page slack, pinned evidence and bounded catalog/audit growth |
| OS and application              |   4 GB | System files, binaries and deployed assets                                  |
| Database and application logs   |   4 GB | Binary logs, redo/undo and rotated application logs                         |
| Temporary and maintenance space |   6 GB | One table rebuild and operating free space                                  |

These allocations are operating limits to monitor, not quotas enforced by the
application. A full database copy or local long-term backup does not fit this
maintenance allowance. A table rebuild must have room for that table and its
indexes before it starts. Recheck the budget if product documents, retained
stale sources, logs or catalogs grow beyond the measured shape. When MySQL and
Plutus share a dedicated host, set at least:

```ini
[mysqld]
innodb_buffer_pool_size = 4G
binlog_expire_logs_seconds = 259200
binlog_transaction_compression = ON
binlog_row_image = MINIMAL
```

The default 30-day binary log retention holds several gigabytes of collection
writes. Keep about three days for point-in-time recovery, or start MySQL with
`disable_log_bin` when neither replication nor point-in-time recovery is used;
backups are then the only restore points. InnoDB reuses pages freed by retention
but never returns them to the file system, so monitor free disk and
`information_schema.TABLES` sizes instead of expecting pruning to shrink files.
`OPTIMIZE TABLE` reclaims a table's space but needs room for a full copy.

## Upgrades and resets

Check migration prerequisites below and apply any
[configuration migration](./development.md#configuration-migration). Stop the
service, replace the binary, run `migrate up`, then start the service and check
readiness. Only one process may use a database. Run `systemctl daemon-reload`
when the unit changes; preserve file ownership and permissions.

After prod's first migration, freeze the baseline and append migrations that
work on populated databases. Never reset prod; see
[ADR 0011](./adr/0011-retained-data-and-upgrades.md).

Test allows reset with `allow_reset = true`; it needs no seed data:

```sh
cd /opt/plutus-test
./plutus --config test.toml --env-file test.env db reset
```

### Prelaunch baseline

The initial schema is
[20261010160000_schema.sql](../db/migrations/20261010160000_schema.sql). Earlier
development schemas are disposable; reset those environments before starting
this build. There is no populated-database upgrade path from the retired
development baselines and no compatibility reader for their JSON. Deploy the
server and embedded SPA together, then check Catalog, Manual editing, Capital
Flows, collection, and history. Agent verification uses only the isolated
[agent instance](./development.md#agent-setup).

This reset decision applies only before the first production migration. Once
production receives the baseline, future changes require appended migrations,
backups, and populated-database verification. MySQL DDL is not transactional;
inspect or restore a failed partial migration before retrying.

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
