# Domain rules

These are the current business rules for Plutus. [Glossary](../GLOSSARY.md)
defines terms, [product scope](./product-scope.md) defines offered Accounts,
[presentation](./presentation.md) defines display behavior, and
[ADRs](./adr/README.md) explain decisions. Technical mechanisms belong in
[architecture](./architecture.md).

## Ownership and the Portfolio Boundary

Each User owns independent, unshared Portfolios. Each Identity belongs to one
Platform and Portfolio. Assets, mappings, contract terms, prices, policies,
financial history, and audit records are Portfolio-owned. Equal symbols or login
references in different Portfolios establish no shared resource. User sessions
and the User-scoped idempotency retry window are the only shared lifetimes. Main
and sub-account logins use separate Connections; a main-account key does not
collect sub-accounts. Identities cannot move between Portfolios after
Observations exist.

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
automatically invalidate it. Within one Portfolio and Platform, active
Identities cannot bind the same login. Disablement and credential clearing
retain the binding; archival releases the active unique key. Reusing a login in
another Portfolio is allowed. An enabled Connection requires credentials and a
nonempty stable login reference.

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
earlier successful report for the same Account, retaining its observation time
and age. Account Structure and product Account Types are immutable scope.
Without compatible history it is missing and its value is unknown.

Freshness and completeness are independent: stale value can be known, while a
fresh report can lack prices. Platform reads are not atomic snapshots; Accounts
and interfaces can observe different instants, including settlement gaps or
double counts.

### Contract quantities and margin

Opposing long and short Positions remain separate. A Position does not add
another Balance to Account equity. Linear Position Size is Base quantity;
inverse Size is Quote face amount. Source contract counts require the Futures
Contract's effective per-contract size exactly once; normalized quantities do
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

Each new Manual Account starts with empty revision 1 and no modification time.
One normalized Asset code has one current signed quantity. Duplicate addition
requires editing; zero clears it. Negative quantities represent liabilities or
offsets. Revisions are immutable. Changing an Asset code requires deleting and
creating a record. Saving takes effect at the next Collection Cycle that reads
Manual inputs afterward; its recorded reports remain fixed across retries.
Manual complete-set reports are distinct from exchange Observations; collection
time is separate from the last User modification time. A successful Manual
report does not count as successful private Identity collection.

Manual uses calculated valuation and eligible own-Platform public prices,
including Assets held only in Manual. Each Manual report references the
immutable revision it reported. A new point uses current Portfolio mappings for
both Manual and Platform reports; committed points never change or admit future
prices. Asset Exclusions apply; excluded current records remain editable.
Missing prices retain signed quantities, name omissions, and make totals
incomplete.

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
valuation confirms no nonzero Balances or holdings of any kind, and no current
Manual quantities remain. Stale, failed, unknown, and offsetting holdings remain
visible. Hiding changes neither selection nor the Portfolio Boundary, history,
or Account counts.

## Assets and valuation

Assets are identified by Portfolio, Platform, and normalized Asset Symbol. Each
maps to a Canonical Asset, automatically by normalized symbol unless the User
overrides it. Equal Canonical Asset Symbols do not merge Asset records,
holdings, or exclusions. Different Canonical Asset Symbols remain distinct even
when economically related; USD and USDT share valuation, not identity. Futures
Contracts are discovered from Positions; perpetual and delivery contracts remain
distinct from each other and from Assets.

USD is the only Valuation Asset. USD, USDT, and USDC count as exactly 1 USD;
Plutus does not detect stablecoin de-pegs. Other Canonical Assets, including
other stablecoins, require prices. Source quantities and prices remain
available.

Eligible prices belong to the Account's Portfolio and own Platform. Only USD,
USDT, or USDC quotes are stored; other quotes leave the Asset unpriced. A cycle
attempt stores at most one price per Platform and Canonical Asset, shared by
that Platform's Identities; later cycles store new observations. Both retrieval
time and source price time must be at or before the source report's recording
time; source price time must be no more than 24 hours old at that cutoff.
Without a source timestamp, retrieval time is price time. Eligibility is
filtered before selecting the latest price, breaking ties by retrieval time and
observation ID. Stale valuation retains the source report's cutoff.
Cross-Platform fallback is not permitted.

Each Position references the Futures Contract whose terms it observed, including
currencies, kind, expiry, and contract size. Changed terms under a symbol record
another Futures Contract; repeated identical terms reuse it, including A → B →
A. Observation bounds do not claim exchange-effective dates, and newer terms do
not rewrite older Positions.

| Report source                                    | Valuation method       | Result                                             |
| ------------------------------------------------ | ---------------------- | -------------------------------------------------- |
| Successful report with equity                    | Reported               | Use the Platform's Account equity                  |
| Successful report without equity                 | Calculated             | Sum Balance values and explicit source adjustments |
| Failed report with compatible successful history | Source report's method | Value the compatible report as stale               |
| Failed report without compatible history         | Missing                | Unknown Account value; incomplete Portfolio total  |

