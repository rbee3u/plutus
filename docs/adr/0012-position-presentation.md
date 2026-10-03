---
status: accepted
---

# ADR 0012: Contract position presentation and margin provenance

## Decision

Contract tables share the order Platform, Identity, Account, Symbol, Side, Size,
Entry Price, Mark Price, Liq. Price, Unrealized PnL, Margin, Notional (USD), and
Observed. Account and historical details omit the already established ownership
columns. Keep the business ordering from ADR 0011 and allow horizontal
scrolling.

Symbol details disclose Base, Quote, Settle, the effective per-contract size
with its currency, linear/inverse payoff, perpetual/delivery term, and the
delivery time. Hover, keyboard focus, and click disclose the same information.
An effective Contract Size already includes any multiplier; do not label a USD
face amount as a separate dimensionless multiplier.

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
Base and Quote is USD-equivalent, by a positive Mark Price. Validate conversion.
Prefer a reported position amount with verified meaning and units. For supported
Binance Cross futures, including retail Portfolio Margin, a missing reliable
source can use the documented initial requirement: absolute mark notional
divided by initial leverage, converted to Settle. Linear contracts use absolute
Base Size times Mark; inverse contracts use Quote face Size and require Mark for
conversion to Base Settle. Validate specifications, currencies, and positive
Mark and leverage. Retain the source amount and distinguish reported from
calculated display values. This is not a universal Platform formula: reported
Bybit fees and hedge treatment take precedence, and Kraken effective leverage is
not inverted. Never allocate Portfolio risk-unit requirements or shared
collateral across member positions. Unsupported optional measures stay unknown
without a core-data alarm.

Margin displays only its amount and Settle currency on one line. Known amounts
and unknown `—` align to the same right edge; the column fits its content
without an empty tag slot or extra table space. Its hover, focus, and click
disclosure is a short line such as `1.88x isolated`, `3x cross`, or
`portfolio margin`. Cross uses reported Position leverage. Isolated uses
absolute USD notional divided by the USD value of allocated margin, calculating
from source decimals before display rounding. USD-equivalent Settle converts
1:1; Base Settle requires a positive Mark with USD-equivalent Quote. Missing or
invalid inputs leave only the known mode, without guessing leverage. Actual
Isolated takes precedence over Portfolio context. Leverage no longer appears in
Symbol details; margin provenance, conversion facts, and unavailable reasons
remain in the API rather than the compact disclosure. The amount is one keyboard
target.

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
contract details remain in Symbol disclosures and margin provenance in the API.

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

The earlier mode tags made a supporting detail consume permanent column space.
Mode and leverage belong together in the Margin disclosure, while the visible
row prioritizes Unrealized PnL before the compact Margin amount. A short mode
line replaces the earlier calculation and provenance paragraphs.

The earlier blanket prohibition on notional/leverage and on all Portfolio
position amounts was too broad. Binance's retail PM still has Futures initial
leverage and initial-margin checks, unlike its separate PM Pro offering. The
owner's screenshots approximately support the documented formula but do not
establish exact reconciliation between asynchronously updated displayed fields.
[Binance's PM overview](https://www.binance.com/en/support/faq/detail/174861644e7049aaa64845f1db2e4376)
defines the Futures initial requirement;
[Bybit's margin calculation rules](https://www.bybit.com/en/help-center/article/Understanding-the-Adjustment-and-Impact-of-the-New-Margin-Calculation)
show why a universal replacement would lose fees and hedge treatment.

OKX's published position `imr` unit and reviewed non-PM inverse observations
conflict. The accepted read policy verifies the source against the documented
native and USD initial-requirement candidates with a relative rounding tolerance
of `1e-8`. An unambiguous match establishes the unit; neither or ambiguous
matches leave it unknown. This repairs old typed-USD presentation without
rewriting financial source amounts. It does not infer missing historical
provenance or distribute PM risk units. See
[OKX's native margin rules](https://www.okx.com/en-sg/help/futures-margin-calculation-rules)
and the
[Adapter mapping](../adapters/okx/mapping.md#position-presentation-provenance).

Historical restoration uses exact Platform-specific references and verifies
direction. Missing provenance cannot be guessed from current configuration;
financial source amounts are not rewritten. This choice leaves some old optional
fields unknown until new collection, while preserving trustworthy history.
