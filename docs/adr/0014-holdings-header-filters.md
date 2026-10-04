---
status: accepted
---

# ADR 0014: Holdings header filters and a smaller flat-list API

## Decision

The global Balances, futures Positions, and Dual Positions tables place filters
in their column headers. Column names stay visible with the current condition;
click opens a single-choice Platform or Identity list, or a symbol search.
Balances displays Identity as its own column. Dual Positions uses its existing
Pair header for currency search. Account remains descriptive rather than
filterable, since Account Types differ by Platform and Account Structure.

Platform and Identity candidates come from the Portfolio's non-archived
Identities, in the ordering accepted by ADR 0011. Each candidate list is limited
by the other selection; symbol search does not change candidates. All remains an
independent choice: selecting an Identity never automatically selects its
Platform. Clearing one filter preserves the others; Clear filters resets all
three. Filter changes restart pagination.

Symbol search is a case-insensitive literal substring, with surrounding
whitespace ignored. Futures searches the native contract Symbol; Balances
searches native and mapped Asset Symbols; Dual Positions searches invested,
Base, and Quote Symbols. Search happens before pagination and preserves the
existing business order. There is no spelling correction or ranked search.

The three flat-list APIs retire their optional Account filter. Balances also
retires its grouping parameter and groups response. Account IDs in rows, Account
detail reads, and Overview's Platform and Identity totals remain in the
ownership Tree. These are accepted API changes: the existing symbol parameter
gains substring semantics, and retired parameters follow the server's existing
unknown-query policy. Old browser Account/grouping keys are removed, with
pagination reset.

Successful candidate reads reconcile invalid URL selections to All. When a valid
Identity conflicts with a Platform, the Identity stays selected and the Platform
becomes All. Loading or failed candidate reads cannot establish that a selection
is invalid. Balances, Futures, and Dual Investment each retain independent
Platform, Identity, symbol search, and pagination state within the application
session, scoped by User and Portfolio. Internal navigation restores the target
view's state and saves outgoing search drafts. Explicit URLs, browser history,
and reloads take precedence; reload preserves only the current URL's view state.
This replaces the former shared Futures/Dual filter state. See
[ADR 0016](./0016-overview-and-holdings-navigation.md).

The Futures notional total and its completeness still cover the whole Portfolio
Boundary before filters. Balance totals still follow the filtered Balances.
Filtering does not change the Portfolio Boundary or financial source facts.

Global Balances keeps Platform, Identity, and Account first, followed by Asset,
Equity, Price (USD), Equity (USD), Liability, Unrealized PnL, and Observed.
Account detail uses the same columns starting with Asset. Asset replaces the
Symbol label without changing its native-symbol display, secondary mapping hint,
or search semantics. Withdrawable and Tradable are hidden from both tables;
their source and API fields remain available.

Balance Observed follows the Futures presentation: the source Account Report
time in the Portfolio timezone, to the minute, with no fresh badge. Stale rows
retain the earlier report time and provide a Stale disclosure using the
Account's existing stale issue, or the standard carried-forward explanation.
Unknown observation times stay unknown. Missing-price hints and page-level
issues remain visible rather than gaining Position-style row issue counts.

## Reason and trade-offs

The former Account list combined every Identity with heterogeneous Account
Types, while the separate filter form repeated labels already in the table.
Header filters keep the condition near its column and reduce vertical space.
Removing Balance grouping avoids a second presentation of the same holdings;
Overview's Tree retains useful ownership totals.

The compact Balance columns prioritize equity and its USD valuation together.
Observed makes source age visible while matching the Futures convention, and
shared columns keep the global and Account views consistent.

Configured candidates can lead to an empty result for an Identity without
holdings, but remain stable during searches and collection failures. Automatic
URL correction can widen results, an accepted preference for recovering from
obsolete links. The API simplification deliberately does not preserve exact
symbol search or the retired flat-list features.

Interactive headers must survive loading, errors, and empty results so requests
cannot remove an active input. Keyboard access and unclipped popovers are part
of the accepted interaction. Runtime mechanisms and verification belong in
architecture and testing documentation, not financial domain rules.
