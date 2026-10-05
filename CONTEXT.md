# Domain model

Canonical terms, relationships, and business rules for Plutus. Use
[product scope](./docs/product-scope.md) for offered Accounts,
[ADRs](./docs/adr/README.md) for decisions and trade-offs, and
[architecture](./docs/architecture.md) for implementation mechanisms.

## Ownership and the Portfolio Boundary

**User**: A person authenticated by Plutus who owns one or more independent
Portfolios. A User is distinct from a Platform Identity; do not call a Plutus
login an Identity or Account.

**Platform**: An exchange or financial service providing Accounts and market
prices, such as Binance, OKX, or Bybit.

**Portfolio**: One User's ledger of selected Accounts and Capital Flows. Each
Portfolio organizes exchange access as `Platform -> Identity -> Account`.

**Identity**: One login recognized by a Platform, belonging to exactly one
Platform and Portfolio. Main and sub-account logins are unrelated Identities
with separate Connections; a main-account key does not collect sub-accounts. An
Identity is not a User, Account, API key, or Connection.

**Account Type**: A kind of wallet, trading account, or product account an
Adapter knows how to collect within an Account Structure. The application-owned
`manual` Account Type is independent of the Adapter catalog.

**Account**: One Account Type selected by the User for an Identity. Only
selected Accounts are collected and valued. Every Identity also has one fixed,
always-selected Manual Account for user-entered signed quantities.

**Portfolio Boundary**: Selected Accounts under enabled Identities. Value
outside the boundary is invisible to Plutus. View filters do not change it; a
disabled or invalid Connection can prevent collection without removing its
Accounts.

User creation creates a Portfolio named `default` that remains undeletable after
renaming. Other Portfolios can be deleted with their scoped data. Portfolios are
not shared, and Identities cannot move between Portfolios after Observations
exist.

## Account Structure, Margin Mode, and Connection

**Account Structure**: How a Platform arranges an Identity's Accounts, deciding
which Account Types exist. It is declared at creation and immutable. Avoid
"Account Mode" or "account level" as synonyms.

Validation and every Collection Run verify the structure where Platform
interfaces permit. A mismatch fails with `unsupported`; compatible earlier
reports can supply stale values. An enabled Connection stays enabled and
scheduled, but the run does not retry `unsupported` failures. Switching
structure on the Platform requires a new Identity and archiving the old one.

