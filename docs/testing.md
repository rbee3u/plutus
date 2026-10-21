# Testing and verification

Use the [development guide](./development.md) for prerequisites and the isolated
agent instance. Automated tests create their own temporary databases.

## Validation gates

Run the gates in order; fix any failure and restart the sequence:

```sh
make format && make web && make build && make test && make lint
```

- `format` rewrites Go imports/formatting and runs frontend Prettier.
- `web` builds the SPA required for Go embedding.
- `build` writes a static, versioned `dist/plutus-<GOOS>-<GOARCH>` binary.
- `test` runs uncached Go and frontend unit tests. Only loopback replay and
  temporary MySQL are allowed; no external requests or live keys.
- `lint` runs Go linters, frontend typecheck, and ESLint.

Reuse the host's Go cache configuration without overrides or persistent changes.
Request elevated execution when sandbox restrictions are expected or
encountered. For future CI, `make style` replaces `make format` with a
non-mutating check. `make generate` regenerates sqlc and Go/TypeScript API
bindings; unchanged sources must produce no diff.

Lint prioritizes correctness, security, and resource lifetimes. Backend
[import boundaries](./adr/0007-backend-architecture.md) and the
[composition-root convention](./architecture.md#module-responsibilities) are
upheld by review; frontend
[module boundaries](./adr/0013-frontend-architecture.md) are checked by ESLint.
Linter settings and exclusions live in [`.golangci.yml`](../.golangci.yml) and
[`web/eslint.config.js`](../web/eslint.config.js); style preferences and
speculative micro-optimizations are outside these gates. Rule-specific
suppressions must explain their bounds or exception; unused suppressions fail.
Review discarded errors: default errcheck settings allow `_` assignments.
Platform-specific `exhaustive` exceptions do not expand supported accounts.

The optional Chromium check uses a loopback-only Vite server and mocked API
responses, so it needs no database. It covers mobile navigation behavior that
DOM-only checks cannot verify. Install the bundled browser once, then run:

```sh
npm --prefix web exec playwright install chromium
npm --prefix web run test:chromium
```

This is separate from `make test`. Put output under `/tmp/plutus`, for example
`-- --output=/tmp/plutus/<session>/playwright`.

Mobile cases cover page and overlay scrolling, orientation changes, URL-backed
reloads, and return to desktop layout. Vite-only fixtures exercise overlapping
dialogs and cleanup; they are excluded from the SPA.

Chromium cannot establish native Safari or Android Chrome pull-to-refresh. Check
ordinary tabs on actual devices with short/long pages, expanded/collapsed
toolbars, and open/closed overlays. Confirm a top-of-page pull reloads the same
URL without requesting collection. Record device/browser versions and outcomes;
leave coverage outstanding until observed. Embedded and home-screen modes are
outside acceptance scope.

## Secret scanning

Run from the repository root:

```sh
gitleaks git -v --redact
```

`--redact` hides secret values in output. [`.gitleaks.toml`](../.gitleaks.toml)
keeps the default rules and ignores reviewed false positives by exact value and
file path. Review new findings before adding exceptions.

## Read-query benchmarks

The financial package has opt-in benchmarks for a ten-year timeline and large
manual/exclusion version histories. They use the ordinary temporary MySQL
harness, count application reads and consumed rows, report allocations, and
inspect scan plans with `EXPLAIN ANALYZE`:

```sh
go test ./internal/service/financial -run '^$' \
  -bench 'Benchmark(History|Revision|ReadScope|PolicyVersion|ComparisonEvent)Queries' \
  -benchtime=3x -v
```

Fixture setup is outside timed sub-benchmarks. Benchmarks are opt-in; the
read-scope regression also uses its shared fixture during `make test`. Keep
output under `/tmp/plutus`; do not substitute a live or pre-created database.
Measurements and their limits are recorded in
[query performance](./query-performance.md).

History benchmarks cover UTC, Asia/Shanghai, and America/New_York, with daily
and hourly curves. The ten-year hourly case expects the point-limit error and
measures early termination. Regression tests cover the 10,000-header cursor,
equal-time successors, DST buckets, raw page totals, and 256-asset policy
batches.

Read-scope benchmarks use 2,000 assets, 10,000 rule versions, and 100 manual
accounts to distinguish output row counts from lookup work. Policy-version
benchmarks compare recent and early windows over 10,000 versions of one asset.
The scoped overlay regression logs opening-rule, page, and count scan plans.
HTTP regressions also count successful SQL writes, verifying one session touch
per create or replay while retaining authentication, CSRF, and ownership checks.
Concurrent creation regressions hold the user lock while revoking, disabling,
expiring, or resetting the authenticated user. Comparison regressions cover
sparse predecessor seeks, equal-time ordering, retained-history anchors, and
both sides of the 1,000-point event-path threshold. Execution plans are checked
before and after refreshing statistics. Event benchmarks contrast short and
year-long windows over sparse and dense evidence histories.

## Documentation verification

After dependencies are installed with `make deps`, use the root Prettier
configuration for Markdown-only changes:

```sh
web/node_modules/.bin/prettier --write AGENTS.md GLOSSARY.md README.md 'docs/**/*.md'
web/node_modules/.bin/prettier --check AGENTS.md GLOSSARY.md README.md 'docs/**/*.md'
```

Check local paths and heading anchors, including links, image paths, and source
comments affected by renames. Commands and behavioral claims must match code,
configuration, and Makefile. Prose-only changes need no application or database
commands; edits to Go comments still follow the Go validation gates.

## Agent runtime verification

After changes affecting `serve`, the JSON API, or the UI, exercise the changed
flow in the agent dev instance in addition to the validation gates. This
includes seed data, valuation reads, the embedded SPA, and history defaults.
Changes confined to adapter packages, their tests, or fixtures use offline tests
and evidence review instead.

1. Confirm `config/agent.toml`, `config/agent.env`, and `plutus_agent` exist. If
   missing, report the [agent setup](./development.md#agent-setup) needed; never
   substitute another instance.
2. Start `./scripts/agent-wipe-and-serve.sh` in the background, or
   `./scripts/agent-serve.sh` to retain data. Wait for
   `GET http://127.0.0.1:8081/health/live` to succeed.
3. Exercise the changed flow on the embedded SPA at `http://127.0.0.1:8081`
   through interaction; screenshots alone are insufficient. API-only changes can
   use `curl` on affected routes. Do not use :8080 or Vite.
4. Terminate session-started process groups, including child `serve` processes.
   Confirm health is unreachable and :8081 has no listener. Leave :8080 alone.
5. Report exercised paths, observations, gaps or blockers, and process cleanup.

## Adapter delivery reports

For a new or expanded real platform adapter delivery, also run
`make build GOOS=linux GOARCH=amd64` after `make web`. Report four items:
recorded evidence with dates and gaps; independent synthetic test results per
affected platform; agent-environment coverage when applicable; and what was not
covered. Documented or synthetic scenarios must remain distinct from observed
real coverage.

## Test ownership

- Package-local tests own local rules and private implementation seams.
- `internal/integration` owns adapter-to-report-to-read workflows, cross-module
  presentation, and migration continuity. Its harness shares collaborators from
  `app.New`; fast retry scenarios explicitly replace only the collection runner.
- `internal/testsupport` contains reusable database and offline network helpers.
  Do not import it from runtime code or move CLI-used fake/recording code there.
- `web/e2e/support` contains common API routing and generated-type-checked
  session responses. Call `route.fallback()` for requests the scenario does not
  own. The common handler fails unknown requests. Keep scenario financial data,
  failures, and assertions in the suite, including explicit empty responses.

Pure helper tests stay with their implementation. MySQL infrastructure tests use
an external test package to avoid a cycle through `mysqltest`. Browser tests
cover session/cache lifetimes, collection invalidation, and raw write results
without duplicating generated schemas.

Give each scenario one owner and retain its assertions when moving it. Do not
export private production methods to relocate tests. Each database-backed
package needs `mysqltest.RunTestMain`; install `offline.Install` first when
exercising platform clients.

## Test configuration

Test configuration uses `PLUTUS_GOTEST_*` in gitignored `config/gotest.env`
(copy [config/gotest-example.env](../config/gotest-example.env)) or the process
environment, which wins. Currently it contains only the throwaway MySQL DSN.
Recording credentials are configured separately through the
[evidence workflow](./platform/evidence-workflow.md#record-one-interface-prepare-a-group).
Tests never load them; runtime `serve`, the worker, and `collect` load neither
developer configuration source.

- `PLUTUS_GOTEST_DSN` is a server-level MySQL DSN with no database name
  (`user:password@tcp(127.0.0.1:3306)/`). Each test package that needs MySQL
  creates its own `plutus_gotest_<unix>_<hex>` database in `TestMain`, migrates
  it, runs, and drops it. Packages share no database; leftovers older than one
  hour are dropped on the next run. Without the variable, database tests skip
  and `make test` can pass without database coverage. Do not edit `gotest.env`
  to fit tests.
- Real adapter tests select synthetic response files and replay them through
  loopback servers; higher-level application tests can use the deterministic
  fake. The network guard fails forbidden HTTP/HTTPS/WebSocket, proxy and
  redirect attempts independently of report error mapping. Local replay and
  temporary MySQL remain supported.

## Recorded evidence

Recorded responses are reviewed interface evidence, separate from automated
tests. The [evidence workflow](./platform/evidence-workflow.md) owns recording
credentials, exact requests, capture guarantees, sanitation, and promotion. Each
platform's verification document tracks dates and outstanding real gaps;
exchange-app reconciliation remains a separate check.
