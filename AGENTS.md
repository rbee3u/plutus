# Repository Guidance

## Language and domain

- Write all code, comments, documentation, and user-facing application text in English.
- Read [CONTEXT.md](./CONTEXT.md) and the relevant [ADRs](./docs/adr/) before changing domain behavior. Use the canonical terms consistently.
- Keep `CONTEXT.md` a glossary: concise definitions only, without implementation details, requirements, or roadmap notes.
- Record durable decisions with meaningful trade-offs in short, sequentially numbered ADRs. Update their status and references when a decision is superseded.
- Follow the accepted scope and valuation decisions in ADRs 0014–0021: one Valuation Point per Collection Cycle, User-selected Account Types, Boundary Events as implicit Capital Flows, USD-only valuation with USDT/USDC at 1 USD, symbol-based Asset merging, Account-level stale carry-forward, manual Capital Flows with read-only keys, and one aligned Account Report. Do not silently expand supported Account Structures, Margin Modes, or Account Types. The declared Account Structure and the observed Margin Mode follow [ADR 0028](./docs/adr/0028-separate-account-structure-from-margin-mode.md); dual investment Accounts, Dual Positions, auxiliary completeness reads, and report scope transitions follow [ADR 0033](./docs/adr/0033-separate-dual-investment-accounts-and-positions.md). Kraken Separate wallets, Unified Wallet, and its two-key credential entry follow [ADR 0026](./docs/adr/0026-support-kraken-separate-and-unified-wallets.md). Documentation-derived scenarios without live coverage follow [ADR 0027](./docs/adr/0027-allow-documented-adapter-scenarios-without-live-coverage.md).
- Unknown value is never written as zero and never dropped silently: a total is the sum of what could be valued, marked `incomplete`, with the omitted items named. Only a successful Account Report may make a coin disappear (the Complete Set rule).
- Adapters may call only read-only balance, position, account, and market-data endpoints. At runtime (`serve`, the worker, `collect`) raw Platform payloads never cross the adapter boundary, are never logged, and are never stored. The only place a raw response may be written to disk is the `plutus adapter record` fixture recorder described in [ADR 0023](./docs/adr/0023-test-platform-adapters-in-three-layers.md), and only into gitignored `tmp/`.

## Environments

[ADR 0022](./docs/adr/0022-separate-dev-test-and-prod-environments.md) defines them. Automated tests are not an environment.

| Environment | Where | Adapter | Config | Reset |
| --- | --- | --- | --- | --- |
| dev `local` (the human, :8080, `plutus_local`) | this machine, `go run` | fake | `config/local.toml` + `local.env` | `scripts/local-wipe-and-serve.sh`; `local-serve.sh` keeps data |
| dev `agent` (the AI agent, :8081, `plutus_agent`) | this machine, `go run` | fake | `config/agent.toml` + `agent.env` | `scripts/agent-wipe-and-serve.sh`; `agent-serve.sh` keeps data |
| test (:8082, `plutus_test`) | remote server, `make dist` binary | live read-only keys | hand-edited `test.toml` + `test.env` on the server | `plutus db reset` (no profile), by the owner |
| prod (:8083, `plutus_prod`) | remote server, `make dist` binary | live read-only keys | hand-edited `prod.toml` + `prod.env` on the server | never |

- Never point a dev instance at real keys, and never register the fake in test or prod. Never connect to `plutus_local`, `plutus_test`, or `plutus_prod` from an agent session; the agent's databases are `plutus_agent` and the throwaway `plutus_gotest_*` ones.
- `go test` reads `PLUTUS_GOTEST_*` from gitignored `config/gotest.env` (copy `config/gotest-example.env`). `PLUTUS_GOTEST_DSN` is a server-level DSN; each database-backed test package creates and drops its own `plutus_gotest_<unix>_<hex>` database through `mysql.RunTestMain`. A package that adds database tests must add `func TestMain(m *testing.M) { mysql.RunTestMain(m) }`. Never edit `config/gotest.env` to fit a test.
- `PLUTUS_GOTEST_<PLATFORM>_APIKEY` / `_SECRET` / `_PASSPHRASE` in the same file are the read-only keys for `make test-live` and `plutus adapter record`. They are the test environment's key set; prod's set never appears on a development machine.
- Once prod has run `migrate up` for the first time, the baseline migration is frozen and every schema change is an appended migration that works on populated databases ([ADR 0024](./docs/adr/0024-freeze-the-baseline-migration-once-prod-has-migrated.md)).

## Changes and documentation