Balance breakdowns, Positions, Earn Positions, and Dual Positions describe value
already represented in reported equity and never add it again. Calculated and
Portfolio totals sum known value and name omissions, including missing Accounts
and Balances without equity or prices. A missing Account's own value is unknown.
Unknown is never zero or silently omitted.

### Asset Exclusions and mapping changes

An Asset Exclusion requires an auditable reason and applies to cycles created
after the change. It removes the asset from calculated values and ordinary
Balance reads without inventing a zero price. Observations remain stored;
reported equity, Positions, and Capital Flows are unaffected. The Catalog and
audit trail expose the rule and reason.

Exclusions do not rewrite existing points. Valuation uses the policy effective
at each cycle's creation: the highest policy version whose effective time is
strictly earlier than cycle creation wins. Value Change is incomplete when a
relevant Asset's treatment differs between endpoint cycles; exclusion and
restoration between endpoints with matching treatment do not alone make it
incomplete. Policy changes are neither Capital Flows nor Boundary Events.

Mapping changes affect subsequent points in their owning Portfolio. An actual
adopted mapping change for a nonzero calculated Asset marks comparisons crossing
that point incomplete, retaining the calculated amount. A change of calculation
version has the same effect. No-op mappings, unadopted changes reverted before
collection, and descriptive mappings on reported Accounts create no marker.
Adopted A-to-B-to-A changes remain two events. These are changes of valuation
policy, not Capital Flows. Exceptional historical corrections require deliberate
operator reconciliation; there is no automatic rebuild or repair workflow.

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
Successful Earn reports include normalized product descriptions consistent with
their Balances. Only a successful empty report establishes an empty product set.
Optional descriptive omissions do not invalidate known Account equity. Current
product data or evidence files cannot invent source facts.

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
Asset Exclusions. A shortfall greater than
`max(1% of the absolute Platform total, 1 USD)` fails explicitly. Missing prices
skip the comparison; selected unpriced Balances retain normal omissions.
Binance's dedicated Dual aggregate is optional diagnostics, not reported equity
or a runtime dependency.

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
quantities value at 1 USD; other Canonical Assets require a price at or before
the flow's effective time, no more than 24 hours old. Use the selected Account's
Platform, or an explicitly selected pricing Platform for Portfolio-level flows.
Without one, non-USD value remains unknown. Store the pricing method, adopted
price, latest pricing attempt time, and calculation version. Changes to
automatically priced inputs revalue the flow; an explicit USD amount remains
until explicitly replaced or cleared. Note-only edits preserve value and
provenance. Later prices do not automatically repair an unvalued flow; missing
values make affected Value Changes incomplete and name the flow.

## Calendar periods

Periods use the Portfolio's IANA timezone; ISO weeks start Monday. Each period
covers `[start, end)`, so a point at its end belongs to the next period. Without
points its close is unknown with an explicit reason. A larger period missing
days is incomplete even if it has a closing point.

Calendar Value Change selects the latest point strictly before each period
boundary. Custom ranges select the latest point at or before each requested
endpoint. Calculations use observed points, never invented endpoint values.
Missing endpoints, incomplete source values, unvalued flows or boundaries, and
relevant policy changes remain explicit. Calendar results also report missing
days for periods larger than a day; custom comparisons do not apply that rule.
Changing timezone changes query boundaries without changing committed points.

## Historical detail retention

Every committed Valuation Point's summary, completeness counts, Manual
adjustment markers, adopted policy-change markers, and Boundary Events remain
indefinitely. Named omissions and Account issues follow detail retention. Commit
is atomic and final. Retrying the same cycle reads its existing result, even
after mappings or calculation code change.

Keep all detailed snapshots for at least 30 days. For older complete local days,
keep the final whole point, including incomplete or stale points. Use the
Portfolio timezone at cleanup to choose anchors, and never replace an archived
anchor after a timezone edit. Cleanup does not alter totals or completeness.

A historical snapshot request selects the latest retained full point before the
requested day's end in the current timezone, falling back across days if needed.
Return its actual cycle, evaluation time, total, and holdings together. Charts,
period closes, and Value Change continue to use all summaries. They do not
substitute daily anchors for their endpoints.

Retained snapshots preserve their source reports, holdings, contract terms, and
adopted prices. Last-good sources remain available even for unselected Accounts.
Current Manual revisions and revisions referenced by retained reports remain.
Capital Flow price references never expire. Unreferenced old prices retain one
eligible daily quote per Portfolio, Platform, and Canonical Asset; ordinary
same-Platform, nonfuture, 24-hour eligibility still applies. A retained daily
quote does not guarantee a price for every arbitrary historical instant.

Old source details outside these dependencies can be removed. Retention trades
intraday investigation and historical recalculation for a practical storage
budget. Missing detail is distinct from a missing point or an unknown value.
