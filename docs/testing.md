# Testing and verification

Use the [development guide](./development.md) for prerequisites and the isolated
agent instance. Automated tests create their own temporary databases.

## Validation gates

Local Go development follows five validation gates in order. `make format` also
runs Prettier and `make lint` also runs the frontend typecheck and ESLint:

1. `make format`: rewrite imports and formatting before validating the code.
2. `make web`: build the SPA for Go embedding.
3. `make build`: confirm the formatted code compiles and build a static
   `dist/plutus-<GOOS>-<GOARCH>` binary for the current platform with the git
   version baked in (`plutus --version`). Override `GOOS` and `GOARCH` to
   cross-compile for another platform.
4. `make test`: run Go and frontend Vitest unit tests without cached results
   after the build passes. This gate is offline: no external requests or live
   keys; loopback replay and temporary MySQL are allowed.
5. `make lint`: run the configured linters after tests pass.

Each gate must pass before proceeding to the next. Stop on failure, fix the
issue, and repeat the sequence:

```sh
make format && make web && make build && make test && make lint
```

`make style` is intended for future GitHub CI, where it replaces `make format`
as the first gate. It checks formatting without rewriting files and fails when
formatting corrections are needed; frontend build, Go build, tests, and lint
follow in the same order:

```sh
make style && make web && make build && make test && make lint
```

Go lint uses a reviewed allowlist focused on correctness, security, and resource
lifetimes, including test files. Staticcheck runs `SA*` except the `SA6*`
performance checks. Gocritic retains behavior diagnostics while excluding style,
performance, comment, builtin-naming, and simplification preferences. Govet also
runs `nilness` and `unusedwrite` to check nil dereferences, redundant nil
comparisons, and writes to struct or array copies that are never read.
Formatting is checked separately; general API-shape preferences and speculative
micro-optimizations are not lint gates.

Narrowly scoped exclusions allow fixture type assertions, invented credentials,
and test-owned paths. The SQL compatibility wrapper, incoming test cookies, and
forbidden network probes have documented file-specific exceptions in
`.golangci.yml`. Suppressions must name the linter and explain the exception;
unused suppressions fail lint. Explicitly discarded errors still require review:
the default errcheck settings allow assignments to `_`.

G115 integer-conversion checks and strict enum exhaustiveness apply across the
project. Validate externally supplied narrowing conversions before casting.
Rule-specific `#nosec G115` comments document established bounds or intentional
signed-bit encodings; revisit those assumptions when changing the caller or
storage contract. Platform-specific switches may explicitly suppress
`exhaustive` when they intentionally handle only a subset of shared Account
Types. Such exceptions never expand supported Accounts.

The optional Chromium check uses a loopback-only Vite server and mocked API
responses, so it needs no database. It covers mobile navigation behavior that
DOM-only checks cannot verify. Install the bundled browser once, then run:

```sh
npm --prefix web exec playwright install chromium
npm --prefix web run test:chromium
```

It is separate from `make test`, which remains the routine offline gate.

`make generate` regenerates the sqlc queries, the Go strict server, and the
TypeScript client; the tree must be unchanged afterwards.

## Documentation verification

After dependencies are installed with `make deps`, use the root Prettier
configuration for Markdown-only changes:

```sh
web/node_modules/.bin/prettier --write AGENTS.md CONTEXT.md README.md 'docs/**/*.md'
web/node_modules/.bin/prettier --check AGENTS.md CONTEXT.md README.md 'docs/**/*.md'
```

Check local paths and heading anchors, including links, image paths, and source
comments affected by renames. Commands and behavioral claims must match code,
configuration, and Makefile. Prose-only changes need no application or database
commands; edits to Go comments still follow the Go validation gates.

## Agent runtime verification

After changes affecting `serve`, the JSON API, or the UI, exercise the changed
flow in the agent dev instance in addition to the validation gates. This
includes seed data, valuation reads, the embedded SPA, and history defaults.
Changes confined to Adapter packages, their tests, or fixtures use offline tests
and evidence review instead.

1. If `config/agent.toml`, `config/agent.env`, or MySQL database `plutus_agent`
   is missing, stop and report the setup needed: copy
   [config/agent-example.toml](../config/agent-example.toml) and
   [config/agent-example.env](../config/agent-example.env) to their agent
   counterparts, set `PLUTUS_DSN` and a unique `PLUTUS_MASTER_KEY`, and run
   `CREATE DATABASE plutus_agent;`. Never substitute `plutus_local`.
2. Start `./scripts/agent-wipe-and-serve.sh` in the background, or
   `./scripts/agent-serve.sh` to keep data. Wait for
   `GET http://127.0.0.1:8081/health/live` to succeed.
3. Open `http://127.0.0.1:8081` and follow the changed path by clicking, typing,
   submitting, and navigating. Verify the embedded SPA, not :8080 or Vite. For
   API-only changes, use `curl` against affected routes on :8081. A screenshot
   alone is insufficient.
4. Stop every process started by the session, including child `serve` processes,
   by terminating their process groups. Confirm
   `GET http://127.0.0.1:8081/health/live` fails and :8081 is no longer
   occupied. Leave the user's :8080 processes alone.
5. Report what was exercised on :8081, what was observed, what remains
   uncovered, and that the started processes were stopped. If setup or
   verification was blocked, report that limitation rather than claiming runtime
   or UI completion.

## Adapter delivery reports

For a new or expanded real Platform Adapter delivery, also run
`make build GOOS=linux GOARCH=amd64` after `make web`. Report four items:
recorded evidence with dates and gaps; independent synthetic test results per
affected Platform; agent-environment coverage when applicable; and what was not
covered. Documented or synthetic scenarios must remain distinct from observed
real coverage.

## Test configuration

Test configuration uses `PLUTUS_GOTEST_*` in gitignored `config/gotest.env`
(copy [config/gotest-example.env](../config/gotest-example.env)) or the process
environment, which wins. Currently it contains only the throwaway MySQL DSN.
Recording credentials are configured separately through the
[evidence workflow](./adapters/evidence-workflow.md#record-one-interface-prepare-a-group).
Tests never load them; runtime `serve`, the worker, and `collect` load neither
developer configuration source.

- `PLUTUS_GOTEST_DSN` is a server-level MySQL DSN with no database name
  (`user:password@tcp(127.0.0.1:3306)/`). Each test package that needs MySQL
  creates its own `plutus_gotest_<unix>_<hex>` database in `TestMain`, migrates
  it, runs, and drops it. Nothing has to be created by hand, editing the
  migration never leaves stale tables behind, and packages run in parallel
  without sharing state. Databases older than an hour, left behind by a killed
  run, are dropped on the next run. Without the variable those tests skip and
  `make test` still passes.
- Real Adapter tests select synthetic response files and replay them through
  loopback servers; higher-level application tests can use the deterministic
  fake. The network guard fails forbidden HTTP/HTTPS/WebSocket, proxy and
  redirect attempts independently of report error mapping. Local replay and
  temporary MySQL remain supported.

## Recorded evidence

Recorded responses are reviewed interface evidence, separate from automated
tests. The [evidence workflow](./adapters/evidence-workflow.md) owns recording
credentials, exact requests, capture guarantees, sanitation, and promotion. Each
Platform's verification document tracks dates and outstanding real gaps;
exchange-app reconciliation remains a separate check.
