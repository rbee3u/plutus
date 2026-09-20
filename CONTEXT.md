# Portfolio Observation

Plutus records normalized observations across configured financial identities and derives portfolio valuations, exposure, and capital-adjusted value changes over time.

## Language

### Identity and access

**User**:
A person authenticated by Plutus who owns and directly manages one or more Portfolios. A User is distinct from every Platform Identity.
_Avoid_: Administrator, Identity, Account (as synonyms for a Plutus login)

**Platform**:
An exchange or other financial service that provides accounts and market information, such as Binance, OKX, or Bybit.

**Identity**:
A distinct identity recognized by one Platform, under which Accounts are organized. Main and subordinate identities are separate Identities, distinct from Plutus login users and independent of access credentials.
_Avoid_: User, Account, API key, Connection (as synonyms for platform identity)

**Account**:
A platform-defined financial account belonging to one Identity, such as a funding or unified trading account, with a particular asset scope. A product category does not by itself define an Account.
_Avoid_: Identity, product type (as synonyms for Account)

**Connection**:
A configured access path to one Identity on one Platform. Replacing its credentials does not change the Identity or its Accounts.
_Avoid_: Identity, Account, API key (as synonyms for configured access)

### Asset catalog

**Asset**:
A distinct unit of financial value with a shared identity across Accounts and Platforms, such as BTC, USDT, or USD. Related assets, including wrapped assets and their underlying assets, remain distinct.
_Avoid_: Ticker, symbol (as sufficient asset identity)

**Asset Listing**:
A Platform's representation of an Asset, retaining its platform-native identity and symbol. Listings with matching symbols do not by themselves establish a shared Asset identity.
_Avoid_: Asset, ticker (as sufficient cross-platform identity)

**Instrument**:
A specific trading market or contract on a Platform, identified by its assets and product characteristics, including settlement asset and expiry where applicable. A spot pair, a perpetual contract, and a delivery contract are distinct Instruments even when they reference the same Asset.
_Avoid_: Asset (when referring to a trading pair or contract)

### Balances and positions

**Balance**:
The state of one Asset in one Account, expressed through reported measures such as total, available, or locked quantities. Different measures can describe overlapping quantities.
_Avoid_: Equity (as a synonym for an asset quantity)

**Position**:
An open directional exposure in one Instrument held in one Account, with its quantity and associated reported measures. Opposing Positions remain distinct even when their net Exposure is zero.
_Avoid_: Balance, margin (as synonyms for a position)

### Observations

**Observation**:
A normalized, time-qualified fact obtained from one Platform for a declared scope. It retains its provenance, coverage, and completeness and is distinct from any derived result.
_Avoid_: Platform response, aggregate (as synonyms for a normalized source fact)

**Identity Snapshot**:
A collection of Observations gathered for one Identity across its Accounts during a refresh, including Balances, Positions, Reported Equity, and Account Metrics. Its contents can have different source observation times.

**Price Observation**:
A price for a particular Instrument or Asset on a Platform, with a price type, quote Asset, and observation time. Market, mark, and index prices are distinct Observations.
_Avoid_: Price (without qualification when source, type, or time matters)

**Reported Equity**:
The net value supplied by a Platform for a specified coverage scope and denomination. Discounted collateral values and margin requirements are different measures.
_Avoid_: Notional value, collateral value, margin requirement (as synonyms for equity)

**Account Metric**:
A Platform-defined measure reported for one Account, with its unit and observation time, such as UniMMR. Metrics are not comparable or aggregatable across Platforms unless their semantics are established.

### Portfolio and capital

**Portfolio**:
The collection of enabled Identities selected by one User for asset observation and the boundary for Capital Flows. A User can own multiple Portfolios, but each Identity belongs to exactly one Portfolio.
_Avoid_: User, Identity, Account, filtered view (as synonyms for the portfolio boundary)

**Capital Baseline**:
The known cumulative contributed capital for the Portfolio at an effective time, expressed in a specified denomination. A Portfolio has one logical Capital Baseline whose current value can be corrected.
_Avoid_: Balance, Portfolio Valuation (as synonyms for contributed capital)

**Capital Flow**:
An external contribution or withdrawal that crosses the Portfolio boundary, stated as a quantity of one Asset with an effective time and, when known, the Account through which it entered or left. Movement between Accounts within the Portfolio is not a Capital Flow.
_Avoid_: Internal transfer, Balance change (as synonyms for external capital movement)

**Contributed Capital**:
The Capital Baseline adjusted by subsequent Capital Flows for a specified time and denomination.
_Avoid_: Portfolio Valuation, profit

### Derived results

**Calculated Valuation**:
An estimate derived from Balance and Position Observations and Price Observations for a specified scope and denomination. It is distinct from Reported Equity, even when the amounts agree.

**Portfolio Valuation**:
The derived net value of the Portfolio for a specified evaluation time, coverage, and denomination, combining non-overlapping Reported Equity and sufficient Calculated Valuations. It retains the provenance, freshness, and completeness of its inputs.

**Exposure**:
The amount of an underlying Asset represented by Balances and Positions, expressed in comparable underlying-asset units. Long and short components remain distinct from their net amount and from equity.
_Avoid_: Equity, Portfolio Valuation (as synonyms for directional exposure)

**Value Change**:
The change between comparable Portfolio Valuations after subtracting contributions and adding withdrawals over the interval; from the Capital Baseline, it uses that baseline as the starting basis. It does not attribute the change or represent an investment return.
_Avoid_: Profit, P&L, return
