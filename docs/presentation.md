# Presentation

This guide owns current display and interaction rules.
[Product scope](./product-scope.md) defines offered views,
[domain rules](./domain-rules.md) define financial meaning, and
[architecture](./architecture.md) describes implementation. Display choices do
not change source facts, selection, or the portfolio boundary.

## Names and ordering

### Capitalization

Follow the shared [capitalization guide](./capitalization.md) for authored
labels, prose, accessible names, and preserved source content.

### Account aliases

Identity lists and creation forms use these compact labels without changing
canonical terms, API names, identifiers, or stored data:

| Platform | Account structure aliases                                     | Margin mode aliases                                                                                          |
| -------- | ------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| Binance  | `Classic trading` → `Classic`; `Portfolio margin` → `Unified` | None                                                                                                         |
| OKX      | `Unified account` → `Unified`                                 | `Spot mode` → `Spot`; `Futures mode` → `Futures`; `Multi-currency margin` → `MCM`; `Portfolio margin` → `PM` |
| Bybit    | `Unified Trading Account 2.0` → `Unified`                     | `Isolated margin` → `ISO`; `Cross margin` → `MCM`; `Portfolio margin` → `PM`                                 |
| Kraken   | `Separate wallets` → `Classic`; `Unified wallet` → `Unified`  | None                                                                                                         |

The list joins structure and mode with `·`, using `Unknown` while an OKX or
Bybit mode is unavailable. Known aliases render before the platform catalog
loads. Unknown identifiers use the catalog name when available, otherwise the
identifier. There is no full-name disclosure. Position margin setting labels and
position portfolio margin descriptions use their own wording.

Compact names improve scanning at the cost of native names and version details.
Shared aliases imply neither identical account arrangements nor identical risk
models: Binance `Unified` is its supported portfolio margin structure; Bybit
`MCM` is its canonical cross margin mode.

### Catalog labels

Catalog uses `Assets` and `Futures` tabs, with `Asset` and `Symbol` as their
main column labels. Futures contains perpetual and delivery futures contracts.
Holdings retains `Balances` and `Futures` with `Asset` and `Symbol` columns.
Catalog asset filters provide `Platform`, `Search asset`, and `Status` (`All`,
`Active`, and `Excluded`) options.

Catalog, balances, and earn display asset symbol first. When its canonical asset
symbol differs, Catalog shows `asset → canonical asset` on one line to keep the
list compact and avoid adding height to mapped rows. Balances and earn show a
secondary `Canonical asset: ...` hint. Omit redundant mapping hints. Earn amount
units use asset symbol. Catalog and mapping dialogs omit full names. Mapping
dialogs label the source `Asset` and the target `Canonical asset`. Capital flow
inputs use `Asset`; the stored symbol still identifies the cross-platform
canonical asset. Capital flow tables present quantity and asset together under
`Amount`.

Catalog futures contract size uses native base units for linear contracts and
quote face amounts for inverse contracts. Binance linear quantities already use
base units, so an absent recorded size displays `1` with the known base symbol.
Digit-leading base symbols use `×` to separate amount from symbol, for example
`1 × 1000PEPE`; do not strip prefixes or apply another multiplier. Other missing
sizes stay `—`. This display convention does not rewrite observed terms.

### Ordering

Lists, groups, and selectors use platform, identity, then account order; views
with fewer dimensions use the corresponding part of that hierarchy.

- Platforms: Binance, OKX, Bybit, Kraken.
- Identities: case-insensitive display name, then immutable ID.
- Accounts: account type order within the declared account structure, then
  immutable ID. The [platform catalog](../internal/platform/adapters/catalog.go)
  owns type order; manual is last. Account names and margin mode do not affect
  order.
- Balances within an account: asset symbol, canonical asset symbol, asset ID.
- Futures within an account: base asset symbol; linear before inverse; perpetual
  before delivery within each family; delivery expiry nearest first, unknown
  last; native symbol; long before short; platform position reference. Missing
  base follows known bases and uses native symbol, side, and reference. Do not
  infer base from symbol or merge SHIB with 1000SHIB.
- Earn within an account: sorted invested asset symbol sequence, with canonical
  asset symbol breaking asset ties; flexible, fixed, bonded, timed, on-chain;
  maturity with unknown last; stable source reference. Multi-asset products stay
  together.
- Dual within an account: expiry with unknown last, then position reference.

