# Repository Guidance

## Language and domain

- Write all code, comments, documentation, and user-facing application text in English.
- Read [CONTEXT.md](./CONTEXT.md) and the relevant [ADRs](./docs/adr/) before changing domain behavior. Use the canonical terms consistently.
- Keep `CONTEXT.md` a glossary: concise definitions only, without implementation details, requirements, or roadmap notes.
- Record durable decisions with meaningful trade-offs in short, sequentially numbered ADRs. Update their status and references when a decision is superseded.
- Follow the accepted scope and valuation decisions in ADRs 0014–0021: one Valuation Point per Collection Cycle, User-selected Account Types, Boundary Events as implicit Capital Flows, USD-only valuation with USDT/USDC at 1 USD, symbol-based Asset merging, Account-level stale carry-forward, manual Capital Flows with read-only keys, and one aligned Account Report. Do not silently expand supported Account Modes or Account Types.
- Unknown value is never written as zero and never dropped silently: a total is the sum of what could be valued, marked `incomplete`, with the omitted items named. Only a successful Account Report may make a coin disappear (the Complete Set rule).
- Adapters may call only read-only balance, position, account, and market-data endpoints. Raw Platform payloads never cross the adapter boundary, are never logged, and are never stored.

## Changes and documentation

- Keep changes focused on the requested stage of work and consistent with existing repository conventions.
- Keep [README.md](./README.md) accurate about implemented capabilities, working commands, and planned work. A proposed architecture or synthetic-data demo is not a completed integration.
- Update affected terminology and decision records when behavior changes. Keep design preferences distinct from accepted decisions.
- `CLAUDE.md` is a relative symlink to this file. Maintain one shared set of instructions by editing `AGENTS.md`.

## Validation

- For local Go code changes, run `make format && make build && make test && make lint`. These are successive gates: format the code, confirm compilation, verify unit tests, then check lint. Each gate must pass before the next starts; fix failures and repeat the sequence. Install the pinned tools with `make deps` when needed.
- Reserve `make style` for future GitHub CI as the non-mutating replacement for `make format`. The CI sequence is `make style && make build && make test && make lint`; formatting failures must be corrected locally with `make format`.
- For documentation-only changes, verify terminology, stated behavior, relative links, and any symlink targets. Domain changes should also be checked against concrete identity, account, valuation, and data-failure scenarios.
- After any change that can affect `serve`, the JSON API, or the UI (including seeds, projections, the embedded SPA, and history defaults), the make gates are not enough. Start the isolated Agent lane and exercise the changed flow before claiming the work is done:
  1. Run `./scripts/agent-wipe-and-serve.sh` in the background. Wait until `GET http://127.0.0.1:8081/health/live` succeeds.
  2. Open `http://127.0.0.1:8081` (not local `:8080` / Vite, and not human walkthrough `:8082`). Follow the changed path as a real user: click, type, submit, and navigate. For API-only changes, `curl` the affected routes on `:8081`. A single screenshot is not verification.
  3. If `config/agent.toml`, `config/agent.env`, or MySQL database `plutus_agent` is missing, stop and say how to copy `config/agent-example.toml` and `config/agent-example.env` and `CREATE DATABASE plutus_agent`. Do not connect to `plutus_local` or `plutus_walk`, and do not skip this check.
  4. In the final report, state what was exercised on `:8081`, what was observed, and what was not covered. Do not call UI or runtime behavior complete without this check.
- Stop every process this session started before finishing, including `./scripts/agent-wipe-and-serve.sh`, other wipe/serve scripts, `go run`, and Vite if this session launched them. Kill the process group so child `serve` processes exit too. Confirm the Agent lane is down (`GET http://127.0.0.1:8081/health/live` fails). Do not leave `:8081` occupied, and do not kill a local `:8080` or walkthrough `:8082` process that the user started. Say in the final report that the started processes were stopped.
- Review the final diff for unintended changes and report the checks performed and any remaining limitations.
