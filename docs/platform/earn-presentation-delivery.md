# Earn presentation delivery

This delivery adds normalized Earn descriptions for existing supported products,
Portfolio and Account reads, an Earn Holdings view, shared Overview details, and
row-local Observed health for Earn and Dual Investment. It adds no product
Account Types or exchange-app validation. See
[ADR 0005](../adr/0005-earn-and-dual-products.md).

## Evidence and limits

No live credentials were loaded and no recording was taken for this delivery.
Existing accepted evidence remains separate from synthetic replay coverage.

| Platform | Existing Earn evidence                                                        | Outstanding evidence                                                                                                                                                              |
| -------- | ----------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Binance  | Flexible and locked responses, recorded 2026-09-30; owner-reviewed 2026-10-01 | Locked API APY versus Help APR and reported reward scope require conservative labels; independent subscriptions, tier changes, and redemption transitions need app reconciliation |
| OKX      | Savings and active on-chain orders, recorded 2026-10-01                       | Multi-Asset combinations and changing reward/redemption states have synthetic coverage, not app validation; minimum lending rate is a preference                                  |
| Bybit    | Flexible positions, recorded 2026-10-01                                       | Fixed-Term positions and the optional public product catalogue have synthetic coverage only; no new recorded product-APR response                                                 |
| Kraken   | Strategies and allocations, recorded 2026-10-01                               | Strategy/bucket history is not an individual subscription ledger; unified maturity and subscription times remain unknown                                                          |

Synthetic tests cover four-Platform detail mapping, optional field faults, rate
units/ranges/tiers, multi-Asset products and reward currencies, independent
fixed positions, product phases, and optional Bybit catalogue failures. Database
tests cover populated migration, old summaries, compatible stale source
alignment, successful empty reports, filtering before pagination, Portfolio
isolation, and unchanged valuation/rebuild results. UI tests cover columns,
qualified values, Observed health, legacy summaries, and accessible product
disclosures.

## Historical validation

This 2026-10-05 record covers the previous baseline. Legacy summary and
populated-migration cases were retired by the
[prelaunch schema redesign](../adr/0015-prelaunch-schema.md).

Verified on 2026-10-05:

- The ordered gates passed: `make format`, `make web`, `make build`,
  `make test`, `make lint`. All Go packages passed with throwaway MySQL
  databases; 88 Vitest tests passed; Go lint reported zero issues and
  TypeScript/ESLint passed.
- All 42 Chromium checks passed using mocked APIs and a loopback Vite server.
  Earn coverage includes multi-Asset rows, touch and keyboard disclosures,
  independent filter memory, old Balance summaries, shared Overview columns,
  source-report mismatch rejection, and mobile containment. Existing mock
  fixtures and assertions were aligned with current navigation, numeric display,
  and history windows.
- `make generate` left SQL and Go/TypeScript API outputs byte-for-byte
  unchanged. `make build GOOS=linux GOARCH=amd64` passed after the embedded SPA
  build.
- The fake-backed `plutus_agent` instance on `127.0.0.1:8081` was seeded and
  exercised through sign-in, Holdings navigation, Earn's legacy Balance summary,
  manual Collection, new flexible and fixed detail rows, Asset search, tab
  memory, reload, product disclosures, Overview Earn expansion, and Dual
  Rate/Observed. Source times used the Portfolio's Asia/Shanghai timezone. The
  final binary was restarted with the same agent data for the last runtime
  check.

The agent process groups were stopped after verification; :8081 no longer had a
listener. No :8080 process or human database was used. This runtime covers the
fake and embedded SPA, not real Platform calls or exchange-app reconciliation.
Native mobile-browser refresh and real product transitions remain separate
validation gaps.

## Current baseline

Fresh installations use the consolidated baseline. The previous incremental Earn
migration and missing-description compatibility flag have been removed. Follow
the [prelaunch reset procedure](../deployment.md#upgrades-and-resets); current
validation is recorded in the
[schema delivery report](./schema-provenance-delivery.md).
