# Agent guidance

## Working rules

- Write code, comments, documentation, and application text in English. Wrap
  Markdown prose near 80 columns; preserve useful content, tables, code blocks,
  and indivisible links.
- Put task and tool temporary artifacts under `/tmp/plutus`, never in a
  repository-local `tmp/` directory. Use unique subdirectories for concurrent
  work and clean up disposable files when finished.
- Do not change the Git index unless explicitly asked. `CLAUDE.md` is a relative
  symlink to this file; edit `AGENTS.md` as the shared source.
- Before changing behavior, read the [domain model](./CONTEXT.md),
  [product scope](./docs/product-scope.md), and relevant
  [ADRs](./docs/adr/README.md). Use canonical terms; do not silently expand
  supported Account Structures, Margin Modes, or Account Types. Record durable
  decisions and meaningful trade-offs in the relevant ADR, distinguishing
  preferences from accepted choices.
- Keep [README status](./README.md#current-status) honest. Synthetic tests and
  recorded evidence do not establish exchange-app validation or a completed
  integration. Technical mechanisms belong in
  [architecture](./docs/architecture.md); domain terms and rules belong in
  `CONTEXT.md`.

## Domain and data safeguards

- Unknown value is never zero or silently omitted. Sum known value, mark
  affected totals incomplete, and name omissions. Only a successful Account
  Report removes holdings; failures use compatible successful history as stale.
- Account Structure and Margin Mode are distinct. Earn and Dual Investment are
  separate selected Accounts with shared completeness reads and scope-aware
  stale fallback. Follow the [domain model](./CONTEXT.md).
- Adapters call only read-only balance, position, account, and market-data
  endpoints. Runtime raw Platform payloads never cross the Adapter boundary,
  enter logs, or reach storage. Only explicit `plutus adapter record` may write
  sanitized responses to `/tmp/plutus/` for review.
- After prod's first `migrate up`, freeze the baseline and append migrations
  that work on populated databases. Follow [deployment](./docs/deployment.md).

## Environment boundaries

- Follow [environments](./docs/development.md#environments): dev uses the fake
  without live keys; remote test and prod use live read-only keys without fake.
- Agent sessions may connect only to `plutus_agent` on :8081 and throwaway
  `plutus_gotest_*` databases, never `plutus_local`, `plutus_test`, or
  `plutus_prod`. Use [agent setup](./docs/development.md#agent-setup), never the
  human's :8080 instance.
- Tests use `config/gotest.env` and per-package temporary databases through
  `mysql.RunTestMain`; new database-backed packages need that `TestMain`. Do not
  edit `gotest.env` to fit tests. See
  [test configuration](./docs/testing.md#test-configuration).
- Only explicit recording loads `config/record.env` / `PLUTUS_RECORD_*`; tests
  and runtime commands never load them. Never use test or runtime credentials as
  a recording fallback. Prod keys never belong on a development machine.

## Verification

- For all Go verification, including golangci-lint and `make` targets, reuse the
  host's existing cache configuration without overrides or persistent changes.
  Request elevated execution when sandbox restrictions are expected or
  encountered.
- Go changes must pass the
  [validation gates](./docs/testing.md#validation-gates) in order:
  `make format && make build && make test && make lint`. Fix failures and repeat
  the sequence. Tests allow loopback replay and temporary MySQL, never external
  requests, live keys, or pre-created databases.
- New or expanded real Adapter deliveries also require a Linux amd64 build and
  an [Adapter delivery report](./docs/testing.md#adapter-delivery-reports).
  Synthetic `testdata/` and recorded `evidence/` are independent; follow the
  [evidence workflow](./docs/adapters/evidence-workflow.md).
- Changes affecting `serve`, the JSON API, or the UI require
  [agent runtime verification](./docs/testing.md#agent-runtime-verification) on
  :8081. Changes confined to Adapters, tests, or fixtures use offline
  verification and evidence review.
- Markdown-only changes require
  [documentation verification](./docs/testing.md#documentation-verification).
