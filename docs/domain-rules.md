# Domain rules

These are the current business rules for Plutus. [Glossary](../GLOSSARY.md)
defines terms, [product scope](./product-scope.md) defines offered accounts,
[presentation](./presentation.md) defines display behavior, and
[ADRs](./adr/README.md) explain decisions. Technical mechanisms belong in
[architecture](./architecture.md).

## Ownership and the portfolio boundary

Each user owns independent, unshared portfolios. Each identity belongs to one
platform and portfolio. Assets, mappings, contract terms, prices, policies,
financial history, and audit records are portfolio-owned. Equal symbols or login
references in different portfolios establish no shared resource. User sessions
and the user-scoped idempotency retry window are the only shared lifetimes. Main
and sub-account logins use separate connections; a main-account key does not
collect sub-accounts. Identities cannot move between portfolios after
observations exist.

User creation creates a portfolio named `default` that remains undeletable after
renaming. Other portfolios can be deleted with their scoped data.

Only selected accounts under enabled identities belong to the portfolio
boundary. Every identity has one fixed, always-selected manual account. View
filters do not change the boundary; disabled or invalid connections can prevent
collection without removing their accounts.

## Account structure, margin mode, and connection

Account structure is declared at identity creation and immutable. Validation and
every collection run check it where platform interfaces permit. A mismatch fails
explicitly; compatible successful history can still supply stale values. The
enabled connection stays enabled and scheduled. Switching structure on the
platform requires a new identity and archiving the old one. Platform-specific
check limitations belong in the [mapping guides](../README.md#documentation).

Margin mode is observed, never chosen in Plutus, and may change between runs.
Only some structures have one. It determines available account report measures;
unknown modes fail explicitly and unavailable measures remain unknown. A
position's cross or isolated setting is distinct from the account regime.

A connection's first successful validation binds its identity to an external
login reference. Replacing credentials preserves the identity and accounts;
credentials for another login are refused. Authentication and authorization
failures invalidate the connection; account-level product failures do not
automatically invalidate it. Within one portfolio and platform, active
identities cannot bind the same login. Disablement and credential clearing
retain the binding; archival releases the active unique key. Reusing a login in
another portfolio is allowed. An enabled connection requires credentials and a
nonempty stable login reference.

## Collection and account reports

A collection cycle records success or failure for every account in its boundary,
including skipped accounts whose connections cannot collect. Private collection
uses enabled connections; manual accounts do not require a usable connection.
Infrastructure failures may interrupt a cycle before its valuation point exists.

Each account report aligns reported equity, fixed Plutus-defined account
measures, and holdings. Measures are mapped by meaning; unsupported values
remain unknown. Observations are immutable source facts, distinct from raw
platform responses and derived results.

Only a successful account report establishes a complete set. Absent holdings are
no longer held; failure cannot clear them. A failed account uses its latest
earlier successful report for the same account, retaining its observation time
and age. Account structure and product account types are immutable scope.
Without compatible history it is missing and its value is unknown.

Freshness and completeness are independent: stale value can be known, while a
fresh report can lack prices. Platform reads are not atomic snapshots; accounts
and interfaces can observe different instants, including settlement gaps or
double counts.

### Contract quantities and margin

Opposing long and short positions remain separate. A position does not add
another balance to account equity. Linear position size is base quantity;
inverse size is quote face amount. Source contract counts require the futures
contract's effective per-contract size exactly once; normalized quantities do
not. Mark price does not change size.

Cross position margin means the initial requirement excluding orders. Isolated
margin means allocated funds excluding unrealized PnL. Requirements follow
platform conventions, including reported fees and hedge treatment. Preserve
source amount, currency, and basis; calculated amounts remain distinguishable.
Missing allocation stays unknown. Unsupported optional measures stay unknown
without a core-data alarm. Portfolio margin does not itself make an individual
initial requirement unavailable, and shared collateral or risk-unit requirements
are never allocated across positions.

A verified platform-specific calculation may supply a missing requirement; there
is no universal notional/leverage formula. Source interpretation and supported
calculations belong in the provenance mappings:
[Binance](./platform/binance/mapping.md#position-presentation-provenance),
[OKX](./platform/okx/mapping.md#position-presentation-provenance),
[Bybit](./platform/bybit/mapping.md#position-presentation-provenance), and
[Kraken](./platform/kraken/mapping.md#position-presentation-provenance).

Margin and unrealized PnL are presented in settle currency. USD-equivalent
amounts convert at 1:1; base settle with USD-equivalent quote can use positive
mark. Unsupported conversions and ambiguous source units remain unknown.

Displayed cross leverage is reported position leverage. Displayed isolated
leverage is absolute USD notional divided by the USD value of allocated margin,
using source decimals before rounding. Missing inputs leave leverage unknown.
Actual isolated takes precedence over account portfolio margin; otherwise that
regime has no position leverage multiplier. Display derivations do not overwrite
source leverage or margin, and current identity settings cannot establish a
historical report's regime.

## Manual balances

Manual supplements quantities missing from an identity's platform API; external
bank assets are outside scope. It remains selected with disabled, invalid, or
unconfigured connections. Identity archive removes it from the boundary while
retaining its history and current records.

Each new manual account starts with empty revision 1 and no modification time.
One normalized asset code has one current signed quantity. Duplicate addition
requires editing; zero clears it. Negative quantities represent liabilities or
offsets. Revisions are immutable. Changing an asset code requires deleting and
creating a record. Saving takes effect at the next collection cycle that reads
manual inputs afterward; its recorded reports remain fixed across retries.
Manual complete-set reports are distinct from exchange observations; collection
time is separate from the last user modification time. A successful manual
report does not count as successful private identity collection.

Each effective user edit audits the changed asset, its previous and new
quantity, actor, and time. An absent entry remains distinct from a stored
quantity. This evidence survives cleanup of the referenced manual revisions;
no-op edits create neither a revision nor an audit event. Audits follow the
ten-year historical retention window.

Manual uses calculated valuation and eligible own-platform public prices,
including assets held only in manual. Each manual report references the
immutable revision it reported. A new point uses current portfolio mappings for
both manual and platform reports; committed points never change or admit future
prices. Asset exclusions apply; excluded current records remain editable.
Missing prices retain signed quantities, name omissions, and make totals
incomplete.

Effective quantity changes mark comparisons crossing their valuation point
incomplete while retaining the calculated difference. They do not create capital
flows. No-op edits, price changes, and edits reverted before collection do not
mark adjustments. For identities already represented at the preceding valuation
point, first manual recording uses zero continuity rather than a boundary
contribution; a first nonzero quantity still marks an adjustment. New identity
entry and archive retain signed boundary rules. Users clear or adjust manual
when the API includes settled amounts; Plutus does not infer settlement or
prevent double counting.

Current editable and pending quantities remain distinct from collected values.
An account hides from Overview only after a fresh successful complete zero
valuation confirms no nonzero balances or holdings of any kind, and no current
manual quantities remain. Stale, failed, unknown, and offsetting holdings remain
visible. Hiding changes neither selection nor the portfolio boundary, history,
or account counts.

## Assets and valuation

Assets are identified by portfolio, platform, and normalized asset symbol. Each
maps to a canonical asset, automatically by normalized symbol unless the user
overrides it. Equal canonical asset symbols do not merge asset records,
holdings, or exclusions. Different canonical asset symbols remain distinct even
when economically related; USD and USDT share valuation, not identity. Futures
contracts are discovered from positions; perpetual and delivery contracts remain
distinct from each other and from assets.

USD is the only valuation asset. USD, USDT, and USDC count as exactly 1 USD;
Plutus does not detect stablecoin de-pegs. Other canonical assets, including
other stablecoins, require prices. Source quantities and prices remain
available.

Eligible prices belong to the account's portfolio and own platform. Only USD,
USDT, or USDC quotes are stored; other quotes leave the asset unpriced. A cycle
attempt stores at most one price per platform and canonical asset, shared by
that platform's identities; later cycles store new observations. Both retrieval
time and source price time must be at or before the source report's recording
time; source price time must be no more than 24 hours old at that cutoff.
Without a source timestamp, retrieval time is price time. Eligibility is
filtered before selecting the latest price, breaking ties by retrieval time and
observation ID. Stale valuation retains the source report's cutoff.
Cross-platform fallback is not permitted.

Each position references the futures contract whose terms it observed, including
currencies, kind, expiry, and contract size. Changed terms under a symbol record
another futures contract; repeated identical terms reuse it, including A → B →
A. Observation bounds do not claim exchange-effective dates, and newer terms do
not rewrite older positions.

| Report source                                    | Valuation method       | Result                                             |
| ------------------------------------------------ | ---------------------- | -------------------------------------------------- |
| Successful report with equity                    | Reported               | Use the platform's account equity                  |
| Successful report without equity                 | Calculated             | Sum balance values and explicit source adjustments |
| Failed report with compatible successful history | Source report's method | Value the compatible report as stale               |
| Failed report without compatible history         | Missing                | Unknown account value; incomplete portfolio total  |

Balance breakdowns, positions, earn positions, and dual positions describe value
already represented in reported equity and never add it again. Calculated and
portfolio totals sum known value and name omissions, including missing accounts
and balances without equity or prices. A missing account's own value is unknown.
Unknown is never zero or silently omitted.

### Asset exclusions and mapping changes

An asset exclusion requires an auditable reason and applies to cycles created
after the change. It removes the asset from calculated values and ordinary
balance reads without inventing a zero price. Observations remain stored;
reported equity, positions, and capital flows are unaffected. The Catalog and
audit trail expose the rule and reason.

Exclusions do not rewrite existing points. Valuation uses the policy effective
at each cycle's creation: the highest policy version whose effective time is
strictly earlier than cycle creation wins. Value change is incomplete when a
relevant asset's treatment differs between endpoint cycles; exclusion and
restoration between endpoints with matching treatment do not alone make it
incomplete. Policy changes are neither capital flows nor boundary events.

Mapping changes affect subsequent points in their owning portfolio. An actual
adopted mapping change for a nonzero calculated asset marks comparisons crossing
that point incomplete, retaining the calculated amount. A change of calculation
version has the same effect. No-op mappings, unadopted changes reverted before
collection, and descriptive mappings on reported accounts create no marker. The
asset must have a nonzero, non-excluded calculated balance at both adjacent
points. Reappearance after an absent or excluded point creates no mapping
marker. Adopted A-to-B-to-A changes remain two events. These are changes of
valuation policy, not capital flows. Exceptional historical corrections require
deliberate operator reconciliation; there is no automatic rebuild or repair
workflow.

## Dual investment and earn completeness

Earn descriptions retain source granularity: product aggregate, strategy
aggregate, or individual position. An aggregate is not an invented subscription;
a multi-asset product remains one earn position. Amount is reported principal,
total holdings, or allocated holdings, matching value already in balances.

Rates preserve APR/APY, estimated ranges, tiers, and additional-reward meaning.
A configured minimum lending rate is a preference, not a reported return.
Rewards retain asset and cumulative, pending, estimated, or other reported
scope; they are neither added together nor added to account valuation. Do not
convert compounding conventions or reconstruct original principal.

Subscription, maturity, delivery, and redemption-stage times are distinct. A
missing date is never calculated from a duration. Product status describes
reported lifecycle phases only. Optional invalid metadata retains known amounts
with a display issue; normal source unavailability is not a core-data alarm.
Successful earn reports include normalized product descriptions consistent with
their balances. Only a successful empty report establishes an empty product set.
Optional descriptive omissions do not invalidate known account equity. Current
product data or evidence files cannot invent source facts.

Binance, OKX, and Bybit expose dual investment as an account selected separately
from ordinary earn. This product boundary does not imply a separate transferable
wallet. Each subscription remains a distinct dual position. Principal is grouped
by invested currency into balances and priced normally; expected yield is
excluded. Target, expiry, and annualized rate describe principal without adding
value. Available and withdrawable amounts are not invented.

Missing optional terms remain unknown with display issues. Missing subscription
identity, missing or nonpositive principal, and contradictory terms fail
normalization. Principal stays held in the running states defined by each
[platform mapping](../README.md#documentation). Only a successful report removes
a settled subscription. Bybit pending orders without positions are not guessed
into holdings; settlement and payout reads may temporarily disagree.

Selecting either earn or dual reads both product groups once for a combined
completeness check; only selected accounts emit reports and contribute value.
Selecting neither reads neither. Necessary auxiliary failures or unexplained
aggregate gaps fail both selected dependents; unrelated accounts retain failure
isolation.

The check compares combined principal with the platform's earn aggregate before
asset exclusions. A shortfall greater than
`max(1% of the absolute platform total, 1 USD)` fails explicitly. Missing prices
skip the comparison; selected unpriced balances retain normal omissions.
Binance's dedicated dual aggregate is optional diagnostics, not reported equity
or a runtime dependency.

## Capital flows, boundary events, and value change

Users record contributions and withdrawals across the portfolio boundary.
Internal transfers and balance changes are not capital flows. Select an account
before funding it: funding an unselected account leaves the boundary and must be
recorded manually.

Boundary entry contributes the account's first valued amount; exit withdraws its
last. Unknown amounts create named omissions. The first valuation point has no
preceding boundary and creates no boundary events.

```text
Value change = end value - start value - contributions + withdrawals
```

Flows enter the calculation during `(start.evaluatedAt, end.evaluatedAt]`. The
user can supply an explicit nonnegative USD value. Otherwise USD-equivalent
quantities value at 1 USD; other canonical assets require a price at or before
the flow's effective time, no more than 24 hours old. Use the selected account's
platform, or an explicitly selected pricing platform for portfolio-level flows.
Without one, non-USD value remains unknown. Store the pricing method, adopted
price, latest pricing attempt time, and calculation version. Changes to
automatically priced inputs revalue the flow; an explicit USD amount remains
until explicitly replaced or cleared. Note-only edits preserve value and
provenance. Later prices do not automatically repair an unvalued flow; missing
values make affected value changes incomplete and name the flow.

## Calendar periods

Periods use the portfolio's IANA timezone; ISO weeks start Monday. Each period
covers `[start, end)`, so a point at its end belongs to the next period. Without
points its close is unknown with an explicit reason. Calendar lists start with
the period containing the portfolio's first retained valuation point; earlier
periods are omitted, and an empty history has no calendar rows. Missing periods
after that start remain explicit, including gaps before the query's first point.
A larger period missing days is incomplete even if it has a closing point; days
before the first retained point's local day are outside its coverage.

Calendar value change selects the latest point strictly before each period
boundary. Custom ranges select the latest point at or before each requested
endpoint. Calculations use observed points, never invented endpoint values.
Missing endpoints, incomplete source values, unvalued flows or boundaries, and
relevant policy changes remain explicit. Calendar results also report missing
days for periods larger than a day; custom comparisons do not apply that rule.
Changing timezone changes query boundaries without changing committed points.

## Historical detail retention

Every committed valuation point's summary, completeness counts, manual
adjustment markers, adopted policy-change markers, and boundary events remain
for ten calendar years. Collection defaults to 30 minutes; every committed cycle
remains, including additional manual collections. Summary retention does not
downsample or merge points. Named omissions and account issues follow detail
retention. Commit is atomic and final; a retained result never recalculates.

Keep all detailed snapshots for at least 14 days. For older complete local days
inside the ten-year window, keep the final whole point, including incomplete or
stale points. Use the portfolio timezone at cleanup to choose anchors, and never
replace an archived anchor after a timezone edit. Cleanup does not alter
retained totals or completeness.

A historical snapshot request selects the latest retained full point inside the
window before the requested day's end, falling back across days if needed.
Return its actual cycle, evaluation time, total, and holdings together. Charts,
period closes, and value change continue to use every retained summary. They do
not substitute daily anchors for their endpoints.

The rolling historical boundary is the start of the portfolio-local day ten
calendar years before maintenance, with February 29 clamped to February's last
day when necessary. Once advanced, the boundary cannot move backward after a
clock or timezone change. Historical lists omit older internal dependencies;
comparisons requested before the boundary report a missing endpoint rather than
using incomplete remnants. A comparison starting at the boundary can use the
last preceding point as its opening value.

Expired history is removed in bounded background batches, including daily
snapshots, summaries, comparison evidence, completed cycles, audit events and
capital flows. Preserve the opening snapshot, each asset's state at that point,
and policy versions adopted at the start of every retained cycle, including a
cycle that starts before the window and finishes inside it. Flows between the
opening point and the window still contribute to its first interval and remain.
Retained exclusion audits pin the exact policy versions they describe.

Retained snapshots preserve their source reports, holdings, contract terms, and
adopted prices. Last-good sources remain available even for unselected accounts.
Current manual revisions and revisions referenced by retained reports remain;
the latest committed manual report still establishes applied state after its
summary expires. Current mappings and exclusion rules remain even if configured
more than ten years ago. Unreferenced old contracts and default, unconfigured
asset catalog entries can expire.

Unreferenced old prices retain one eligible daily quote per portfolio, platform,
and canonical asset within the historical window. Older anchors can expire once
retained valuations, capital flows and last-good source windows release them.
Ordinary same-platform, nonfuture, 24-hour eligibility still applies. A retained
daily quote does not guarantee a price for every arbitrary historical instant.

Retention bounds time-driven history growth under fixed workload; it is not a
hard quota on user-created configuration, pinned dependencies or metadata size.
Missing detail is distinct from a missing point or an unknown value.
