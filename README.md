# Plutus

Plutus is a personal, read-only portfolio observation tool. It collects exchange
balances, positions, and account reports, values them in USD, and shows how the
total changed over time after excluding the owner's transfers in and out.

A User owns one or more Portfolios. Each Portfolio organizes exchange accounts
as `Platform -> Identity -> Account`; each Identity represents one exchange
login with its own read-only credentials.

## Current status

The portfolio application is implemented end to end with a deterministic fake
Adapter. Live Binance, OKX, Bybit, and Kraken Adapters are also implemented, but
**none has been validated against its exchange app on the test environment**. No
exchange integration is complete yet.

Implemented capabilities include:

- Session authentication, multiple Portfolios, and Portfolio settings.
- Identity and Account selection, encrypted credentials, and Connection
  validation, enabling, disabling, and archiving.
- Scheduled and on-demand collection, USD valuation, stale report carry-forward,
  and explicit incomplete totals when some holdings cannot be valued.
- Unified rotating application, HTTP access, and full-value SQL logs; see
  [logging configuration](./docs/development.md#logging).
- Holdings, historical Valuation Points, Period Closes, and Value Changes, with
  manual Capital Flows and derived Boundary Events.
- Asset mapping, reviewed Portfolio-specific Balance exclusions, Activity and
  audit views, and deterministic development seed profiles.

Remaining work is exchange-app reconciliation and validation of unexercised
Account Structures, Margin Modes, and products. Synthetic tests and reviewed
recordings provide separate evidence; neither completes an integration. See the
Platform coverage notes: [Binance](./docs/adapters/binance/verification.md),
[OKX](./docs/adapters/okx/verification.md),
[Bybit](./docs/adapters/bybit/verification.md), and
[Kraken](./docs/adapters/kraken/verification.md).

The implementation also has an unresolved
[same-Platform pricing discrepancy](./docs/architecture.md#known-implementation-discrepancy).

## Product scope

Plutus answers two questions: **how much is there now, and where**, and **how
did the total change over a period, excluding my own transfers**.

Only selected Accounts are observed. Unknown value stays explicit and totals can
be stale or incomplete. USD is the only valuation currency; USDT and USDC count
as 1 USD. Plutus uses read-only API keys. Trading, transfers, trade history,
automatic deposit import, percentage returns, attribution, non-USD display, and
Exposure aggregation are outside the product. Value Change is not an investment
return.

See [product scope](./docs/product-scope.md) for supported Accounts and
[domain model](./CONTEXT.md) for business rules.

## Quick start

Use the Go toolchain required by [go.mod](./go.mod), Node.js required by
[web/package.json](./web/package.json), Make, and MySQL. Install the pinned
tools and dependencies, with the Go tool installation directory on `PATH`:

```sh
make deps
cp config/local-example.toml config/local.toml
cp config/local-example.env config/local.env
```

For the human's local instance, create MySQL database `plutus_local`, then set
`PLUTUS_DSN` and a unique `PLUTUS_MASTER_KEY` in `config/local.env`. Generate
the master key with `openssl rand -base64 32`. Dev uses the fake Adapter without
real exchange keys. Agent sessions use the separate
[agent setup](./docs/development.md#agent-setup) on :8081.

For a fresh local walkthrough, the following command resets `plutus_local`,
migrates it, loads the seed data, builds the frontend, and starts the service:

```sh
./scripts/local-wipe-and-serve.sh
```

Open [localhost:8080](http://127.0.0.1:8080) and sign in with `alice` /
`Alice=123456`. On subsequent runs, `./scripts/local-serve.sh` keeps the
existing data. Public registration is unavailable; create other Users with
`plutus user create`.

See [development](./docs/development.md) for complete setup, configuration,
seeds, and CLI commands, and [deployment](./docs/deployment.md) for remote test
and prod environments and upgrades.

## Documentation

Plutus is a contract-first Go monolith with a versioned JSON API, embedded React
frontend, and externally managed MySQL database.

| Read to                                   | Document                                                                                                                                                                              |
| ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Check supported products and boundaries   | [Product scope](./docs/product-scope.md)                                                                                                                                              |
| Understand terminology and business rules | [Domain model](./CONTEXT.md)                                                                                                                                                          |
| Understand the system and its decisions   | [Architecture](./docs/architecture.md), [ADRs](./docs/adr/README.md)                                                                                                                  |
| Set up and develop                        | [Development](./docs/development.md)                                                                                                                                                  |
| Verify a change                           | [Testing](./docs/testing.md)                                                                                                                                                          |
| Deploy and upgrade                        | [Deployment](./docs/deployment.md)                                                                                                                                                    |
| Maintain a Platform integration           | Mapping: [Binance](./docs/adapters/binance/mapping.md), [OKX](./docs/adapters/okx/mapping.md), [Bybit](./docs/adapters/bybit/mapping.md), [Kraken](./docs/adapters/kraken/mapping.md) |
| Record and review real responses          | [Evidence workflow](./docs/adapters/evidence-workflow.md); per-Platform verification links above                                                                                      |
| Work as an agent                          | [Agent guidance](./AGENTS.md)                                                                                                                                                         |
