# Presentation

This guide owns current display and interaction rules.
[Product scope](./product-scope.md) defines offered views,
[domain rules](./domain-rules.md) define financial meaning, and
[architecture](./architecture.md) describes implementation. Display choices do
not change source facts, selection, or the Portfolio Boundary.

## Names and ordering

### Account aliases

Identity lists and creation forms use these compact labels without changing
canonical terms, API names, identifiers, or stored data:

| Platform | Account Structure aliases                             | Margin Mode aliases                                                                          |
| -------- | ----------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| Binance  | Classic Trading → Classic; Portfolio Margin → Unified | none                                                                                         |
| OKX      | Unified account → Unified                             | Spot mode → SPOT; Futures mode → FUTURES; Multi-currency margin → MCM; Portfolio margin → PM |
| Bybit    | Unified Trading Account 2.0 → Unified                 | Isolated Margin → ISO; Cross Margin → MCM; Portfolio Margin → PM                             |
| Kraken   | Separate wallets → Classic; Unified Wallet → Unified  | none                                                                                         |

The list joins structure and mode with `·`, using `UNKNOWN` while an OKX or
Bybit mode is unavailable. Known aliases render before the Platform catalog
loads. Unknown identifiers use the catalog name when available, otherwise the
identifier. There is no full-name disclosure. Position Margin Setting labels and
Position Portfolio Margin descriptions use their own wording.

Compact names improve scanning at the cost of native names and version details.
Shared aliases imply neither identical Account arrangements nor identical risk
models: Binance Unified is its supported Portfolio Margin structure; Bybit MCM
is its canonical Cross Margin mode.

### Catalog labels

Catalog uses Assets and Futures tabs, with Asset and Symbol as their main column
labels. Futures contains perpetual and delivery Futures Contracts. Holdings
retains Balances and Futures with Asset and Symbol columns. Catalog Assets
column filters provide Platform, Asset search, and Status (All, Active, and
Excluded) options.

Catalog, Balances, and Earn display Asset Symbol first. When its Canonical Asset
Symbol differs, show a secondary `Canonical Asset: ...` hint; omit redundant
hints. Earn amount units use Asset Symbol. Catalog and mapping dialogs omit full
names. Mapping dialogs label the source Asset and the target Canonical Asset.
Capital Flow inputs and columns use Canonical Asset because flows identify
cross-Platform value.

Catalog Futures Contract size uses native Base units for linear contracts and
Quote face amounts for inverse contracts. Binance linear quantities already use
Base units, so an absent recorded size displays `1` with the known Base symbol.
Digit-leading Base symbols use `×` to separate amount from symbol, for example
`1 × 1000PEPE`; do not strip prefixes or apply another multiplier. Other missing
sizes stay `—`. This display convention does not rewrite observed terms.

### Ordering

Lists, groups, and selectors use Platform, Identity, then Account order; views
with fewer dimensions use the corresponding part of that hierarchy.

- Platforms: Binance, OKX, Bybit, Kraken.
- Identities: case-insensitive display name, then immutable ID.
- Accounts: Account Type order within the declared Account Structure, then
  immutable ID. The [Platform catalog](../internal/platform/adapters/catalog.go)
  owns type order; Manual is last. Account names and Margin Mode do not affect
  order.
- Balances within an Account: Asset Symbol, Canonical Asset Symbol, Asset ID.
- Futures within an Account: Base Asset Symbol; linear before inverse; perpetual
  before delivery within each family; delivery expiry nearest first, unknown
  last; native Symbol; long before short; Platform Position reference. Missing
  Base follows known Bases and uses native Symbol, side, and reference. Do not
  infer Base from Symbol or merge SHIB with 1000SHIB.
- Earn within an Account: sorted invested Asset Symbol sequence, with Canonical
  Asset Symbol breaking Asset ties; Flexible, Fixed, Bonded, Timed, On-chain;
  maturity with unknown last; stable source reference. Multi-Asset products stay
  together.
- Dual within an Account: expiry with unknown last, then Position reference.

Ordering precedes pagination and ignores value, freshness, and collection
arrival. Global and Account-detail views share within-Account order. History and
Activity timelines retain time order; their Account details use the shared
hierarchy. There is no user-selectable column sorting.

