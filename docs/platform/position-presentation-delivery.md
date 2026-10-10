# Position presentation delivery: 2026-10-03

This delivery implements the accepted
[contract presentation](../presentation.md#futures): shared contract columns,
normalized Size, margin provenance, historical source context, and Observed
badges. The existing business ordering remains in use. No exchange integration
is declared complete.

The subsequent [margin correction](./margin-presentation-delivery.md) supersedes
this delivery's blanket Binance PM suppression and the original OKX USD unit
assumption. Those initial synthetic assertions did not establish the correct
exchange-app margin meaning. Original validation results below describe this
delivery; the follow-up report records the corrected behavior and new checks.

## Automated verification

The ordered gates passed on 2026-10-03:
`make format && make web && make build && make test && make lint`. This includes
all Go packages, throwaway MySQL integration tests, five Vitest files with ten
tests, TypeScript checking, ESLint, and golangci-lint. A separate Linux amd64
build passed. Regenerating SQL and API clients left generated file contents
unchanged.

Independent synthetic Adapter coverage passed for each changed Platform:

| Platform | Additional coverage                                                                                                                                                                                       |
| -------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Binance  | One-way/hedged mode, Cross initial versus Isolated wallet amounts, no PM allocation, exact COIN-M Symbol/side/quantity join, ambiguous or changed-size rejection, actual Account isolated-wallet fallback |
| OKX      | Net/hedged mode, Cross `imr` in USD versus isolated native `margin`, PM cross risk suppression, normalized mode spelling                                                                                  |
| Bybit    | One-way/hedged index and direction validation, missing-index rejection, Cross initial provenance, isolated initial requirement kept distinct from allocation, PM amounts absent                           |
| Kraken   | One-way mode, returned isolated allocation in USD from `initial_margin_without_orders`, no addition of PnL, absent versus empty isolated map, no Cross wallet allocation                                  |

Shared calculation tests cover all four contract kinds, native versus contract
quantity units, no repeated Contract Size conversion, Both sign, hedged positive
Size, missing/zero specifications, and inverse USD face notional without Mark.
Margin tests cover units, stablecoin equivalence, valid and unavailable
USD-to-Base conversion, initial versus allocated basis, and Portfolio risk.

Populated-database migration tests cover exact historical references across all
four Platforms, case mismatch and contradictory direction, unchanged financial
source amounts, and unknown historical Account Margin Mode. Collection tests
cover historical point scoping, old observation times, stale source regime,
known totals before filtering/pagination, missing linear valuation, missing
Accounts without rows, and Balance omissions independent of contract notional.
Existing ordering tests verify stable pagination.

The two Chromium tests passed against loopback Vite with mocked API responses.
Position checks cover 13-column order, hover/focus/click/Escape, source
precision, margin conversion disclosure, stale/core badges, Delivery timezone,
and a 390-pixel viewport with horizontal table scrolling. These are synthetic
browser checks, not exchange-app validation.

## Agent runtime coverage

The additive migration ran against the existing `plutus_agent` database. The
embedded fake-backed SPA was exercised on :8081 with its data preserved:

- Global Futures table: the agreed 13 columns, Base/Quote Size, Both/Long/Short,
  native settlement amounts, Portfolio badges, Report times, and stable order.
- Holdings Account expansion: the shared ten position columns and contract
  detail disclosure, without repeated ownership columns.
- Historical point expansion: the selected Account Report's positions; the stale
  Bybit sample retained its 2026-10-02 12:00 observation at a 13:15 point. Older
  fake references kept the known direction and positive Size with an
  unknown-mode explanation.
- Contract and numeric hover, click, Escape, horizontal scrolling to Observed,
  optional missing-margin explanation, stale reason, and missing-mode badge.

The runtime seed contains Binance, OKX, and Bybit positions. Kraken and Delivery
presentation have automated coverage; they were not represented by these runtime
seed rows. All session-started service and browser-test processes were stopped;
:8081 liveness failed afterwards and no :8081 listener remained.

## Recorded interface evidence

No live credentials were loaded and no new recording was requested or taken. The
existing accepted evidence remains unchanged. These are separate interface
observations, not atomic Account Reports or reconciled exchange-app totals.

| Platform                             | Existing evidence                                                            | Relevant limits                                                                                                                                                                 |
| ------------------------------------ | ---------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [Binance](./binance/verification.md) | 34 reviewed responses, 2026-09-30 UTC; owner review 2026-10-01 Asia/Shanghai | One-way/Hedge and cross/isolated fields support source shapes. CM Cross Hedge, PM delivery-specific observations, and exact metadata for some held UM contracts remain gaps.    |
| [OKX](./okx/verification.md)         | 20 reviewed responses, 2026-10-01                                            | Net and hedge positions, PM/MCM contract kinds, and Futures isolated rows are observed. Responses are separate captures; all settings and Futures delivery are not established. |
| [Bybit](./bybit/verification.md)     | 19 reviewed responses, 2026-10-01                                            | PM One-way and Cross One-way/Hedge are observed. Isolated is owner-deferred, and returned cursors have no recorded continuation proof.                                          |
| [Kraken](./kraken/verification.md)   | 11 reviewed responses, 2026-10-01                                            | Unified wallet and isolated-map fields are observed. Separate wallet, inverse delivery, and single-collateral vanilla positions remain gaps.                                    |

## Remaining real validation

Exchange-app comparison, complete Account reconciliation, unobserved contract
settings, and difficult live failure/pagination cases remain pending as listed
in each Platform's verification document. The new displayed margin amounts and
conversion policy have no completed exchange-app reconciliation. Synthetic
results and prior field observations do not close those gaps.

This historical report describes the previous additive migration. The current
prelaunch build consolidates its fields into the new baseline; follow
[the reset procedure](../deployment.md#upgrades-and-resets) and
[the current delivery report](./schema-provenance-delivery.md).