Ordering precedes pagination and ignores value, freshness, and collection
arrival. Global and account-detail views share within-account order. History and
Activity timelines retain time order; their account details use the shared
hierarchy. There is no user-selectable column sorting.

This gives a predictable ownership path. Renaming an identity deliberately moves
it; renaming an account does not. Trading accounts precede funds and product
accounts; the Kraken `Derivatives holding` account is a holding wallet before
earn. Stable ties and explicit missing-metadata rules avoid inventing facts.

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
selected. IMR, MMR, and the reported account margin ratio remain unscaled;
leverage retains `x`. The reported ratio is labeled `Margin level` for Binance
classic cross margin, `UniMMR` for Binance portfolio margin, and `Margin ratio`
for OKX. Labels follow immutable account scope, including historical and stale
reports; these source ratios do not share one denominator. Source asset amounts
use quantity precision even for USD-equivalent assets; derived USD valuations
use USD precision. Price precision is independent of the quoting asset.

USD valuations, flows, and differences in summaries, tree nodes, signed badges,
and tooltips use `$`, `+$`, or `-$`. Tables put currency in headers such as
`Value (USD)` and use unadorned cells; never combine currency prefix and suffix.

Chart tooltips use source decimal strings and USD precision. Numeric axes are
the exception: values at least 1 use English compact notation whose fractional
precision adapts to the tick interval to prevent duplicate labels (one or two
fractional digits for narrow spans; uncompacted integers for increments below
10). Smaller nonzero ticks use four significant digits; rounded absolute values
below `0.0001` use scientific notation. A carry to 1 uses compact notation; zero
stays zero. Y-axis labels retain matching fractional precision for their tick
interval, including trailing zeros, and align right in tabular or monospace
type. For example, `191.0K` stays aligned with `191.4K`; the general trimming
rule does not apply to these axis labels.

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

Overview guides portfolios without identities to add one, even when an empty
valuation point already exists. Before the first valuation point, Overview and
all Holdings views guide portfolios with only archived identities to add one,
and portfolios with a non-archived identity to collect. These setup prompts use
a successful identity read, not an empty holdings tree or failed request.
Archiving identities does not replace retained valuations with setup prompts.

Overview shows metric cards, a recent-point chart, then the platform → identity
→ account tree. Each node shows its name and adjacent USD valuation; unknown
value is `—`. The nested disclosure list has no table headers or node counts.
Its whole node row is a native expansion button with an accessible name and
expanded state. Indentation and quiet guides express ownership; wide account
tables scroll locally inside their parent.

Platforms start expanded, identities and accounts collapsed. Keyboard-operable
controls exist at each level. Data updates preserve expansion; re-entering
Overview or changing portfolio resets it. Account details show known measures
and holdings directly. Unavailable measures are omitted without inventing
values; method, report ID, and source time remain in an initially collapsed
report details disclosure.

