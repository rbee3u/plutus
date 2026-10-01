---
status: accepted
---

# Separate dev, test, and prod environments and deploy remotely by hand

The recording-key storage rule below is amended by [ADR 0038](./0038-separate-recording-credentials-from-test-configuration.md); the runtime environment boundaries remain accepted.

Until live Platform adapters existed, four databases were split by who used them (`plutus_local`, `plutus_agent`, `plutus_walk`, `plutus_test`). Real exchange data changes the question from "who is looking" to "which adapter is talking and whether the data may be thrown away". Environments are therefore named the way the owner expects from other projects, and automated tests are not an environment at all.

| Environment | Where | Adapter | Data | Instances |
| --- | --- | --- | --- | --- |
| dev | the developer's machine, `go run` | `fake_adapter = true`, seeds | reset at will | `local` (:8080, `plutus_local`, the human) and `agent` (:8081, `plutus_agent`, the AI agent) |
| test | a remote server | `fake_adapter = false`, live read-only keys | reset at will, never seeded | :8082, `plutus_test` |
| prod | a remote server | `fake_adapter = false`, live read-only keys | never reset | :8083, `plutus_prod` |

- An environment runs one adapter kind only: dev never holds a real key, test and prod never register the fake.
- test exists to compare collected numbers with the exchange apps by hand and to rehearse the prod setup. It starts from `migrate up` and `user create`; Identities and keys are entered through the UI. `db reset` without `--profile` drops and migrates it.
- `allow_reset` follows whether the data may be thrown away, not which adapter runs: `local`, `agent`, and test ship `true`; only prod keeps `false`, and its database name contains `prod`, which `db reset` refuses. prod's `PLUTUS_MASTER_KEY` is backed up separately.
- Remote deployment is manual. The repository ships `make dist` (a static `CGO_ENABLED=0` cross-build with the git version injected through ldflags) and `test-example` / `prod-example` configuration pairs. The owner copies the binary and hand-edited `toml` / `env` files, then runs `migrate up`, `user create`, and `serve` on the server. No remote scripts, unit files, or CI deploy steps are kept in the repository.
- The former `walk` lane is dropped; the `local` instance takes over human walkthroughs of fake data. Both dev instances offer a serve script that keeps data and a wipe script that reseeds.
- `go test` reads `PLUTUS_GOTEST_*` variables from gitignored `config/gotest.env`, currently only the throwaway MySQL DSN. Explicit `plutus adapter record` reads separate `PLUTUS_RECORD_*` credentials from gitignored `config/record.env`; it never falls back to test configuration. Runtime commands (`serve`, the worker, `collect`) load neither developer file. [ADR 0037](./0037-separate-recorded-evidence-from-synthetic-tests.md) defines independent synthetic offline tests and reviewed recording evidence; [ADR 0038](./0038-separate-recording-credentials-from-test-configuration.md) separates their credential configuration.

## Consequences

- Key sets per Platform: recording may share the remote test environment's read-only set through `record.env`, while prod keeps a separate set. Offline tests need no Platform credentials. Revoking the test/recording set never affects prod.
- Each environment has its own master key and session cookie name, so instances on one host do not share sessions.
- This supersedes [ADR 0010](./0010-keep-local-mysql-dsn-in-gitignored-plutus-toml.md). Its rule that DSNs and master keys live in gitignored `config/*.env` files and never in logs still holds.
