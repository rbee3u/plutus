---
status: accepted
---

# Portfolio risk exposure

[ADR 0017](./0017-account-risk-exposure.md) defines exposure for one supported
unified account. A user with accounts on several platforms still cannot see one
coin's net quantity across the whole portfolio, and the account-level rule
deliberately excludes funding, earn, dual investment, classic margin and
futures, Kraken separate wallets, and manual balances. A portfolio total that
merely summed the visible account-level cells would silently omit those
accounts.

Portfolio exposure is one canonical asset's signed coin quantity summed across
every selected account in the portfolio boundary, across platforms. Each account
contributes its balance equity for the asset plus the net base quantity of its
open linear and inverse contracts in the same account. Linear contracts
contribute base quantity; inverse contracts contribute quote face divided by
their positive mark price. Long is positive and short is negative. Cross and
isolated margin, one-way and hedged sides, and perpetual and delivery contracts
net. Every selected account type participates, including funding, earn, dual
investment, classic margin and futures, Kraken wallets, and manual. This is
intentionally broader than account-level exposure; the balance table's column
and ADR 0017 do not change.

Dual investment principal is already held in the dual account's balances in its
purchase coin; positions are descriptive and are not added again. Funding and
earn principal are likewise already balances. Balance equity is net of debt, so
liability is not subtracted again. Excluded platform assets are omitted from
both legs and counted for disclosure. Pending manual edits are excluded because
they take effect at the next collection; collected manual balances participate.
Stale accounts contribute their compatible successful report and are marked.

An account's contribution to one asset is unknown when a position size, side,
inverse mark, or balance quantity is unconvertible. Known accounts still sum and
the row is partial; when nothing is known the row is unknown, never zero.
Missing accounts and positions without a catalog base make the filtered list
incomplete and are named at list level rather than guessed into a row. A
contract base with no balance row contributes from zero equity. Canonical assets
merge across platforms; multiplier-prefixed contracts such as 1000PEPE remain
distinct from PEPE.

Rows show the net coin quantity and a reference value. The reference price is
the price of the known contribution with the largest absolute quantity, with
account order breaking ties, using that contribution's adopted spot price or
display-only contract mark. Value (USD) is the net quantity times that price; a
known zero is zero, a nonzero quantity without a price is unknown, and the value
is not a valuation or liquidation value. There is no cross-coin USD total: long
BTC and short ETH are not a hedge, and a single signed total would imply one.
Rows without a value sort last.

The read is point-scoped and derived from retained reports, positions, terms,
marks, and prices. It is not persisted, does not change valuation or totals, and
is not a liquidation, margin, or hedge measure. Cross-platform netting can
offset positions that cannot collateralize one another; the view discloses that.
Options are not collected; Binance portfolio margin may omit them, while OKX,
Bybit, and Kraken fail reports containing option rows.

Rejected alternatives were summing the account-level column (misses every
excluded account type and is not a portfolio aggregate), a USD net total
(confuses risk with valuation and hides offsetting coin positions), per-platform
rows only (does not answer the portfolio question), expanding the account-level
column to every account (changes the accepted 0017 measure and its presentation
contract), persisting a portfolio exposure table (duplicates retained facts and
freezes a display derivation), and adding history immediately (the point-scoped
read is the prerequisite).

[Domain rules](../domain-rules.md#portfolio-level-risk-exposure) own behavior,
[presentation](../presentation.md#portfolio-exposure) owns display, and the
[platform mappings](../../README.md#documentation) own source fields.