This gives a predictable ownership path. Renaming an Identity deliberately moves
it; renaming an Account does not. Trading Accounts precede funds and product
Accounts; Kraken Derivatives Holding is a holding wallet before Earn. Stable
ties and explicit missing-metadata rules avoid inventing facts.

## Numeric presentation

Cards, tables, details, explanatory text, counts, and chart tooltips share this
policy. Select precision by absolute value and retain sign and unit. Limits are
maxima rather than padding requirements; `d` is the number of integer digits.

| Type                    | Absolute value at least 1         | Nonzero absolute value below 1 |
| ----------------------- | --------------------------------- | ------------------------------ |
| USD value               | 2 fractional digits               | 4 significant digits           |
| Asset quantity or price | `max(2, 6 - d)` fractional digits | 6 significant digits           |
| Unscaled ratio          | `max(2, 4 - d)` fractional digits | 4 significant digits           |
| Percentage or leverage  | 2 fractional digits               | 4 significant digits           |
| Count                   | Whole integer                     | Not applicable                 |

Round to nearest with midpoint ties away from zero. Remove fractional trailing
zeros, normalize negative zero, and group integer digits with English thousands
commas. Missing, invalid, and non-finite values display as `—`, never zero.
Known zero displays as `0` with its relevant unit.

After rounding, at least four consecutive leading fractional zeros use
zero-count shorthand: `0.00000444` becomes `0.0₅444`. The subscript counts all
zeros after the decimal point before the first nonzero digit and may have
multiple digits. There is no eight-place ceiling. A rounding carry can remove
shorthand: USD `0.000099999` becomes `0.0001`.

Fractional rates displayed as percentages scale by 100 before precision is
selected. IMR, MMR, and the reported Account margin ratio remain unscaled;
leverage retains `x`. The reported ratio is labeled Margin level for Binance
Classic Cross Margin, UniMMR for Binance Portfolio Margin, and Margin ratio for
OKX. Labels follow immutable Account scope, including historical and stale
reports; these source ratios do not share one denominator. Source Asset amounts
use quantity precision even for USD-equivalent Assets; derived USD valuations
use USD precision. Price precision is independent of the quoting Asset.

USD valuations, flows, and differences in summaries, Tree nodes, signed badges,
and tooltips use `$`, `+$`, or `-$`. Tables put currency in headers such as
`Value (USD)` and use unadorned cells; never combine currency prefix and suffix.

Chart tooltips use source decimal strings and USD precision. Numeric axes are
the exception: values at least 1 use English compact notation whose fractional
precision adapts to the tick interval to prevent duplicate labels (one or two
fractional digits for narrow spans; uncompacted integers for increments below
10). Smaller nonzero ticks use four significant digits; rounded absolute values
below `0.0001` use scientific notation. A carry to 1 uses compact notation; zero
stays zero.

Screen readers receive rounded ordinary decimals, not zero-count shorthand.
There is no full-precision disclosure or copy-value interaction. Position
disclosures retain units and terms using the same precision as the main display;
margin provenance and conversion diagnostics remain available in the API.

Inputs, submitted values, diagnostic JSON, API decimals, storage, calculations,
aggregation, and ordering retain source precision. Dates, identifiers, protocol
values, and versions are not numeric amounts. Format totals after aggregation;
adding rounded rows can differ from the displayed total.

