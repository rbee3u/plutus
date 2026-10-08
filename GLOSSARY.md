# Plutus

Plutus observes a User's selected exchange Accounts and compares their value
over time. This glossary defines its shared language;
[domain rules](./docs/domain-rules.md) define behavior and
[product scope](./docs/product-scope.md) defines coverage.

## Language

### Ownership and access

**User**: A person authenticated by Plutus who owns independent Portfolios.
_Avoid_: Identity, Account, exchange login.

**Platform**: An exchange or financial service providing Accounts and market
prices, such as Binance, OKX, Bybit, or Kraken.

**Portfolio**: One User's ledger of selected Accounts and Capital Flows,
organized by Platform, Identity, and Account.

**Identity**: One Platform login belonging to exactly one Platform and
Portfolio. Main and sub-account logins are separate Identities. _Avoid_: User,
Account, API key, Connection.

**Account Type**: A kind of wallet, trading account, or product account
available within an Account Structure, or the application-owned Manual type.

**Account**: One Account Type selected for an Identity, including its fixed
Manual Account. _Avoid_: Identity, Connection.

**Portfolio Boundary**: The selected Accounts under enabled Identities whose
value belongs to a Portfolio.

**Account Structure**: A Platform's arrangement of an Identity's Accounts,
determining which Account Types exist. _Avoid_: Account Mode, account level,
Margin Mode.

**Margin Mode**: A Platform's observed margin regime for a whole trading Account
within an Account Structure. _Avoid_: Account Structure, Position Margin
Setting.

**Connection**: An Identity's read-only Platform credentials, bound to an
external login after successful validation. _Avoid_: Identity, Account.

**Manual Account**: An Identity's always-selected Account for user-entered
signed quantities missing from the Platform API.

### Collection and holdings

**Collection Cycle**: One scheduled or manual pass over a Portfolio's enabled
Identities, ending in one Valuation Point after collection and valuation.
_Avoid_: refresh, sync, snapshot.

**Collection Run**: The work for one Connection within a Collection Cycle.

**Observation**: A normalized Platform fact obtained during a Collection Run,
with its Platform observation time. _Avoid_: raw response, derived value.

**Account Report**: One Account's aligned collection facts: outcome, reported
equity and account measures when available, Balances, Positions, Earn Positions,
and Dual Positions. _Avoid_: equity, snapshot.

**Balance**: An Asset Listing's equity in an Account Report, accompanied by
availability, liability, and unrealized-profit measures where known. _Avoid_:
quantity, holding as synonyms for its equity measure.

**Position**: An open directional contract holding with side, size, prices, and
margin measures in an Account Report. _Avoid_: Balance, Earn Position, Dual
Position.

**Position Mode**: Whether a contract Position is one-way, with opposing trades
netted, or hedged, with separate long and short holdings. _Avoid_: direction,
Margin Mode.

**Position Margin Setting**: Whether a Position uses cross collateral or
isolated funds. _Avoid_: Account Margin Mode, Portfolio Margin.

**Position Size**: Base quantity for a linear contract or Quote face amount for
an inverse contract. _Avoid_: contract count, mark-derived settlement amount.

**Position Margin**: The initial requirement excluding orders for a cross
Position, or allocated funds excluding unrealized PnL for an isolated Position.
_Avoid_: shared collateral allocation, Account margin requirement.

**Complete Set**: All Balances, Positions, Earn Positions, and Dual Positions
held by one Account according to a successful Account Report.

**Stale**: Value carried from an earlier successful Account Report with
compatible scope after collection fails. _Avoid_: missing, incomplete as
synonyms for freshness.

**Earn Position**: A descriptive holding in a selected Earn Account, at the
Platform's supplied product, strategy, or individual-position granularity.

**Dual Investment**: A product accepting principal in one Asset and settling in
that Asset or its paired Asset according to target price and expiry.

**Dual Position**: One outstanding Dual Investment subscription with its
principal and available product terms. _Avoid_: Balance, futures Position.

### Assets and valuation

**Asset**: A unit of financial value identified by normalized symbol across
Platforms, such as BTC or USDT. _Avoid_: Asset Listing, Instrument.

**Asset Listing**: One Platform's native symbol for an Asset.

**Instrument**: A Platform-specific contract or market identified by symbol,
product kind, settlement Asset, and expiry where applicable. _Avoid_: Asset.

**Valuation Asset**: USD, the unit of all derived financial values in Plutus.

**Price Observation**: An Asset's market price on a Platform at an observation
time, quoted in a stated Asset.

**Account Valuation**: One Account's USD value at a Valuation Point, qualified
by valuation method, freshness, and completeness. _Avoid_: unqualified equity,
Balance total.

**Valuation Point**: A Portfolio's total of Account Valuations at the end of one
Collection Cycle. _Avoid_: snapshot, projection, generation.

**Incomplete**: A valuation or comparison for which required, non-excluded value
or another required input cannot be determined. _Avoid_: zero, stale as synonyms
for completeness.

**Listing Exclusion**: A reviewed, reversible Portfolio rule excluding an Asset
Listing's Balances from future calculated Account Valuations.

### Flows and periods

**Capital Flow**: A User-recorded contribution or withdrawal crossing the
Portfolio Boundary, expressed as Asset quantity at an effective time, with an
optional Account. _Avoid_: internal transfer, balance change, Manual adjustment.

**Boundary Event**: An Account entering or leaving the Portfolio Boundary
through selection, deselection, or Identity archive.

**Value Change**: The absolute USD difference between observed Valuation Points
after accounting for Capital Flows and Boundary Events. _Avoid_: profit, P&L,
return, performance.

**Period Close**: The last Valuation Point within a calendar day, ISO week,
month, quarter, or year in the Portfolio's timezone.