- Keep changes focused on the requested stage of work and consistent with existing repository conventions.
- Keep [README.md](./README.md) accurate about implemented capabilities, working commands, and planned work. A proposed architecture or synthetic-data demo is not a completed integration. Adapter order is Binance, OKX, Bybit, then Kraken; each is a separate delivery.
- Update affected terminology and decision records when behavior changes. Keep design preferences distinct from accepted decisions.
- `CLAUDE.md` is a relative symlink to this file. Maintain one shared set of instructions by editing `AGENTS.md`.

## Validation

- For local Go code changes, run `make format && make build && make test && make lint`. These are successive gates: format the code, confirm compilation, verify unit tests, then check lint. Each gate must pass before the next starts; fix failures and repeat the sequence. Install the pinned tools with `make deps` when needed. `make test` must stay offline: no live keys, no network, no pre-created database.
- Reserve `make style` for future GitHub CI as the non-mutating replacement for `make format`. The CI sequence is `make style && make build && make test && make lint`; formatting failures must be corrected locally with `make format`.
- For documentation-only changes, verify terminology, stated behavior, relative links, and any symlink targets. Domain changes should also be checked against concrete identity, account, valuation, and data-failure scenarios.
- Platform adapter tests have three layers ([ADR 0023](./docs/adr/0023-test-platform-adapters-in-three-layers.md)): `httptest` replay of committed fixtures in `internal/adapters/<platform>/testdata/` (in `make test`), the fake adapter for collection and valuation rules (in `make test`), and live smoke tests behind the `live` build tag (`make test-live` only, structure not numbers).
- `make test-live` sends real requests to exchanges and consumes their rate limits. It is not a routine validation gate. Run it only when the current task changes a real Platform adapter's runtime request, authentication, endpoint selection, response mapping, or Account Report behavior, or changes that Platform's live smoke test. Use `make test-live PLATFORM=<platform>` once for each affected Platform; for shared adapter runtime changes, select each affected Platform separately. Repeat only after a fix that could change the live result. Do not run an unqualified full sweep unless the user explicitly requests one. UI, API, collection, valuation, fake adapter, recorder, fixture, offline adapter test, and documentation-only changes do not trigger live tests by themselves. Unrelated staged or unstaged work already in the checkout does not expand the current task's live-test scope. If a required selected Platform lacks keys or is unreachable, report that limitation instead of substituting a full sweep.
- Fixtures come from `plutus adapter record` into gitignored `tmp/`, trimmed to the per-endpoint symbol whitelist and redacted. Do not commit a fixture the owner has not reviewed. Each fixture starts with a header naming Platform, endpoint, Account Structure, Margin Mode where the structure has one, and recording date. A documentation-derived fixture ([ADR 0027](./docs/adr/0027-allow-documented-adapter-scenarios-without-live-coverage.md)) is labeled `documented`, names its documentation page instead of a recording date, and lives apart from the recordings.
- After any change that can affect `serve`, the JSON API, or the UI (including seeds, valuation reads, the embedded SPA, and history defaults), the make gates are not enough. Changes confined to adapter packages, their tests, or fixtures do not need this step; verify them with the applicable offline tests and the live-test rule above. Otherwise start the agent dev environment and exercise the changed flow before claiming the work is done:
  1. Run `./scripts/agent-wipe-and-serve.sh` in the background (or `agent-serve.sh` to keep data). Wait until `GET http://127.0.0.1:8081/health/live` succeeds.
  2. Open `http://127.0.0.1:8081` (not `:8080` / Vite). Follow the changed path as a real user: click, type, submit, and navigate. For API-only changes, `curl` the affected routes on `:8081`. A single screenshot is not verification.
  3. If `config/agent.toml`, `config/agent.env`, or MySQL database `plutus_agent` is missing, stop and say how to copy `config/agent-example.toml` and `config/agent-example.env` and `CREATE DATABASE plutus_agent`. Do not connect to `plutus_local`, and do not skip this check.
  4. In the final report, state what was exercised on `:8081`, what was observed, and what was not covered. Do not call UI or runtime behavior complete without this check.
- For a new or expanded real Platform adapter delivery, also run `make dist` once so a new dependency cannot break the cross-build, and structure the final report in four parts: fixture provenance and recording date, `make test-live` result per affected Platform, agent-environment coverage, and what was not covered.
- Stop every process this session started before finishing, including the agent scripts, `go run`, and Vite if this session launched them. Kill the process group so child `serve` processes exit too. Confirm the agent environment is down (`GET http://127.0.0.1:8081/health/live` fails). Do not leave `:8081` occupied, and do not kill a `:8080` process that the user started. Say in the final report that the started processes were stopped.
- Review the final diff for unintended changes and report the checks performed and any remaining limitations.