Adaptive precision keeps small amounts visibly nonzero while favoring concise
display. It sacrifices fixed decimal alignment and reconstruction of exact
source values from the display. These are project preferences, not an industry
standard;
[Bybit Spot Cost](https://www.bybit.com/en/help-center/article/How-to-Calculate-View-or-Adjust-Spot-Cost-in-Unified-Trading-Account)
and
[Capital.com](https://help.capital.com/hc/en-us/articles/33093412785170-Why-do-some-crypto-prices-look-different)
are precedents for individual conventions, not this combined policy.

## Overview and navigation

Overview shows metric cards, a recent-point chart, then the Platform → Identity
→ Account Tree. Each node shows its name and adjacent USD valuation; unknown
value is `—`. The nested disclosure list has no table headers or node counts.
Its whole node row is a native expansion button with an accessible name and
expanded state. Indentation and quiet guides express ownership; wide Account
tables scroll locally inside their parent.

Platforms start expanded, Identities and Accounts collapsed. Keyboard-operable
controls exist at each level. Data updates preserve expansion; re-entering
Overview or changing Portfolio resets it. Account details show known measures
and holdings directly. Unavailable measures are omitted without inventing
values; method, Report ID, and source time remain in an initially collapsed
Report details disclosure.

Account details contain failed, stale, and incomplete states with source times
and omissions. Page-level Issues remain visible when nodes are collapsed; normal
Connection labels and repeated ancestor badges do not occupy the Tree. Balances
always appear in details. Futures, Earn, and Dual sections hide only after a
successful empty read; loading, failed reads, and stale holdings stay explicit.
Account hiding follows the
[confirmed-empty rule](./domain-rules.md#manual-balances). The Identity add
action stays available when all Accounts hide.

Summary and Tree must describe one consistent Valuation Point; expanded details
must match the visible Account Report, apart from explicitly pending Manual
additions described below. Failed or conflicting reads offer recovery, retain
compatible prior data, and expose its age through point/report times.
Latest-only endpoints can race with collection and require a retry or reload;
the display must not combine inconsistent reports.

Holdings has independent Balances, Futures, Earn, and Dual Investment views,
with Balances as default and no Options placeholder. Ownership totals belong in
Overview, while flat views support browsing distinct holding kinds. Each view
remembers Platform, Identity, search, and pagination during the application
session, scoped by User and Portfolio. Internal navigation saves outgoing search
drafts and restores the target view. Explicit URLs, browser history, and reload
take precedence; reload preserves only the current URL's view state.

### Filters

Global holdings filters live in their column headers. Labels remain visible with
their condition; opening one offers a single-choice Platform or Identity list,
or a symbol search. Account is descriptive, not filterable, because Account
Types differ across Platforms and structures. Dual uses Pair for its currency
search. Headers remain interactive during loading, failure, and empty results,
with keyboard access and unclipped popovers.

Platform and Identity candidates are the Portfolio's non-archived Identities in
shared order. Each list is constrained by the other selection; symbol search
does not affect candidates. All is independent: choosing an Identity does not
choose its Platform. Clearing one filter preserves others; Clear filters resets
all three. Filter changes restart pagination.

Symbol search is a case-insensitive literal substring with surrounding
whitespace ignored, applied before pagination without ranking or spelling
correction. Futures searches native contract Symbol; Balances searches Asset and
Canonical Asset Symbols; Dual searches invested, Base, and Quote Asset Symbols.
Earn searches invested Asset and Canonical Asset Symbols, retaining a matching
multi-Asset Position as one whole row. Balance flat lists have no grouping
option.

Successful candidate reads reconcile invalid URL selections to All. If a valid
Identity conflicts with a Platform, keep Identity and reset Platform to All.
Loading or failed candidate reads cannot prove a selection invalid. Configured
candidates may yield no holdings, but stay stable during searches and collection
failures; URL correction can deliberately widen results.

The Futures notional total and completeness cover the Portfolio Boundary before
filtering or pagination. Balance totals follow filtered collected Balances;
pending Manual additions have no valuation and do not enter those totals.
Filtering never changes the Portfolio Boundary.

### Manual editing

Overview Identity and Manual headings and global Balances share an editor,
available before the first Valuation Point. Each dialog handles one addition,
edit, or deletion and reads the current quantity. Version conflicts require
explicit reload before another save; adding retains the draft on reload, while
editing reloads the latest quantity. Users remove amounts when they become
API-visible; collection time does not imply User reconfirmation.

Balance tables include current Manual additions before collection, with prices,
USD values, and observation times unknown. Changes to collected records retain
their collected quantities and values until a cycle applies the change. Equity
uses a dotted disclosure for short pending-addition, pending-quantity, or
pending-removal hints, available on hover, focus, and touch. Pending removals
hide edit/delete actions; all other current records remain editable. Reverting
to the collected quantity clears the hint, and deleting an uncollected addition
removes its row immediately. Excluded Manual records remain visible and
editable, with an Excluded hint and no valuation. There is no separate records
or pending-changes table. Current quantities keep their Manual Account visible;
Accounts without a report display no invented valuation.

## Holding tables

Global and Account tables share columns, units, numeric policy, and provenance.
Account and historical details omit ownership columns already established by
their context. Tables scroll horizontally where needed. Pointer hover, keyboard
focus, and click/touch disclosures expose the same supporting information.

### Balances

Columns are Platform, Identity, Account, Asset, Equity, Price (USD), Equity
(USD), Liability, Unrealized PnL, Issues. Account details start at Asset. Asset
shows the native Symbol with a secondary mapping hint. Withdrawable and Tradable
remain source/API fields rather than displayed columns.

Issues stays quiet (`—`) when healthy, exposing Stale or data issue badges on
hover and disclosing carried-forward source observation times when applicable.
Missing-price hints and page-level Issues remain visible; Balances do not add
Position-style row issue counts.

### Futures

Columns are Platform, Identity, Account, Symbol, Side, Size, Entry Price, Mark
Price, Liq. Price, Unrealized PnL, Margin, Notional (USD), Issues.

Symbol disclosures show Base, Quote, Settle, effective Contract Size and its
currency, linear/inverse payoff, perpetual/delivery term, and delivery time.
Contract Size already includes any multiplier; a USD face amount is not a
dimensionless multiplier.

One-way Positions display Both with signed Size; hedged Positions display Long
or Short with positive Size. Unknown historical mode keeps known direction and
positive Size with a missing-mode issue. Source direction and quantity remain
unchanged. [Contract rules](./domain-rules.md#contract-quantities-and-margin)
own Size, Margin, conversion, and leverage calculations.

Margin displays amount and Settle currency on one line. Known amounts and `—`
share the same right edge; the content-sized column has no empty tag slot. The
amount is one keyboard target disclosing a short line such as `1.88x isolated`,
`3x cross`, or `portfolio margin`. Missing leverage leaves the known mode.
Symbol details do not repeat leverage; margin provenance, conversion facts, and
unavailable reasons remain in the API. Mode and leverage belong together while
the visible row prioritizes Unrealized PnL and Margin amount.

Issues remains quiet (`—`) when healthy, exposing Stale or data issue badges on
hover. Badges identify stale data and missing Position Mode, Size, or USD
notional. Failures without rows remain in page-level Issues. Known notionals are
summed before filtering and pagination; missing required notionals or missing
position-capable Accounts make that total incomplete. Balance valuation
omissions alone do not make known contract notional incomplete.

### Earn and Dual Investment

Earn columns are Platform, Identity, Asset, Type, Maturity, Rate, Amount,
Rewards, Subscribed, Status, Issues. Expanded Earn Accounts share this table
without repeated Platform and Identity columns; Balances remain independently
visible. Both Earn and Dual put row-local stale or data issues in the final
Issues column. Status contains product phases only.

Earn Type uses Flexible, Fixed, Bonded, Timed, On-chain, with names and lock or
redemption rules in details. Count Positions, not subscriptions: rows can be
individual Positions or product/strategy aggregates. Multi-Asset products stay
in one row with separate amounts and qualified rewards. Failed collection
carries forward compatible successful descriptions with their original Observed
time; it never fabricates product rows from Balance sums.

Amount, Rewards, Rate, and Maturity follow the
[Earn semantic rules](./domain-rules.md#dual-investment-and-earn-completeness).
Show their reported basis and qualifiers; minimum lending preferences, rewards
with unverified scope, and delivery/redemption details belong in disclosures.
Source-unavailable fields remain quiet; metadata faults and stale reports stay
discoverable. Platform mappings own source-field interpretation.

## Editing and conflicts

An open editor keeps its draft and the version of the data it presented
together. Background reads cannot replace the draft or advance its write
precondition. Deletion uses the version shown in the confirmation, without
fetching a newer version just before deleting. Successful saves accept the
returned data as the new baseline.

Portfolio settings, Capital Flow editing, and Manual editing require explicit
reload after a version conflict before another save. Reload replaces editable
values with current data; Manual additions retain their proposed new record as
defined [above](#manual-editing). Failed reloads retain the draft and blocked
save. Identity and Catalog conflicts close the editor and require reopening the
updated item. Capital Flow deletion conflicts close the confirmation and require
reviewing the updated list before a new confirmation.

## Browser refresh and scrolling

Ordinary iPhone Safari and Android Chrome tabs use native pull-to-refresh on all
mobile pages, including sign-in and settings. Desktop uses browser reload. A
reload reads existing data and never requests a Collection Cycle; Collect now is
the explicit collection action. There are no page-specific reload buttons or
periodic Overview/Activity list refreshes. Running Collection progress,
completion reads, and reads after User mutations remain; scheduled backend
collection is independent.

URL-backed filters and pagination survive reload. Unsaved inputs, Tree
expansion, and unshown view memory follow ordinary full reload behavior, without
draft storage or refresh interception. An open page can lag scheduled
collection; its displayed point time remains the reference. When separate reads
refer to different cycles, hide mismatched details and refresh related reads
before showing them together. Committed points themselves do not change.
Portfolio deletion progress polls its small status list until the marked
Portfolio disappears; its switcher entry is disabled while deleting.

Page title and navigation stay visible during scrolling. Navigation drawers and
dialogs isolate background scrolling and restore its position after the last
overlay closes. Tables and overlay lists retain their own scrolling. Native
gestures and presentation are browser-controlled; embedded browsers and
home-screen installations are outside acceptance scope. Browser automation can
verify scrolling and reload semantics, but does not establish actual Safari or
Android browser-chrome coverage.

## History windows and tables

Both History views default to the rolling last three days. They expose 1D, 3D,
1W, 2W, 1M, 3M, and 6M shortcuts and populated From / To controls in the
Portfolio timezone. Days and weeks are fixed elapsed durations; months and years
preserve local calendar time and clamp to the target month's last day. Reject
ambiguous or nonexistent manually entered wall times explicitly.

Shortcuts apply immediately; manual edits apply together through Apply. Anchor a
relative window once for all reads while browsing: pagination, detail expansion,
and retries cannot move it. Reload resolves relative windows again; custom
absolute endpoints stay fixed. URLs retain the range, manually selected period,
and independent table pagination.

The Portfolio chart uses Raw through three days, Hour through 45 days, and Day
for longer ranges. Hour and Day use the final actual point in each existing
timezone bucket; they never average or synthesize value. Raw points remain
individually accessible in a separate paginated table. Curves ascend in time;
History tables use newest first. Sampling is display-only, not retention or
stored preaggregation. Old raw summaries remain expandable after detail expiry;
show an explicit expiry message and a separate retained-snapshot action. Label
the selected snapshot with its actual timestamp and total, including cross-day
fallback. Never present another point's holdings under the original total.

Valuation Points shows filters, the full-window chart, Period closes, then raw
points. Period closes shares the window. Both calendar views initially use Day
for day/week shortcuts, Week for 1M, and Month for 3M or 6M. Users can override
the period; applying a custom window retains it. Calendar reads calculate whole
intersecting periods, not clipped intervals. Value Changes also offers an
independent Custom range comparison.

Period closes columns are Period, Close (USD), Change, Issues. Raw Valuation
Points columns are Evaluated at, Total (USD), Change, Issues. Value Changes
columns are Period, Value change (USD), Close (USD), Flows, Issues.
Point-to-point and period-to-period movements display directional amounts and
percentages. Flows consolidates external capital and boundary movements into
compact descriptors. Across History tables, Issues stays quiet (`—`) during
complete intervals, opening badges and diagnostics for missing periods,
incomplete data, or stale Accounts.

All three tables default to 20 rows with 10/20/50 choices, Previous / Next, and
row counts. Range changes reset pagination and open point details; period
changes reset only that period table. Expanding a point cannot reset a page. Raw
points have unbounded count and server pagination; calendar results use the
400-period ceiling and independent browser pagination, preserving the complete
Value Changes chart. Read mechanisms belong in
[architecture](./architecture.md#history-windows-and-tables).
