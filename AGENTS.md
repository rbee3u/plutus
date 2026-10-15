# Agent guidance

## Working rules

- Write code, comments, documentation, and application text in English. Wrap
  Markdown prose near 80 columns; preserve tables, code blocks, and links.
  Follow the shared [capitalization guide](./docs/capitalization.md).
- Prefer deletion or reuse to new files and abstractions. Keep one source per
  rule; link instead of repeating it. Document constraints, non-obvious intent,
  trade-offs, and evidence gaps; omit common knowledge and code paraphrases.
- Put task and tool temporary artifacts under `/tmp/plutus`, never in a
  repository-local `tmp/` directory. Use unique subdirectories for concurrent
  work and clean up disposable files when finished.
- Do not change the Git index unless explicitly asked. `CLAUDE.md` is a relative
  symlink to this file; edit `AGENTS.md` as the shared source.
- Before changing behavior, read the [glossary](./GLOSSARY.md),
  [domain rules](./docs/domain-rules.md),
  [product scope](./docs/product-scope.md), and relevant
  [ADRs](./docs/adr/README.md). Use canonical terms and preserve supported
  scope. For UI changes, also read [presentation](./docs/presentation.md).
  Maintain decisions under the [ADR rules](./docs/adr/README.md#maintenance).
- Keep [README status](./README.md#current-status) honest. Synthetic tests and
  recorded evidence do not establish exchange-app validation. The glossary owns
  terms, domain rules own business behavior, presentation owns display and
  interaction rules, [architecture](./docs/architecture.md) owns technical
  mechanisms, and ADRs own decisions and trade-offs. Keep each rule in its owner
  and link to it.

## Domain and data safeguards

- Unknown value is never zero or silently omitted. Sum known value, mark
  affected totals incomplete, and name omissions. Only a successful account
  report removes holdings; failures use compatible successful history as stale.
- Account structure and margin mode are distinct. Earn and dual investment are
  separate selected accounts with shared completeness reads and scope-aware
  stale fallback. Follow the [domain rules](./docs/domain-rules.md).
- Adapters call only read-only balance, position, account, and market-data
  endpoints. Runtime raw platform payloads never cross the adapter boundary,
  enter logs, or reach storage. Only explicit `plutus adapter record` may write
  sanitized responses to `/tmp/plutus/` for review.
- Follow [migration safeguards](./docs/deployment.md#upgrades-and-resets):
  freeze the baseline after prod's first migration; append populated-database
  upgrades.

## Environment boundaries

- Follow [environments](./docs/development.md#environments): dev uses the fake
  without live keys; remote test and prod use live read-only keys without fake.
- Agent sessions may connect only to `plutus_agent` on :8081 and throwaway
  `plutus_gotest_*` databases, never `plutus_local`, `plutus_test`, or
  `plutus_prod`. Use [agent setup](./docs/development.md#agent-setup), never the
  human's :8080 instance.
- Follow [test configuration](./docs/testing.md#test-configuration); do not edit
  `gotest.env` to fit tests. Every database-backed package needs
  `mysqltest.RunTestMain` for its temporary database.
- Only explicit recording loads `config/record.env` / `PLUTUS_RECORD_*`; tests
  and runtime commands never load them. Never use test or runtime credentials as
  a recording fallback. Prod keys never belong on a development machine.

## Verification

- For all Go verification, including golangci-lint and `make` targets, reuse the
  host's existing cache configuration without overrides or persistent changes.
  Request elevated execution when sandbox restrictions are expected or
  encountered.
- Go changes, including comments, must pass the ordered
  [validation gates](./docs/testing.md#validation-gates). Tests allow only
  loopback replay and temporary MySQL, never external requests, live keys, or
  pre-created databases.
- New or expanded real adapter deliveries also require a Linux amd64 build and
  an [adapter delivery report](./docs/testing.md#adapter-delivery-reports).
  Synthetic `testdata/` and recorded `evidence/` are independent; follow the
  [evidence workflow](./docs/platform/evidence-workflow.md).
- Changes affecting `serve`, the JSON API, or the UI require
  [agent runtime verification](./docs/testing.md#agent-runtime-verification) on
  :8081. Changes confined to adapters, tests, or fixtures use offline
  verification and evidence review.
- Markdown-only changes require
  [documentation verification](./docs/testing.md#documentation-verification).
