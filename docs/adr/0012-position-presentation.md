---
status: accepted
---

# ADR 0012: Contract position presentation and margin provenance

## Decision

Contract tables share the order Platform, Identity, Account, Symbol, Side, Size,
Entry Price, Mark Price, Liq. Price, Margin, Unrealized PnL, Notional (USD), and
Observed. Account and historical details omit the already established ownership
columns. Keep the business ordering from ADR 0011 and allow horizontal
scrolling.

Symbol details disclose Base, Quote, Settle, Leverage, the effective
per-contract size with its currency, linear/inverse payoff, perpetual/delivery
term, and the delivery time. Hover, keyboard focus, and click disclose the same
information. An effective Contract Size already includes any multiplier; do not
label a USD face amount as a separate dimensionless multiplier.

Size is Base quantity for linear contracts and Quote face amount for inverse
contracts. Multiply source contract counts once by Contract Size; coin or quote
quantities already normalized by an Adapter are not multiplied again. Size does
not fluctuate with Mark Price. One-way positions display Both with signed Size;
hedged positions display Long or Short with positive Size. Stored direction and
source quantity are retained. Unknown historical mode keeps the known direction
and positive Size, with an issue explaining the missing mode.

Margin follows Platform conventions: Cross uses the position initial requirement
excluding orders, and Isolated uses allocated margin excluding unrealized PnL.
Keep the source amount, currency, and basis. Margin and unrealized PnL display
in Settle currency. USD-equivalent amounts may convert at 1:1 or, when Settle is
Base and Quote is USD-equivalent, by a positive Mark Price. Disclose conversion.
Do not estimate margin from notional/leverage or allocate Portfolio risk across
member positions. Portfolio remains visible alongside any known position margin
setting. Unsupported optional measures stay unknown without a core-data alarm.

Observed is the source Account Report time in the Portfolio timezone. Stale
fallback retains that time and the source Report's account Margin Mode. Badges
identify stale data and missing position mode, Size, or USD notional. Failures
without rows remain visible in page-level issues. Known notionals are summed
before row filtering and pagination; unknown required notionals or missing
position-capable Accounts make that total incomplete. Balance valuation
omissions do not make an otherwise known contract notional incomplete.

Position numbers follow the shared precision and zero-count notation in
[ADR 0013](./0013-numeric-presentation.md). It supersedes the temporary
eight-significant-digit policy and removes full-precision numeric disclosure;
contract and margin provenance details remain available.

## Reason and trade-offs

Displaying native contract counts obscured comparable position sizes. Quote face
amounts for inverse contracts preserve fixed size, unlike mark-derived
settlement amounts. Keeping position mode distinct from direction preserves
one-way sign semantics without changing the directional Position model.

Margin is not one interchangeable quantity across Platforms. Provenance and
explicit unknowns are preferable to showing an initial requirement as isolated
allocation, using deprecated Bybit collateral fields, or repeating Portfolio
risk-unit requirements on each Position. Account Report modes must be observed
history rather than the Identity's latest setting.

Historical restoration uses exact Platform-specific references and verifies
direction. Missing provenance cannot be guessed from current configuration;
financial source amounts are not rewritten. This choice leaves some old optional
fields unknown until new collection, while preserving trustworthy history.