Kraken exposes its structure flag through Futures access. Validation without
Futures credentials cannot check it, and Earn-only collection reads no wallet
snapshot because Earn is shared by both structures. See
[Kraken's structure check](./docs/adapters/kraken/mapping.md#account-structure-check).

**Margin Mode**: A Platform's margin regime for a whole trading account within
an Account Structure. It is observed, never chosen in Plutus, may change between
runs, and decides which measures an Account Report can contain. Only some
structures have one. A Position's isolated or cross setting is distinct.

Mode changes are stored and audited. Unknown modes fail with `unsupported`;
unavailable measures remain null.

**Connection**: An Identity's read-only API credentials. Its first successful
validation binds the Identity to an external login reference. Replacing
credentials preserves the Identity and Accounts; credentials reaching another
login are refused. Authentication and authorization failures invalidate the
Connection; Account-level product failures do not automatically invalidate it.

**Adapter**: The component translating one Platform's responses into normalized
Observations. It declares supported Account Structures and Account Types and
reports observed Margin Modes. A runtime Platform payload is not an Observation.

## Collection and Account Reports

**Collection Cycle**: One scheduled or manual pass over a Portfolio's enabled
Identities, producing one Valuation Point after collection and valuation
complete. Private collection uses enabled Connections; Manual Accounts do not
require a usable Connection. Avoid "refresh", "sync", or "snapshot" as synonyms.

**Collection Run**: Work for one Connection within a Collection Cycle. The cycle
records success or failure for every Account in its boundary, including skipped
Accounts whose Connection cannot collect. Infrastructure failures may interrupt
a cycle before its point is written.

**Observation**: A normalized Platform fact obtained during a Collection Run,
with its Platform observation time. Observations have no editing API; later runs
produce new facts. They are distinct from raw responses and derived values.

**Account Report**: One Account's aligned collection facts: collection outcome,
reported equity when available, fixed Plutus-defined account-level measures,
Balances, Positions, Earn Positions, and Dual Positions. It is not a synonym for
equity or a generic snapshot. Adapters map measures by meaning and leave
unsupported values null.

**Balance**: An Asset Listing's equity in an Account Report, with availability,
liability, and unrealized-profit measures. Quantity or holding is not a synonym
for its equity measure.

**Position**: An open directional contract holding in an Account Report, with
side, size, prices, and margin measures. Opposing long and short holdings remain
separate. A Position is not another Balance to add to Account equity.

**Position Mode**: Whether a contract Position is one-way (opposing trades net
against it) or hedged (separate long and short holdings). It is distinct from
the Position's direction and from Account Margin Mode.

**Position Margin Setting**: Whether a Position uses cross collateral or
isolated funds. Portfolio Margin is an Account regime, not a third
cross/isolated setting.

**Position Size**: Base quantity for a linear contract or Quote face amount for
an inverse contract. Contract counts require the Instrument's per-contract size.

**Position Margin**: The position initial requirement excluding orders for cross
positions, or allocated funds excluding unrealized PnL for isolated positions.
Initial requirements follow the Platform's convention, including fees or hedge
treatment when reported. A documented initial-leverage calculation can supply an
otherwise unavailable requirement; it is not an allocation of shared collateral.
Source and calculated amounts remain distinct, with a known currency and basis.
Unavailable allocation stays unknown. Portfolio Margin does not by itself imply
that an individual initial requirement is unavailable.

Displayed Position leverage belongs with Position Margin: cross uses the
reported Position leverage, while isolated uses absolute USD notional divided by
the USD value of its allocated margin. It does not overwrite source leverage or
margin. Portfolio Margin is displayed as the Account regime without a position
leverage multiplier.

**Complete Set**: A successful Account Report contains all of that Account's
Balances, Positions, Earn Positions, and Dual Positions; anything absent is no
longer held. A failure cannot clear holdings.

**Stale**: A failed Account uses its latest earlier successful Account Report
with the same scope, retaining its observation time and displaying its age.
Without compatible history, the Account is missing and its value is unknown.
Freshness and completeness are independent: stale value can be known, while a
fresh report can lack prices.

Platform reads are not atomic snapshots. Accounts and interfaces may observe
different instants, including temporary settlement gaps or double counts.

## Manual balances

A **Manual Account** belongs to one Identity and records quantities missing from
its Platform API. It remains selected with disabled, invalid, or unconfigured
Connections. Identity archive removes it from the boundary; history and current
records remain. External bank assets are outside the offered scope.

One normalized Asset code has one current signed quantity. Duplicate addition
requires editing; zero clears it. Negative quantities represent liabilities or
offsets. Revisions are immutable. Saving waits for the next Collection Cycle,
which freezes its input selection before Platform reads; retries use that same
selection. Manual complete-set reports are distinct from exchange Observations,
with collection time separate from last user modification time. Successful
Manual reports do not count as successful private Identity collection.

Manual uses calculated valuation and eligible own-Platform public prices, even
for Assets held only in Manual. USD, USDT, and USDC count as 1 USD. Missing
prices retain signed quantities, name omissions, and make totals incomplete.
Mapping and Listing Exclusions apply; excluded current records remain editable.

Effective quantity changes mark comparisons crossing their Valuation Point
incomplete while retaining their computed difference. They do not create Capital
Flows. No-op edits, price changes, and edits reverted before collection do not
mark adjustments. Existing Identities initially add Manual as zero continuity,
including first nonzero recording; new Identity entry and archive retain signed
boundary rules. Users clear or adjust Manual when the API includes settled
amounts; Plutus does not infer settlement or prevent that double counting.

Current editable and pending quantities are displayed separately from collected
Balance values. Accounts hide from Overview only after a fresh successful
complete zero valuation confirms no nonzero Balances or holdings of any kind.
Stale, failed, unknown, and offsetting holdings remain visible. Hiding changes
neither selection nor the Portfolio Boundary or history.

## Assets and valuation

**Asset**: A unit of financial value identified by normalized symbol across
Platforms, such as BTC or USDT. Different symbols remain distinct Assets even
when economically related; USD and USDT share valuation, not identity.

**Asset Listing**: One Platform's native symbol for an Asset. It maps
automatically by normalized symbol unless the User overrides the mapping. Use
Asset Listing when the Platform-specific spelling matters.

**Instrument**: A Platform-specific contract or market identified by symbol,
product kind, settlement Asset, and expiry where applicable, discovered from
Positions. Perpetual and delivery contracts are distinct Instruments; neither is
interchangeable with an Asset.

**Valuation Asset**: USD, the unit for all derived values. USD, USDT, and USDC
count as exactly 1 USD; Plutus does not detect stablecoin de-pegs. Other Assets,
including other stablecoins, need prices. Source quantities and prices remain
visible.

**Price Observation**: A market price for an Asset on a Platform at an
observation time, quoted in a stated Asset. Valuation accepts USD, USDT, or USDC
quotes at or before the report's observation time, no more than 24 hours old.
The accepted policy requires the Account's own Platform; the unresolved
[implementation discrepancy](./docs/architecture.md#known-implementation-discrepancy)
is not a change to that policy.

**Account Valuation**: One Account's USD value at a Valuation Point, with its
method, freshness, and completeness. State the method rather than calling it
"equity" or "Balance total" without qualification.

| Report source                                    | Valuation method       | Result                                            |
| ------------------------------------------------ | ---------------------- | ------------------------------------------------- |
| Successful report with equity                    | Reported               | Use the Platform's Account equity                 |
| Successful report without equity                 | Calculated             | Sum Balance equity times price                    |
| Failed report with compatible successful history | Source report's method | Value the compatible report as stale              |
| Failed report without compatible history         | Missing                | Unknown Account value; incomplete Portfolio total |

Balance breakdowns, Positions, Earn Positions, and Dual Positions describe value
already represented in reported equity and never add it again.

**Valuation Point**: The Portfolio's total of Account Valuations at the end of
one Collection Cycle. Derived results can be rebuilt; avoid "snapshot",
"projection", or "generation" as synonyms.

**Incomplete**: Required non-excluded value cannot be determined. Calculated and
Portfolio totals sum known value and name omissions, including missing Accounts
and Balances without equity or prices. A missing Account's own value is unknown.
Unknown is never zero or silently omitted.

### Listing Exclusions and mapping changes

**Listing Exclusion**: A reviewed, reversible rule omitting an Asset Listing's
Balances from future calculated Account Valuations in one Portfolio. It requires
an auditable reason and applies to cycles created after the change.

Exclusions remove the listing from calculated values and ordinary Balance reads
without inventing a zero price. Observations remain stored; reported Account
equity, Positions, and Capital Flows are unaffected. The Catalog and audit trail
expose the rule and reason.

Exclusions do not rewrite existing points. Rebuilding uses the policy effective
at each cycle's creation. Value Change is incomplete when a relevant listing's
treatment differs between endpoint cycles; exclusion and restoration between
endpoints with matching treatment do not alone make it incomplete. Policy
changes are not Capital Flows or Boundary Events.

Mapping corrections enqueue affected Portfolio rebuilds. Evaluation times and
observed facts remain; derived Account values, Boundary Events, and totals can
change. Rebuild mechanisms belong in
[architecture](./docs/architecture.md#persistence-and-rebuilds).

## Dual Investment and Earn completeness

**Earn Position**: A descriptive holding within a selected Earn Account. Its
source granularity is a product aggregate, strategy aggregate, or individual
position; an aggregate is not an invented subscription. A multi-Asset product
remains one Earn Position. Its Amount is reported principal, total holdings, or
allocated holdings, matching value already represented in Balances.

Rates retain APR/APY, estimated ranges, and tier or additional-reward meaning. A
configured minimum lending rate is a preference, not a reported return. Rewards
retain their Asset and cumulative, pending, estimated, or other reported scope;
they are not added together or added again to Account Valuation. Subscription,
maturity, delivery, and redemption-stage times are distinct. A missing date is
never calculated from a duration. Product Status describes only reported
lifecycle phases; collection health belongs to Observed.

Optional invalid metadata retains known amounts with a display issue. Normal
source unavailability is not a core-data alarm. A report without collected Earn
details, including pre-delivery history, exposes known Balance summaries and
explicit missing detail rather than claiming an empty set of product holdings.
Adding descriptions changes neither Account scope nor financial completeness.

**Dual Investment**: A product accepting principal in one Asset and settling in
that Asset or its paired Asset according to target price and expiry. Binance,
OKX, and Bybit expose it as a separately selected Account from ordinary Earn.
This product boundary does not imply a separate transferable wallet.

**Dual Position**: One outstanding subscription in an Account Report, with its
principal and available product terms. Separate subscriptions stay separate; a
Dual Position is distinct from a Balance or futures Position.

Principal is grouped by invested currency into Balances and priced normally.
Expected yield is excluded. Target, expiry, and annualized rate describe that
principal without adding value; available and withdrawable amounts are not
invented. Missing optional terms remain null with display issues. Missing
subscription identity, missing or nonpositive principal, or contradictory terms
fail normalization.

Selecting either Earn or Dual reads both product groups once for a combined
completeness check; only selected Accounts emit reports and contribute value.
Selecting neither reads neither. Necessary auxiliary failures or unexplained
aggregate gaps fail both selected dependents; unrelated Accounts retain normal
failure isolation.

The check compares combined principal with the Platform's Earn aggregate before
Listing Exclusions. A shortfall greater than
`max(1% of the Platform total, 1 USD)` fails with `unsupported`. Missing prices
skip the comparison; selected unpriced Balances retain normal omissions.
Binance's dedicated Dual aggregate is optional diagnostics, not reported equity
or a runtime dependency.

Principal stays held in the running states defined by each
[Platform mapping](./README.md#documentation). Only a successful report removes
a settled subscription. Bybit pending orders without Positions are not guessed
into holdings. Settlement and payout reads may temporarily disagree.

### Report scope compatibility

Reports preserve product scope, including failures. Stale fallback requires
equal scope; an old combined Earn report cannot supply ordinary Earn value after
the split.

The first observed split is reclassification, not an additional contribution. An
unknown ordinary-Earn portion does not produce a guessed withdrawal when only
Dual remains. If the entire old slice leaves, its known combined withdrawal is
retained. Value Changes spanning a scope transition are incomplete even when
endpoint holdings are fully valued; rebuilding preserves this rule. Scope
versions belong in
[architecture](./docs/architecture.md#persistence-and-rebuilds), and
existing-data upgrades belong in
[deployment](./docs/deployment.md#dual-investment-split-upgrade).

## Capital Flows, Boundary Events, and Value Change

**Capital Flow**: A User-recorded contribution or withdrawal crossing the
Portfolio Boundary, expressed as Asset quantity at an effective time, with an
optional Account. Internal transfers and balance changes are not Capital Flows.
Select an Account before funding it: funding an unselected Account leaves the
boundary and must be recorded manually.

**Boundary Event**: An Account entering or leaving the boundary through
selection, deselection, or Identity archive. Entry contributes its first valued
amount; exit withdraws its last. Unknown amounts create named omissions. The
first point has no preceding boundary and creates no Boundary Events.

**Value Change**: The absolute USD difference between observed points after
accounting for both manual flows and Boundary Events. Avoid profit, P&L, return,
or performance as synonyms.

```text
Value Change = end value - start value - contributions + withdrawals
```

Flows enter the calculation during `(start.evaluatedAt, end.evaluatedAt]`. The
User can supply an explicit nonnegative USD value. Otherwise USD-equivalent
quantities value at 1 USD; other Assets require a price at or before the flow's
effective time, no more than 24 hours old. Automatic values are stored when the
flow is created or updated. Later prices do not automatically repair an unvalued
flow; missing values make affected Value Changes incomplete and name the flow.

## Calendar periods

**Period Close**: The last Valuation Point within a calendar day, ISO week,
month, quarter, or year in the Portfolio's IANA timezone. Weeks start Monday. A
period covers `[start, end)`; a point at its end belongs to the next period.
Without points, the API returns a null close and an explicit reason. A larger
period missing days is incomplete even if it has a closing point.

Calendar Value Change selects the latest point strictly before each period
boundary. Custom ranges select the latest point at or before each requested
endpoint. Calculations use observed points, never invented endpoint values.
Missing endpoints, incomplete source values, unvalued flows or boundaries, and
relevant policy or scope changes remain explicit. Calendar results also report
missing days; custom comparisons do not apply that calendar rule. Changing
timezone changes query boundaries without rebuilding points.
