# Portfolio Observation

Plutus periodically collects the balances, positions, and account reports that configured exchange accounts disclose, values them in USD, and shows how the total changed over time once the owner's own transfers in and out are excluded.

## Language

### Identity and access

**User**:
A person authenticated by Plutus who owns one or more Portfolios. A User is distinct from every Platform Identity.
_Avoid_: Administrator, Identity, Account (as synonyms for a Plutus login)

**Platform**:
An exchange or other financial service that provides Accounts and market prices, such as Binance, OKX, or Bybit.

**Identity**:
One login identity recognized by a Platform, under which that Platform organizes Accounts. A main account and each of its sub-accounts are separate, unrelated Identities.
_Avoid_: User, Account, API key, Connection, sub-account (as synonyms for platform identity)

**Account Mode**:
The Platform-defined margin or account regime an Identity runs in, such as Binance Portfolio Margin or Bybit Unified Trading Account 2.0. It determines which Account Types the Platform can offer.

**Account Type**:
One kind of Platform wallet or trading account that an Adapter knows how to collect for a given Platform and Account Mode, such as a unified trading account, a funding account, or an earn account.

**Account**:
One Account Type selected by the User for one Identity. Only selected Accounts are collected and valued.
_Avoid_: Identity, wallet, product type (as synonyms for Account)

**Connection**:
The read-only API credentials configured for one Identity. Replacing credentials does not change the Identity or its Accounts.
_Avoid_: Identity, Account, API key (as synonyms for configured access)

**Adapter**:
The Plutus component that talks to one Platform and translates its responses into Observations. It declares the Account Modes and Account Types it supports.

### Asset catalog

**Asset**:
A unit of financial value identified by its normalized symbol across Platforms, such as BTC or USDT. Assets with different symbols remain distinct even when economically related.
_Avoid_: Ticker, coin (as synonyms for canonical asset identity)

**Asset Listing**:
One Platform's symbol for an Asset. A listing maps to an Asset automatically by symbol unless the User overrides the mapping.
_Avoid_: Asset (when the Platform-specific spelling matters)

**Instrument**:
A specific tradable contract or market on one Platform, identified by its symbol, product kind, settlement Asset, and expiry where applicable. A perpetual and a delivery contract on the same Asset are distinct Instruments.
_Avoid_: Asset, pair (when referring to a contract)

**Valuation Asset**:
The Asset in which every derived value is expressed. It is USD; USDT and USDC are treated as equal to USD when valuing.

### Collection and observations

**Collection Cycle**:
One scheduled or manual pass over every enabled Connection in a Portfolio, ending in one Valuation Point.
_Avoid_: Refresh, sync, snapshot (as synonyms for the cycle)

**Collection Run**:
The work done for one Connection within one Collection Cycle.

**Observation**:
A fact obtained from one Platform during one Collection Run, stored with its Platform observation time. Observations are never edited; the next Collection Run produces new ones.
_Avoid_: Platform response, projection (as synonyms for a stored source fact)

**Account Report**:
The Observation of one Account in one Collection Run: whether collection succeeded, the Account's reported equity when the Platform provides one, and a fixed set of Plutus-defined account-level measures such as margin ratios.
_Avoid_: Snapshot, metrics, equity (as the whole report)

**Balance**:
The Observation of one Asset Listing in one Account Report, led by that Asset's equity in the Account together with availability, liability, and unrealized-profit measures.
_Avoid_: Quantity, holding (as the equity measure)

**Position**:
The Observation of one open directional contract holding in one Account Report, with its side, size, prices, and margin measures. Opposing long and short holdings are separate Positions.
_Avoid_: Balance, margin, exposure (as synonyms for a position)

**Price Observation**:
A market or mark price for one Asset Listing or Instrument on one Platform at an observation time, quoted in a stated Asset.

**Complete Set**:
The rule that the Balances and Positions in a successful Account Report are everything the Account holds; anything absent is no longer held.

**Stale**:
The state of an Account whose latest Collection Run failed, so its most recent successful Account Report is still in use, labeled with that report's age.

### Portfolio and boundary

**Portfolio**:
One User's ledger: the set of enabled Identities and their selected Accounts whose values are totaled together and whose Capital Flows are recorded. Each Identity belongs to exactly one Portfolio.
_Avoid_: User, Identity, Account, view (as synonyms for the portfolio boundary)

**Portfolio Boundary**:
The set of Accounts currently selected under enabled Identities in a Portfolio. Value inside the boundary is counted; value outside it is invisible to Plutus.

**Capital Flow**:
A transfer of value across the Portfolio Boundary recorded manually by the User as an Asset quantity at an effective time, either a contribution or a withdrawal. Transfers between Accounts inside the boundary are not Capital Flows.
_Avoid_: Deposit, internal transfer, balance change (as synonyms for boundary-crossing value)

**Boundary Event**:
An Account entering or leaving the Portfolio Boundary, whether by selection, deselection, or Identity archive. It is treated as an implicit Capital Flow equal to that Account's first or last valued amount.

### Derived results

**Account Valuation**:
The USD value of one Account at one Valuation Point, taken from the Account Report's reported equity when present and otherwise calculated from its Balances and Price Observations, with its method, freshness, and completeness recorded.
_Avoid_: Balance total, equity (without stating the method)

**Valuation Point**:
The USD total of every Account Valuation in a Portfolio at the end of one Collection Cycle, stored once and never recalculated except after a mapping correction.
_Avoid_: Snapshot, projection, generation

**Incomplete**:
The state of a Valuation Point or Account Valuation whose total omits value that could not be determined, such as a Balance without a price. The total remains a real number and names what it omits.
_Avoid_: Null, zero (as representations of unknown value)

**Period Close**:
The last Valuation Point at or before the end of a calendar day, ISO week, month, quarter, or year in the Portfolio's timezone. A period with no Valuation Point has no Period Close.

**Value Change**:
The difference between two Valuation Points minus the contributions and plus the withdrawals, including Boundary Events, whose effective times fall between them. It is an absolute USD amount, not a return.
_Avoid_: Profit, P&L, return, performance