Account details contain failed, stale, and incomplete states with source times
and omissions. Cross-account Issues identify platform, identity, account, and
asset or contract where available, using metadata from the displayed view.
Overview's page-level Issues remain visible when nodes are collapsed; normal
connection labels and repeated ancestor badges do not occupy the tree. Balances
always appear in details. Futures, earn, and dual sections hide only after a
successful empty read; loading, failed reads, and stale holdings stay explicit.
Account hiding follows the
[confirmed-empty rule](./domain-rules.md#manual-balances). The identity add
action stays available when all accounts hide.

Summary and tree must describe one consistent valuation point; expanded details
must match the visible account report, apart from explicitly pending manual
additions described below. Failed or conflicting reads offer recovery, retain
compatible prior data, and expose its age through point/report times. Expanded
account reads request the displayed valuation point even after a newer
collection finishes. Unavailable or expired details offer recovery without
substituting another point's holdings.

Holdings has independent balances, futures, earn, and dual investment views,
with balances as default and no options placeholder. Ownership totals belong in
Overview, while flat views support browsing distinct holding kinds. Each view
remembers platform, identity, search, and pagination during the application
session, scoped by user and portfolio. Internal navigation saves outgoing search
drafts and restores the target view. Explicit URLs, browser history, and reload
take precedence; reload preserves only the current URL's view state.

### Filters

Global holdings filters live in their column headers. Labels remain visible with
their condition; opening one offers a single-choice platform or identity list,
or a symbol search. Account is descriptive, not filterable, because account
types differ across platforms and structures. Dual uses Pair for its currency
search. Headers remain interactive during loading, failure, and empty results,
with keyboard access and unclipped popovers.

Platform and identity candidates are the portfolio's non-archived identities in
shared order. Platform is the top-level selector; selecting a platform unlocks
Identity to filter non-archived identities within that platform. When Platform
is All, Identity renders as a plain text header without dropdown controls.
Clearing or switching the platform resets Identity to All. Clear filters resets
all three. Filter changes restart pagination.

Symbol search is a case-insensitive literal substring with surrounding
whitespace ignored, applied before pagination without ranking or spelling
correction. Futures searches native contract symbol; balances searches asset and
canonical asset symbols; dual searches invested, base, and quote asset symbols.
Earn searches invested asset and canonical asset symbols, retaining a matching
multi-asset position as one whole row. Balance flat lists have no grouping
option.

Successful candidate reads reconcile invalid URL selections to All. If an
identity is specified without a platform or conflicts with the selected
platform, clear identity to All while keeping the valid platform. Loading or
failed candidate reads cannot prove a selection invalid. Configured candidates
may yield no holdings, but stay stable during searches and collection failures;
URL correction can deliberately widen results.

The futures notional total and completeness cover the portfolio boundary before
filtering or pagination. Balance totals follow filtered collected balances;
pending manual additions have no valuation and do not enter those totals. The
balance total's badge qualifies that filtered sum, independently of the
portfolio valuation. Its omissions name unpriced balances and missing reports.
Empty holdings use report coverage for their filtered account scope; stale
reports with readable holdings do not turn into unknown sets. Filtering never
changes the portfolio boundary. A holdings view keeps its collected valuation
point across filter changes, pagination, and retries; re-entry, browser reload,
or collection completion resolves it again. Manual saves reset balance
pagination while keeping the collected point.

### Manual editing

Overview identity and manual headings and global balances share an editor,
available before the first valuation point. Each dialog handles one addition,
edit, or deletion and reads the current quantity. Version conflicts require
explicit reload before another save; adding retains the draft on reload, while
editing reloads the latest quantity. Users remove amounts when they become
API-visible; collection time does not imply user reconfirmation.

Balance tables include current manual additions before collection, with prices,
USD values, and observation times unknown. Changes to collected records retain
their collected quantities and values until a cycle applies the change. Equity
uses a dotted disclosure for short pending-addition, pending-quantity, or
pending-removal hints, available on hover, focus, and touch. Pending removals
hide edit/delete actions; all other current records remain editable. Reverting
to the collected quantity clears the hint, and deleting an uncollected addition
removes its row immediately. Excluded manual records remain visible and
editable, with an Excluded hint and no valuation. There is no separate records
or pending-changes table. Current quantities keep their manual account visible;
accounts without a report display no invented valuation.

## Holding tables

Issue lists follow the [capitalization guide](./capitalization.md) and use
readable warning badges and explanations. Machine codes remain available in API
responses and diagnostic details. In account details, each holding section
places its own Issues after its table, matching the independent Holdings views.
Shared account-level issues can appear in each affected section; they are not
merged into one combined list. A holding section with issues remains visible
even when it has no rows. Without a compatible successful report, account
details describe holdings as unknown rather than empty and do not claim that
earlier holdings were retained.

Global and account tables share columns, units, numeric policy, and provenance.
Account and historical details omit ownership columns already established by
their context. In global tables, Account uses compact structure aliases with
their full names preserved in disclosures. Tables scroll horizontally where
needed. Pointer hover, keyboard focus, and click/touch disclosures expose the
same supporting information.

### Balances

Columns are `Platform`, `Identity`, `Account`, `Asset`, `Equity`, `Price (USD)`,
`Equity (USD)`, `Liability`, `Unrealized PnL`, and `Issues`. Account details
start at `Asset`, which shows the native symbol with a secondary mapping hint.
Withdrawable and tradable remain source/API fields rather than displayed
columns.

Issues stays quiet (`—`) when healthy, exposing stale or data issue badges on
hover and disclosing carried-forward source observation times when applicable.
Missing-price hints and page-level Issues remain visible; balances do not add
position-style row issue counts.

### Futures

Columns are `Platform`, `Identity`, `Account`, `Symbol`, `Side`, `Size`,
`Entry price`, `Mark price`, `Unrealized PnL`, `Margin`, and `Issues`.

Entry price and mark price disclose their corresponding USD notional in a
`Notional: $...` popover on hover, focus, or click when their price is
available. Missing prices display a static `—` without popovers; unavailable
notionals display `Notional: —`.

Symbol disclosures reuse the Catalog futures values in column order: base,
quote, settle, delivery expiry as a portfolio-local time without a timezone
label, then effective contract size and its currency. Linear/inverse payoff and
perpetual/delivery term stay in the header badges. Contract size already
includes any multiplier; a USD face amount is not a dimensionless multiplier.

One-way positions display `Both` with signed size; hedged positions display
`Long` or `Short` with positive size. Unknown historical mode keeps known
direction and positive size with a missing-mode issue. Source direction and
quantity remain unchanged.
[Contract rules](./domain-rules.md#contract-quantities-and-margin) own size,
margin, conversion, and leverage calculations.

Margin displays amount and settle currency on one line. Known amounts and `—`
share the same right edge; the content-sized column has no empty tag slot. The
amount is one keyboard target disclosing a short line such as `1.88x isolated`,
`3x cross`, or `portfolio margin`. Missing leverage leaves the known mode.
Symbol details do not repeat leverage; margin provenance, conversion facts, and
unavailable reasons remain in the API. Mode and leverage belong together while
the visible row prioritizes `Unrealized PnL` and the margin amount.

Issues remains quiet (`—`) when healthy, exposing stale or data issue badges on
hover. Badges identify stale data and missing position mode, size, or USD
notional. Failures without rows remain in page-level Issues. Known notionals are
summed before filtering and pagination; missing required notionals or missing
position-capable accounts make that total incomplete. Balance valuation
omissions alone do not make known contract notional incomplete.

### Earn and dual investment

Earn columns are `Platform`, `Identity`, `Asset`, `Type`, `Maturity`, `Rate`,
`Amount`, `Rewards`, `Subscribed`, `Status`, and `Issues`. Expanded earn
accounts share this table without repeated platform and identity columns;
balances remain independently visible. Both earn and dual put row-local stale or
data issues in the final Issues column. Status contains product phases only.

Earn type uses `Flexible`, `Fixed`, `Bonded`, `Timed`, and `On-chain`, with
names and lock or redemption rules in details. Count positions, not
subscriptions: rows can be individual positions or product/strategy aggregates.
Multi-asset products stay in one row with separate amounts and qualified
rewards. Failed collection carries forward compatible successful descriptions
with their original observation time; it never fabricates product rows from
balance sums.

Amount, rewards, rate, and maturity follow the
[earn semantic rules](./domain-rules.md#dual-investment-and-earn-completeness).
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

Portfolio settings, capital flow editing, and manual editing require explicit
reload after a version conflict before another save. Reload replaces editable
values with current data; manual additions retain their proposed new record as
defined [above](#manual-editing). Failed reloads retain the draft and blocked
save. Identity and Catalog conflicts close the editor and require reopening the
updated item. Capital flow deletion conflicts close the confirmation and require
reviewing the updated list before a new confirmation.

Capital flow time inputs show and interpret wall times in the portfolio's
timezone, with the timezone stated beside the input. Reject nonexistent or
ambiguous replacement times, as for
[history windows](#history-windows-and-tables). Unchanged times preserve the
stored instant and its precision, including during a clock transition; editing
another field does not reinterpret that instant.

## Browser refresh and scrolling

Ordinary iPhone Safari and Android Chrome tabs use native pull-to-refresh on all
mobile pages, including sign-in and settings. Desktop uses browser reload. A
reload reads existing data and never requests a collection cycle; Collect now is
the explicit collection action. There are no page-specific reload buttons or
periodic Overview/Activity list refreshes. Running collection progress,
completion reads, and reads after user mutations remain; scheduled backend
collection is independent.

URL-backed filters and pagination survive reload. Unsaved inputs, tree
expansion, and unshown view memory follow ordinary full reload behavior, without
draft storage or refresh interception. An open page can lag scheduled
collection; its displayed point time remains the reference. When separate reads
refer to different cycles, hide mismatched details and refresh related reads
before showing them together. Committed points themselves do not change.
Portfolio deletion progress polls its small status list until the marked
portfolio disappears; its switcher entry is disabled while deleting.

Page title and navigation stay visible during scrolling. Navigation drawers and
dialogs isolate background scrolling and restore its position after the last
overlay closes. Tables and overlay lists retain their own scrolling. Native
gestures and presentation are browser-controlled; embedded browsers and
home-screen installations are outside acceptance scope. Browser automation can
verify scrolling and reload semantics, but does not establish actual Safari or
Android browser-chrome coverage.

## History windows and tables

History views and capital views default to the rolling last week (seven days).
They expose 1D, 3D, 1W, 2W, 1M, 3M, and 6M shortcuts and populated From / To
controls in the portfolio timezone. Days and weeks are fixed elapsed durations;
months and years preserve local calendar time and clamp to the target month's
last day. Reject ambiguous or nonexistent manually entered wall times
explicitly.

Shortcuts apply immediately; manual edits apply together through Apply. Anchor a
relative window once for all reads while browsing: pagination, detail expansion,
and retries cannot move it. Reload resolves relative windows again; custom
absolute endpoints stay fixed. URLs retain the range, manually selected period,
and independent table pagination.

The portfolio chart uses Raw through three days, Hour through 45 days, and Day
for longer ranges. Hour and Day use the final actual point in each existing
timezone bucket; they never average or synthesize value. Raw points remain
individually accessible in a separate paginated table. Curves ascend in time;
History tables use newest first. Sampling is display-only, not retention or
stored preaggregation. Old raw summaries remain expandable after detail expiry;
show an explicit expiry message and a separate retained-snapshot action. Label
the selected snapshot with its actual timestamp and total, including cross-day
fallback. Never present another point's holdings under the original total.

Boundary events use newest collection cycles first, preserving the account order
within each cycle. Their time order is fixed, without a sorting control. Capital
flow and boundary event tables use server pages of 50 rows with Previous / Next
and the full filtered count. Range changes reset their offset; reload retains
URL pagination. Flow writes restart from the first page in the same window;
reads recover an offset that no longer contains rows. A series exceeding its
declared point budget shows an explicit recovery message; all original points
remain available in the raw table.

Capital flows columns are `Effective at`, `Platform`, `Identity`, `Account`,
`Direction`, `Amount`, `Value (USD)`, and `Note`. Unallocated flows display `—`
for platform, identity, and account. Amount combines quantity and canonical
asset symbol. Boundary events columns are `Evaluated at`, `Platform`,
`Identity`, `Account`, `Direction`, `Value (USD)`, and `Issues`. Issues remains
quiet (`—`) during complete intervals, showing an `Incomplete` badge when
incomplete.

The `Valuation points` view shows filters, the full-window chart,
`Period closes`, then raw points. Period closes shares the window. Both calendar
views initially use Day for day/week shortcuts, Week for 1M, and Month for 3M or
6M. Users can override the period; applying a custom window retains it. Calendar
reads calculate whole intersecting periods, not clipped intervals.

Value changes places a range summary below the shared time filters, before its
chart and table. It updates automatically for the applied window, independently
of the calendar period and pagination. Show adjusted USD value change and its
percentage, start and end values with their actual point times, and capital and
boundary flows. There is no separate comparison form or Compute action.

Missing endpoints or two endpoints resolving to the same point show unknown
change and flows (`—`), with the reason visible. Keep the selected window; never
silently shorten it to available history. Known results with incomplete inputs
remain visible with an Incomplete badge. Keep limitations and boundary event
details collapsed until requested. Explain that calendar rows cover whole
periods and their changes may not sum to the range summary. Endpoint selection
and calculation rules belong to
[domain rules](./domain-rules.md#calendar-periods).

Period closes columns are `Period`, `Close (USD)`, `Change`, and `Issues`. Raw
valuation points columns are `Evaluated at`, `Total (USD)`, `Change`, and
`Issues`. Value changes columns are `Period`, `Value change (USD)`,
`Close (USD)`, `Flows`, and `Issues`. Point-to-point and period-to-period
movements display directional amounts and percentages. Change retains the
known-total difference when either endpoint valuation point is incomplete, with
an `Incomplete` badge identifying the previous and/or current point. An absent
endpoint displays `—`. Flows consolidates external capital and boundary
movements into compact descriptors. Across History tables, Issues stays quiet
(`—`) during complete intervals, opening badges and diagnostics for missing
periods, incomplete data, or stale accounts.

All three tables default to 20 rows with 10/20/50 choices, Previous / Next, and
row counts. Range changes reset pagination and open point details; period
changes reset only that period table. Expanding a point cannot reset a page. Raw
points have unbounded count and server pagination; calendar results use the
400-period ceiling and independent browser pagination, preserving the complete
value changes chart. Read mechanisms belong in
[architecture](./architecture.md#history-windows-and-tables).
