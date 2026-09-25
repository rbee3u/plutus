---
status: accepted
---

# Separate the Account Structure from the Margin Mode

One "Account Mode" used to name two different things. The first is how a Platform arranges an Identity's wallets, which decides the Account Types: Binance Classic Trading keeps Spot, Margin, USDⓈ-M, and COIN-M apart, while Portfolio Margin merges them into one Account, and Kraken's Separate wallets and Unified Wallet differ the same way. The second is the margin regime of one trading account: OKX's Spot mode, Futures mode, Multi-currency margin, and Portfolio margin, and Bybit's Isolated, Cross, and Portfolio Margin. These keep the same Accounts and change only which figures a report carries. Declaring the second kind made the User predict a setting the owner can change in the exchange app at any moment, and every switch failed collection ([ADR 0025](./0025-re-check-the-account-structure-in-every-collection-run.md)).

Model the two levels separately.

- The **Account Structure** is declared when the Identity is created and never changes. The Adapter verifies it at validation and at the start of every Collection Run. A mismatch fails the run with `unsupported`, the Accounts stay Stale, and the Connection stays enabled.
- The **Margin Mode** is observed, never declared. The Adapter reads it at validation and at the start of every Collection Run, and it selects the field mapping. Plutus stores it on the Identity and shows it. A change is written to the audit log with the system as the actor. A value Plutus does not know fails the run with `unsupported`. Before the first successful validation it is unknown.

| Platform | Account Structure | Margin Mode | Source |
| --- | --- | --- | --- |
| Binance | `binance_classic` Classic Trading, `binance_pm` Portfolio Margin | none | `/sapi/v1/account/info` `isPortfolioMarginRetailEnabled` |
| OKX | `okx_unified` Unified account | `okx_spot`, `okx_futures`, `okx_mcm`, `okx_pm` | `/api/v5/account/config` `acctLv` 1–4 |
| Bybit | `bybit_uta_2_0` Unified Trading Account 2.0 | `bybit_isolated`, `bybit_cross`, `bybit_pm` | `/v5/account/info` `unifiedMarginStatus` 5/6 and `marginMode` |
| Kraken | `kraken_separate` Separate wallets, `kraken_unified` Unified Wallet | none | Futures balances `flex_futures.unified_balances` |

Binance Portfolio Margin Pro, Bybit classic accounts, and Bybit UTA 1.0 remain unsupported. Options remain out of scope: an open option position still fails its Account's report.

## Consequences

- Switching structure on the Platform means creating a new Identity with the new structure and archiving the old one. The archive and the new selection are Boundary Events ([ADR 0016](./0016-treat-boundary-events-as-implicit-capital-flows.md)), valued at the old Accounts' last and the new Accounts' first valued amounts. Market movement between those two reports therefore counts as zero Value Change. This is expected.
- An Identity is bound to the Platform account its credentials first reached, recorded as its external reference. Validation refuses credentials that reach a different Platform account with `unsupported`, and the Connection stays disabled: a different account is a different Identity.
- `identities.account_structure` is checked against the Platform, and the nullable `identities.margin_mode` is checked against the structure. Prod had not migrated yet, so the baseline migration was edited in place under [ADR 0024](./0024-freeze-the-baseline-migration-once-prod-has-migrated.md) and every environment was reset.
- Some Margin Modes report fewer figures, and Plutus leaves the missing ones unknown instead of zero. OKX Futures mode reports no account-level adjusted equity, margin ratios, or per-coin borrowing (borrowings exist only on margin positions, which are not itemized). Bybit Isolated Margin reports no account-wide figures, so its value is calculated from Balances.
- This replaces the mode lists of [ADR 0005](./0005-support-explicitly-selected-account-structures-and-margin-modes.md) and [ADR 0026](./0026-support-kraken-separate-and-unified-wallets.md) and narrows [ADR 0025](./0025-re-check-the-account-structure-in-every-collection-run.md) to the Account Structure.
