# Repository Guidance

## Language and domain

- Write all code, comments, documentation, and user-facing application text in English.
- Read [CONTEXT.md](./CONTEXT.md) and the relevant [ADRs](./docs/adr/) before changing domain behavior. Use the canonical terms consistently.
- Keep `CONTEXT.md` a glossary: concise definitions only, without implementation details, requirements, or roadmap notes.
- Record durable decisions with meaningful trade-offs in short, sequentially numbered ADRs. Update their status and references when a decision is superseded.
- Follow accepted scope and valuation decisions, including distinctions between equity, exposure, report coverage, and observation time. Do not silently expand supported account modes or treat missing data as zero.

## Changes and documentation

- Keep changes focused on the requested stage of work and consistent with existing repository conventions.
- Keep [README.md](./README.md) accurate about implemented capabilities, working commands, and planned work. A proposed architecture or synthetic-data demo is not a completed integration.
- Update affected terminology and decision records when behavior changes. Keep design preferences distinct from accepted decisions.
- `CLAUDE.md` is a relative symlink to this file. Maintain one shared set of instructions by editing `AGENTS.md`.

## Validation

- For local Go code changes, run `make format && make build && make test && make lint`. These are successive gates: format the code, confirm compilation, verify unit tests, then check lint. Each gate must pass before the next starts; fix failures and repeat the sequence. Install the pinned tools with `make deps` when needed.
- Reserve `make style` for future GitHub CI as the non-mutating replacement for `make format`. The CI sequence is `make style && make build && make test && make lint`; formatting failures must be corrected locally with `make format`.
- For documentation-only changes, verify terminology, stated behavior, relative links, and any symlink targets. Domain changes should also be checked against concrete identity, account, valuation, and data-failure scenarios.
- Review the final diff for unintended changes and report the checks performed and any remaining limitations.
