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

Implemented capabilities:

- Authentication, multiple Portfolios, selected Accounts, and encrypted
  read-only Connections.
- Scheduled and on-demand collection, USD valuation, compatible stale history,
  and explicit incomplete totals.
- Overview ownership Tree; Balances, Futures, Earn, and Dual Investment tables;
  Manual balances for API-uncovered quantities.
- Historical points, calendar comparisons, manual Capital Flows, Asset mapping,
  Asset Exclusions, and audit history.
- Rotating application, access, and SQL logs, plus a deterministic
  [development dataset](./docs/seeds.md).

Remaining work is exchange-app reconciliation and unexercised structures, modes,
and products. Synthetic tests and recordings are separate evidence. Coverage:
[Binance](./docs/platform/binance/verification.md),
[OKX](./docs/platform/okx/verification.md),
[Bybit](./docs/platform/bybit/verification.md), and
[Kraken](./docs/platform/kraken/verification.md).

Kraken Dual Investment remains unsupported; Manual can record API-uncovered
principal. See the
[contract review and observed omission](./docs/platform/kraken/verification.md#dual-investment-contract-review).

The prelaunch database baseline separates immutable observations from derived
valuations. Summaries remain permanent; detailed snapshots keep 30 days and
daily archives with pinned dependencies; Collection Cycles remain with them. See
[the schema decision](./docs/adr/0015-prelaunch-schema.md) and
[reset requirements](./docs/deployment.md#upgrades-and-resets).

## Product scope

Only selected Accounts are observed. USD is the valuation currency; USDT and
USDC count as 1 USD. Value Change accounts for the owner's transfers and is not
an investment return. Trading, transfers, and trade-history accounting are out
of scope.

See [product scope](./docs/product-scope.md) for supported Accounts and
[domain rules](./docs/domain-rules.md) for business rules. The
[glossary](./GLOSSARY.md) defines the canonical terms.

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

For a fresh local walkthrough, the following command builds the frontend once,
resets `plutus_local`, migrates it, loads the seed data, and starts the service:

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
| Understand terminology and business rules | [Glossary](./GLOSSARY.md), [Domain rules](./docs/domain-rules.md)                                                                                                                     |
| Understand display and interaction rules  | [Presentation](./docs/presentation.md)                                                                                                                                                |
| Understand the system and its decisions   | [Architecture](./docs/architecture.md), [ADRs](./docs/adr/README.md)                                                                                                                  |
| Set up and develop                        | [Development](./docs/development.md)                                                                                                                                                  |
| Verify a change                           | [Testing](./docs/testing.md)                                                                                                                                                          |
| Deploy and upgrade                        | [Deployment](./docs/deployment.md)                                                                                                                                                    |
| Maintain a Platform integration           | Mapping: [Binance](./docs/platform/binance/mapping.md), [OKX](./docs/platform/okx/mapping.md), [Bybit](./docs/platform/bybit/mapping.md), [Kraken](./docs/platform/kraken/mapping.md) |
| Record and review real responses          | [Evidence workflow](./docs/platform/evidence-workflow.md); per-Platform verification links above                                                                                      |
| Work as an agent                          | [Agent guidance](./AGENTS.md)                                                                                                                                                         |

Feature delivery reports:
[Positions](./docs/platform/position-presentation-delivery.md),
[Margin](./docs/platform/margin-presentation-delivery.md),
[Earn](./docs/platform/earn-presentation-delivery.md),
[Manual balances](./docs/platform/manual-balances-delivery.md), and
[Proxy](./docs/platform/proxy-verification.md).
