# Domain rules

These are the current business rules for Plutus. [Glossary](../GLOSSARY.md)
defines terms, [product scope](./product-scope.md) defines offered Accounts,
[presentation](./presentation.md) defines display behavior, and
[ADRs](./adr/README.md) explain decisions. Technical mechanisms belong in
[architecture](./architecture.md).

## Ownership and the Portfolio Boundary

Each User owns independent, unshared Portfolios. Each Identity belongs to one
Platform and Portfolio. Main and sub-account logins use separate Connections; a
main-account key does not collect sub-accounts. Identities cannot move between
Portfolios after Observations exist.

User creation creates a Portfolio named `default` that remains undeletable after
renaming. Other Portfolios can be deleted with their scoped data.

Only selected Accounts under enabled Identities belong to the Portfolio
Boundary. Every Identity has one fixed, always-selected Manual Account. View
filters do not change the boundary; disabled or invalid Connections can prevent
collection without removing their Accounts.

## Account Structure, Margin Mode, and Connection

Account Structure is declared at Identity creation and immutable. Validation and
every Collection Run check it where Platform interfaces permit. A mismatch fails
explicitly; compatible successful history can still supply stale values. The
enabled Connection stays enabled and scheduled. Switching structure on the
Platform requires a new Identity and archiving the old one. Platform-specific
check limitations belong in the [mapping guides](../README.md#documentation).

Margin Mode is observed, never chosen in Plutus, and may change between runs.
Only some structures have one. It determines available Account Report measures;
unknown modes fail explicitly and unavailable measures remain unknown. A
Position's cross or isolated setting is distinct from the Account regime.

A Connection's first successful validation binds its Identity to an external
login reference. Replacing credentials preserves the Identity and Accounts;
credentials for another login are refused. Authentication and authorization
failures invalidate the Connection; Account-level product failures do not
automatically invalidate it.

## Collection and Account Reports

A Collection Cycle records success or failure for every Account in its boundary,
including skipped Accounts whose Connections cannot collect. Private collection
uses enabled Connections; Manual Accounts do not require a usable Connection.
Infrastructure failures may interrupt a cycle before its Valuation Point exists.

Each Account Report aligns reported equity, fixed Plutus-defined account
measures, and holdings. Measures are mapped by meaning; unsupported values
remain unknown. Observations are immutable source facts, distinct from raw
Platform responses and derived results.

Only a successful Account Report establishes a Complete Set. Absent holdings are
no longer held; failure cannot clear them. A failed Account uses its latest
earlier successful report with equal scope, retaining its observation time and
age. Without compatible history it is missing and its value is unknown.

Freshness and completeness are independent: stale value can be known, while a
fresh report can lack prices. Platform reads are not atomic snapshots; Accounts
and interfaces can observe different instants, including settlement gaps or
double counts.

### Contract quantities and margin

Opposing long and short Positions remain separate. A Position does not add
another Balance to Account equity. Linear Position Size is Base quantity;
inverse Size is Quote face amount. Source contract counts require the
Instrument's effective per-contract size exactly once; normalized quantities do
not. Mark Price does not change Size.

Cross Position Margin means the initial requirement excluding orders. Isolated
Margin means allocated funds excluding unrealized PnL. Requirements follow
Platform conventions, including reported fees and hedge treatment. Preserve
source amount, currency, and basis; calculated amounts remain distinguishable.
Missing allocation stays unknown. Unsupported optional measures stay unknown
without a core-data alarm. Portfolio Margin does not itself make an individual
initial requirement unavailable, and shared collateral or risk-unit requirements
are never allocated across Positions.

A verified Platform-specific calculation may supply a missing requirement; there
is no universal notional/leverage formula. Source interpretation and supported
calculations belong in the provenance mappings:
[Binance](./platform/binance/mapping.md#position-presentation-provenance),
[OKX](./platform/okx/mapping.md#position-presentation-provenance),
[Bybit](./platform/bybit/mapping.md#position-presentation-provenance), and
[Kraken](./platform/kraken/mapping.md#position-presentation-provenance).

Margin and unrealized PnL are presented in Settle currency. USD-equivalent
amounts convert at 1:1; Base Settle with USD-equivalent Quote can use positive
Mark. Unsupported conversions and ambiguous source units remain unknown.

Displayed Cross leverage is reported Position leverage. Displayed Isolated
leverage is absolute USD notional divided by the USD value of allocated margin,
using source decimals before rounding. Missing inputs leave leverage unknown.
Actual Isolated takes precedence over Account Portfolio Margin; otherwise that
regime has no position leverage multiplier. Display derivations do not overwrite
source leverage or margin, and current Identity settings cannot establish a
historical Report's regime.

## Manual balances

Manual supplements quantities missing from an Identity's Platform API; external
bank assets are outside scope. It remains selected with disabled, invalid, or
unconfigured Connections. Identity archive removes it from the boundary while
retaining its history and current records.

One normalized Asset code has one current signed quantity. Duplicate addition
requires editing; zero clears it. Negative quantities represent liabilities or
offsets. Revisions are immutable. Changing an Asset code requires deleting and
creating a record. Saving takes effect at the next Collection Cycle, whose
chosen inputs remain fixed across retries. Manual complete-set reports are
distinct from exchange Observations; collection time is separate from the last
User modification time. A successful Manual report does not count as successful
private Identity collection.

Manual uses calculated valuation and eligible own-Platform public prices,
including Assets held only in Manual. Mapping and Listing Exclusions apply;
excluded current records remain editable. Missing prices retain signed
quantities, name omissions, and make totals incomplete.

Effective quantity changes mark comparisons crossing their Valuation Point
incomplete while retaining the calculated difference. They do not create Capital
Flows. No-op edits, price changes, and edits reverted before collection do not
mark adjustments. For Identities already represented at the preceding Valuation
Point, first Manual recording uses zero continuity rather than a boundary
contribution; a first nonzero quantity still marks an adjustment. New Identity
entry and archive retain signed boundary rules. Users clear or adjust Manual
when the API includes settled amounts; Plutus does not infer settlement or
prevent double counting.

Current editable and pending quantities remain distinct from collected values.
An Account hides from Overview only after a fresh successful complete zero
valuation confirms no nonzero Balances or holdings of any kind. Stale, failed,
unknown, and offsetting holdings remain visible. Hiding changes neither
selection nor the Portfolio Boundary, history, or Account counts.

## Assets and valuation

Assets are identified by normalized symbol across Platforms. Different symbols
remain distinct even when economically related; USD and USDT share valuation,
not identity. Asset Listings map automatically by normalized symbol unless the
User overrides them. Instruments are discovered from Positions; perpetual and
delivery contracts remain distinct from each other and from Assets.

USD is the only Valuation Asset. USD, USDT, and USDC count as exactly 1 USD;
Plutus does not detect stablecoin de-pegs. Other Assets, including other
stablecoins, require prices. Source quantities and prices remain available.

Eligible prices use USD, USDT, or USDC quotes at or before the report's
observation time, no more than 24 hours old. The accepted policy requires the
Account's own Platform. The
[known implementation discrepancy](./architecture.md#known-implementation-discrepancy)
does not change that policy.

| Report source                                    | Valuation method       | Result                                            |
| ------------------------------------------------ | ---------------------- | ------------------------------------------------- |
| Successful report with equity                    | Reported               | Use the Platform's Account equity                 |
| Successful report without equity                 | Calculated             | Sum Balance equity times price                    |
| Failed report with compatible successful history | Source report's method | Value the compatible report as stale              |
| Failed report without compatible history         | Missing                | Unknown Account value; incomplete Portfolio total |

Balance breakdowns, Positions, Earn Positions, and Dual Positions describe value
already represented in reported equity and never add it again. Calculated and
Portfolio totals sum known value and name omissions, including missing Accounts
and Balances without equity or prices. A missing Account's own value is unknown.
Unknown is never zero or silently omitted.

### Listing Exclusions and mapping changes

A Listing Exclusion requires an auditable reason and applies to cycles created
after the change. It removes the listing from calculated values and ordinary
Balance reads without inventing a zero price. Observations remain stored;
reported equity, Positions, and Capital Flows are unaffected. The Catalog and
audit trail expose the rule and reason.

Exclusions do not rewrite existing points. Rebuilding uses the policy effective
at each cycle's creation. Value Change is incomplete when a relevant listing's
treatment differs between endpoint cycles; exclusion and restoration between
endpoints with matching treatment do not alone make it incomplete. Policy
changes are neither Capital Flows nor Boundary Events.

Mapping corrections rebuild affected Portfolios. Evaluation times and observed
facts remain; derived Account values, Boundary Events, and totals can change.
Rebuild mechanisms belong in
[architecture](./architecture.md#persistence-and-rebuilds).

## Dual Investment and Earn completeness

Earn descriptions retain source granularity: product aggregate, strategy
aggregate, or individual Position. An aggregate is not an invented subscription;
a multi-Asset product remains one Earn Position. Amount is reported principal,
total holdings, or allocated holdings, matching value already in Balances.

Rates preserve APR/APY, estimated ranges, tiers, and additional-reward meaning.
A configured minimum lending rate is a preference, not a reported return.
Rewards retain Asset and cumulative, pending, estimated, or other reported
scope; they are neither added together nor added to Account Valuation. Do not
convert compounding conventions or reconstruct original principal.

Subscription, maturity, delivery, and redemption-stage times are distinct. A
missing date is never calculated from a duration. Product Status describes
reported lifecycle phases only. Optional invalid metadata retains known amounts
with a display issue; normal source unavailability is not a core-data alarm.
Reports without collected Earn details expose known Balance summaries and
explicit missing detail, never an invented empty product set. Description
availability changes neither scope nor financial completeness. Current product
data or evidence files cannot backfill missing historical descriptions.

Binance, OKX, and Bybit expose Dual Investment as an Account selected separately
from ordinary Earn. This product boundary does not imply a separate transferable
wallet. Each subscription remains a distinct Dual Position. Principal is grouped
by invested currency into Balances and priced normally; expected yield is
excluded. Target, expiry, and annualized rate describe principal without adding
value. Available and withdrawable amounts are not invented.

Missing optional terms remain unknown with display issues. Missing subscription
identity, missing or nonpositive principal, and contradictory terms fail
normalization. Principal stays held in the running states defined by each
[Platform mapping](../README.md#documentation). Only a successful report removes
a settled subscription. Bybit pending orders without Positions are not guessed
into holdings; settlement and payout reads may temporarily disagree.

Selecting either Earn or Dual reads both product groups once for a combined
completeness check; only selected Accounts emit reports and contribute value.
Selecting neither reads neither. Necessary auxiliary failures or unexplained
aggregate gaps fail both selected dependents; unrelated Accounts retain failure
isolation.

The check compares combined principal with the Platform's Earn aggregate before
Listing Exclusions. A shortfall greater than
`max(1% of the absolute Platform total, 1 USD)` fails explicitly. Missing prices
skip the comparison; selected unpriced Balances retain normal omissions.
Binance's dedicated Dual aggregate is optional diagnostics, not reported equity
or a runtime dependency.

### Report scope compatibility

Reports preserve product scope, including failures. Stale fallback requires
equal scope; combined Earn reports cannot supply ordinary Earn value.

A transition from combined Earn to separate ordinary Earn and Dual is
reclassification, not an additional contribution. An unknown ordinary-Earn
portion does not produce a guessed withdrawal when only Dual remains. If the
entire combined slice leaves, its known withdrawal is retained. Value Changes
spanning a scope transition are incomplete even when endpoint holdings are fully
valued; rebuilding preserves this rule. Scope versions belong in
[architecture](./architecture.md#persistence-and-rebuilds).

## Capital Flows, Boundary Events, and Value Change

Users record contributions and withdrawals across the Portfolio Boundary.
Internal transfers and balance changes are not Capital Flows. Select an Account
before funding it: funding an unselected Account leaves the boundary and must be
recorded manually.

Boundary entry contributes the Account's first valued amount; exit withdraws its
last. Unknown amounts create named omissions. The first Valuation Point has no
preceding boundary and creates no Boundary Events.

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

Periods use the Portfolio's IANA timezone; ISO weeks start Monday. Each period
covers `[start, end)`, so a point at its end belongs to the next period. Without
points its close is unknown with an explicit reason. A larger period missing
days is incomplete even if it has a closing point.

Calendar Value Change selects the latest point strictly before each period
boundary. Custom ranges select the latest point at or before each requested
endpoint. Calculations use observed points, never invented endpoint values.
Missing endpoints, incomplete source values, unvalued flows or boundaries, and
relevant policy or scope changes remain explicit. Calendar results also report
missing days for periods larger than a day; custom comparisons do not apply that
rule. Changing timezone changes query boundaries without rebuilding points.
