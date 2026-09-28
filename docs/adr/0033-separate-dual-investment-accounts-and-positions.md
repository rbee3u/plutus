---
status: accepted
---

# Separate Dual Investment Accounts and Dual Positions

Dual subscriptions have their own target prices, maturities, directions, and annualized rates. Aggregating their principal into Earn preserved value but discarded these holdings and hid the product boundary. Published APIs expose subscriptions; they do not consistently expose a separately transferable wallet or unused cash balance. OKX's funded test account also provides evidence that its aggregate Earn valuation includes Dual principal.

## Decision

- Binance, OKX, and Bybit offer a separately selected `dual_investment` Account named Dual Investment. Ordinary Earn excludes Dual principal. This is a product Account, not a claim that the Platform has a separately transferable wallet.
- Each outstanding subscription is one Dual Position, identified by Binance `id`, OKX `ordId`, or Bybit `positionId`. Positions are never merged by product, coin, target price, or maturity. A separate type and table preserve terms without imposing futures Instrument or margin semantics.
- Dual Balances sum outstanding principal by invested currency. Principal is valued in that currency at the usual USD price; expected yield is excluded. Dual Positions describe those same Balances and never add value again. No available or withdrawable Balance is invented.
- Position identity, invested currency, and positive principal are required. Contradictory or malformed terms fail the report. Missing optional product terms remain null, with a named display issue, while known principal remains valued. Annualized rates preserve their APR, APY, or annualized-yield label.
- Principal remains held in the running states established in ADR 0029. Only a successful report can remove a Balance or Dual Position. Settlement reads remain non-atomic across Accounts, so temporary gaps or double counts during payout are still possible. Bybit pending orders before a position exists are not guessed into holdings.
- When either Earn or Dual is selected, both product groups are read once to compare their combined principal against the Platform's Earn aggregate. This is a narrow exception to ADR 0015: unselected product data is auxiliary only and is neither emitted as an Account Report nor valued. When neither is selected, neither is read. Necessary auxiliary failures and unexplained aggregate gaps fail both selected dependent Accounts. Authentication failures retain their existing Connection-wide effect.
- The aggregate check retains ADR 0029's one-sided tolerance: a shortfall greater than `max(1% of the Platform total, 1 USD)` fails with `unsupported`. It runs before Portfolio exclusions. Missing prices skip the comparison; selected unpriced Balances retain normal valuation omissions. Binance's dedicated Dual aggregate is an additional live diagnostic, not reported equity or a runtime dependency.

## Existing data

The appended migration creates Dual Accounts for existing Identities and copies each Earn selection. Existing Dual configuration is never overwritten. Historical observations and points are retained; the old aggregated principal cannot be split retrospectively.

Reports persist their scope version, including failed reports. Historical reports have scope 1; new Earn and Dual reports on these three Platforms have scope 2. Stale fallback requires an equal scope. A new Earn report without compatible history is missing, with an explicit explanation, rather than reusing combined principal. Other Platforms and Accounts keep their existing scopes.

The first point after an observed legacy Earn is split records `account_scope_changed`. Dual's inherited inclusion is reclassification, not a contribution. If only Dual remains, no withdrawal amount is guessed for the unknown savings part. If the entire old slice leaves, its known combined withdrawal is retained. Value Changes spanning a scope transition are incomplete; points with fully known holdings can remain complete. Later ordinary selection changes retain normal Boundary Events. Historical rebuilding uses stored report scopes and reproduces the transition.

Drain active or partially collected cycles with the previous release and stop every worker before upgrading. The migration refuses those unfinished cycles. New and old workers must never run together.

## Consequences

- Earn and Dual can be selected independently, but their completeness checks share a failure dependency.
- The Positions UI separates Futures and Dual Investment. This delivery adds no options, settled-order browser, payoff simulation, or transfer reconciliation.
- This supersedes ADR 0029's combined Account and completeness arrangement, extends ADRs 0015, 0016, and 0019, and preserves the principal valuation, Complete Set, and one aligned Account Report decisions.
