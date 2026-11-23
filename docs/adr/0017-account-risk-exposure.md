---
status: accepted
---

# Account-level risk exposure

Unified accounts hold spot and contract positions in one place, but the balance
table only shows per-coin equity. A user cannot see how much of a coin remains
unhedged after open contracts. Portfolio-level aggregation is a separate product
step; this decision covers one account.

Risk exposure is one asset's signed coin quantity in one supported unified
account. The spot leg is the balance's net equity: it already includes
coin-denominated unrealized PnL and futures collateral and is net of debt.
Linear contracts contribute base quantity; inverse contracts contribute quote
face divided by their positive mark price. Long is positive and short is
negative. Cross and isolated margin, one-way and hedged sides, and perpetual and
delivery contracts all net. A contract-only asset with no spot balance row is
synthesized with zero spot equity, zero liability, zero equity USD, and the
actual coin-settled position result: positions settled in another coin
contribute zero, and unknown contributing PnL stays unknown.

Supported scopes are Binance portfolio margin, OKX unified trading, Bybit
unified, and Kraken unified main. Kraken coin_m and derivatives holding are
separate accounts and do not enter main's account-level exposure. Funding, earn,
dual investment, manual balances, and other accounts are excluded. Options are
not collected and do not contribute; Binance PM does not fetch them, while OKX,
Bybit, and Kraken fail a report that contains an option row. Platform rollout is
explicit: a supported scope without an enabled calculation is unknown, never
zero. Every supported scope currently computes: Binance portfolio margin, OKX
unified trading, Bybit unified, and Kraken unified main.

A row without an adopted spot price may show a contract mark price for
reference. The mark is the largest absolute USD notional position for that
asset, with display order breaking ties. The fallback changes neither equity USD
nor valuation completeness, and historical reads use the point's retained
balances, positions, terms, and marks, never current prices. Contract base
assets are recognized in the catalog; multiplier-prefixed contracts such as
1000PEPE remain distinct from PEPE.

The alternative of a USD notional exposure would mix risk with valuation and
lose the coin quantity users reconcile against their balances. Partial sums were
rejected because one unconvertible position would silently understate the net.
Portfolio-level netting, option delta, and triangle or multiplier symbol merges
are out of scope; they require separate product decisions.

[Domain rules](../domain-rules.md#account-level-risk-exposure) own behavior,
[presentation](../presentation.md#balances) owns display, and the
[platform mappings](../../README.md#documentation) own source fields.
