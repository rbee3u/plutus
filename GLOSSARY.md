# Plutus

Plutus observes a user's selected exchange accounts and compares their value
over time. This glossary defines its shared language;
[domain rules](./docs/domain-rules.md) define behavior and
[product scope](./docs/product-scope.md) defines coverage.

Term definitions identify concepts; case follows the shared
[capitalization guide](./docs/capitalization.md).

## Language

### Ownership and access

**user**: A person authenticated by Plutus who owns independent portfolios.
_Avoid_: identity, account, exchange login.

**platform**: An exchange or financial service providing accounts and market
prices, such as Binance, OKX, Bybit, or Kraken.

**portfolio**: One user's ledger of selected accounts and capital flows,
organized by platform, identity, and account.

**identity**: One platform login belonging to exactly one platform and
portfolio. Main and sub-account logins are separate identities. _Avoid_: user,
account, API key, connection.

**account type**: A kind of wallet, trading account, or product account
available within an account structure, or the application-owned manual type.

**account**: One account type selected for an identity, including its fixed
manual account. _Avoid_: identity, connection.

**portfolio boundary**: The selected accounts under enabled identities whose
value belongs to a portfolio.

**account structure**: A platform's arrangement of an identity's accounts,
determining which account types exist. _Avoid_: account mode, account level,
margin mode.

**margin mode**: A platform's observed margin regime for a whole trading account
within an account structure. _Avoid_: account structure, position margin
setting.

**reported margin ratio**: An account-level margin ratio supplied by its
platform, with the meaning specific to that platform and account type. _Avoid_:
a universal risk ratio across platforms.

**connection**: An identity's read-only platform credentials, bound to an
external login after successful validation. _Avoid_: identity, account.

**manual account**: An identity's always-selected account for user-entered
signed quantities missing from the platform API.

### Collection and holdings

**collection cycle**: One scheduled or manual pass over a portfolio's enabled
identities, ending in one valuation point after collection and valuation.
_Avoid_: refresh, sync, snapshot.

**collection run**: The work for one connection within a collection cycle.

**observation**: A normalized platform fact obtained during a collection run,
with its platform observation time. _Avoid_: raw response, derived value.

**account report**: One account's aligned collection facts: outcome, reported
equity and account measures when available, balances, positions, earn positions,
and dual positions. _Avoid_: equity, snapshot.

**balance**: An asset's equity in an account report, accompanied by
availability, liability, and unrealized-profit measures where known. _Avoid_:
quantity, holding as synonyms for its equity measure.

**price observation**: One stored market quote, retaining the requested
canonical asset, actual source market, retrieval time, and optional source time.
A cycle attempt stores one per platform and canonical asset; retrievals in other
cycles remain separate observations.

**position**: An open directional contract holding with side, size, prices, and
margin measures in an account report. _Avoid_: balance, earn position, dual
position.

**position mode**: Whether a contract position is one-way, with opposing trades
netted, or hedged, with separate long and short holdings. _Avoid_: direction,
margin mode.

**position margin setting**: Whether a position uses cross collateral or
isolated funds. _Avoid_: account margin mode, portfolio margin.

**position size**: Base quantity for a linear contract or quote face amount for
an inverse contract. _Avoid_: contract count, mark-derived settlement amount.

**position margin**: The initial requirement excluding orders for a cross
position, or allocated funds excluding unrealized PnL for an isolated position.
_Avoid_: shared collateral allocation, account margin requirement.

**complete set**: All balances, positions, earn positions, and dual positions
held by one account according to a successful account report.

**stale**: Value carried from an earlier successful account report with
compatible scope after collection fails. _Avoid_: missing, incomplete as
synonyms for freshness.

**earn position**: A descriptive holding in a selected earn account, at the
platform's supplied product, strategy, or individual-position granularity.

**dual investment**: A product accepting principal in one asset and settling in
that asset or its paired asset according to target price and expiry.

**dual position**: One outstanding dual investment subscription with its
principal and available product terms. _Avoid_: balance, futures position.

### Assets and valuation

**asset**: A unit of financial value recognized within one portfolio and
platform under its own symbol and mapped to a canonical asset. Assets on
different platforms remain distinct even when their symbols match. _Avoid_:
platform asset, asset listing, listing.

**canonical asset**: A financial identity across platforms within one portfolio,
identified by a normalized symbol, such as BTC or USDT. _Avoid_: asset as a
synonym.

**futures contract**: A portfolio-owned, platform-specific perpetual or delivery
contract: one symbol with one immutable set of observed terms, including
settlement asset, expiry, and contract size. Changed terms record another
futures contract under the same symbol; identical terms reuse it, including A →
B → A. Observation times do not establish when the platform made those terms
effective. _Avoid_: instrument, asset, trading pair, contract version.

**asset symbol**: An asset's code within its platform, which may differ from its
canonical asset symbol.

**canonical asset symbol**: The normalized code identifying a canonical asset
across platforms.

**futures symbol**: A futures contract's code within its platform. Contracts
with different terms can share it; it identifies neither an open position nor a
base/quote pair.

**valuation asset**: USD, the canonical asset used as the unit of all derived
financial values in Plutus.

**account valuation**: One account's USD value at a valuation point, qualified
by valuation method, freshness, and completeness. _Avoid_: unqualified equity,
balance total.

**valuation point**: A portfolio's total of account valuations at the end of one
collection cycle. _Avoid_: snapshot, projection, generation.

**incomplete**: A valuation or comparison for which required, non-excluded value
or another required input cannot be determined. _Avoid_: zero, stale as synonyms
for completeness.

**asset exclusion**: A reviewed, reversible portfolio rule excluding an asset's
balances from future calculated account valuations.

### Flows and periods

**capital flow**: A user-recorded contribution or withdrawal crossing the
portfolio boundary, expressed as canonical asset quantity at an effective time,
with an optional account. _Avoid_: internal transfer, balance change, manual
adjustment.

**boundary event**: An account entering or leaving the portfolio boundary
through selection, deselection, or identity archive.

**value change**: The absolute USD difference between observed valuation points
after accounting for capital flows and boundary events. _Avoid_: profit, P&L,
return, performance.

**period close**: The last valuation point within a calendar day, ISO week,
month, quarter, or year in the portfolio's timezone.

**daily snapshot**: The final whole valuation point retained for an older local
day when detail cleanup runs. Its cycle, timestamp, total, and source evidence
remain unchanged; timezone edits do not replace it.
