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
detail routes, and Overview's Platform and Identity totals remain. These are
accepted API changes: the existing symbol parameter gains substring semantics,
and retired parameters follow the server's existing unknown-query policy. Old
browser Account/grouping keys are removed, with pagination reset.

Successful candidate reads reconcile invalid URL selections to All. When a valid
Identity conflicts with a Platform, the Identity stays selected and the Platform
becomes All. Loading or failed candidate reads cannot establish that a selection
is invalid. Futures and Dual Position tabs share filter state; Balances and
Positions retain their independent page state.

The Futures notional total and its completeness still cover the whole Portfolio
Boundary before filters. Balance totals still follow the filtered Balances.
Filtering does not change the Portfolio Boundary or financial source facts.

## Reason and trade-offs

The former Account list combined every Identity with heterogeneous Account
Types, while the separate filter form repeated labels already in the table.
Header filters keep the condition near its column and reduce vertical space.
Removing Balance grouping avoids a second presentation of the same holdings;
Overview retains useful ownership totals.

Configured candidates can lead to an empty result for an Identity without
holdings, but remain stable during searches and collection failures. Automatic
URL correction can widen results, an accepted preference for recovering from
obsolete links. The API simplification deliberately does not preserve exact
symbol search or the retired flat-list features.

Interactive headers must survive loading, errors, and empty results so requests
cannot remove an active input. Keyboard access and unclipped popovers are part
of the accepted interaction. Runtime mechanisms and verification belong in
architecture and testing documentation, not financial domain rules.
