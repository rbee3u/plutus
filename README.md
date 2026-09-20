# Plutus

Plutus is a personal, read-only portfolio observation project. It is intended to periodically collect normalized account observations, persist their history, and present balances, positions, valuations, and basic directional exposure through a web frontend.

The domain hierarchy is `Platform -> Identity -> Account`: each main or subordinate identity on a financial platform has its own funding or trading Accounts. A Connection configures access to one Identity, and Identity is distinct from any future Plutus login user.

## Current status

The repository contains a minimal Go/Cobra CLI with a placeholder `demo` command. Collection of Identity Snapshots, persistence, and the web frontend have not been implemented. The domain model and initial product boundaries are documented; technical architecture is the next design stage.

## First-release scope

Initial integrations target these account modes:

| Platform | Account modes |
| --- | --- |
| Binance | Portfolio Margin (PM) |
| OKX | Multi-currency margin (MCM), Portfolio margin (PM) |
| Bybit | Unified Trading Account 2.0 (UTA 2.0) |

The product scope includes spot balances and stablecoin-margined (linear) and coin-margined (inverse) perpetual and delivery contracts. Availability and report coverage must be verified for each selected platform and mode.

Each enabled Identity has at most one enabled Connection. Its Accounts and Platform catalog entries are discovered automatically, while subordinate Identities require explicit enablement. Every reported Balance and Position is retained even before a newly discovered Asset Listing is mapped, and prices are tracked for current holdings, open Positions, and any additional entries pinned by an administrator.

The planned frontend presents Balance and Position tables that can be filtered or grouped by Platform, Identity, Account, Asset, and Instrument. Platform-specific Account Metrics such as UniMMR remain visible at Account scope rather than being treated as cross-platform aggregates.

Portfolio Valuation uses Reported Equity for its declared coverage and sufficient Calculated Valuation for uncovered scopes. One editable Capital Baseline and manually entered external Capital Flows support absolute Value Changes for calendar days, ISO weeks, months, quarters, and years, including provisional period-to-date results. Observation times, failures, stale data, incomplete coverage, and incomparable periods remain visible. Display defaults to USD with a CNY option while preserving original quantities and denominations.

Plutus persists normalized Observations rather than raw Platform response payloads. Latest state, aggregates, valuations, exposure, contributed capital, value changes, and period summaries are rebuildable projections; correcting an Observation or manual source fact invalidates and rebuilds affected projections.

Value Changes are not investment returns. Percentage returns, return attribution, a full transaction ledger, trading execution, and internal-transfer tracking are outside the first release; asynchronous observations around an internal transfer can therefore create a temporary discrepancy. Manual asset management, including manually tracked securities or DeFi assets, is deferred to a second phase.

## Roadmap

1. Establish the domain glossary and initial decisions.
2. Decide the technical architecture; a Go backend with an embedded TypeScript frontend is the current direction under consideration.
3. Build a frontend demo with synthetic data to settle presentation and interactions.
4. Implement backend data integrations and persistence.

Database, API, frontend framework, and deployment choices remain open.

## Documentation

- [Domain glossary](./CONTEXT.md)
- [Architecture decision records](./docs/adr/)
- [Contributor and agent guidance](./AGENTS.md)

## Development

Use the Go toolchain required by `go.mod` and Make. Install the pinned formatting and lint tools with `make deps`, and ensure the Go tool installation directory is on `PATH`.

Local Go development follows four validation gates in order:

1. `make format`: rewrite imports and formatting before validating the code.
2. `make build`: confirm the formatted code compiles and build `bin/plutus`.
3. `make test`: verify unit tests without cached results after the build passes.
4. `make lint`: run the configured linters after tests pass.

Each gate must pass before proceeding to the next. Stop on failure, fix the issue, and repeat the sequence:

```sh
make format && make build && make test && make lint
```

`make style` is intended for future GitHub CI, where it replaces `make format` as the first gate. It checks formatting without rewriting files and fails when formatting corrections are needed; build, tests, and lint follow in the same order:

```sh
make style && make build && make test && make lint
```

Use `make help` to list commands in their Makefile order. Run `make run` for local execution through `go run .`, without a separate build step; it currently displays CLI help. Pass CLI arguments with `ARGS`, for example `make run ARGS=demo` to execute the existing placeholder command; this is not the planned frontend demo.
