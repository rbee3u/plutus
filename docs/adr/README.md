# Architecture decision records

ADRs record accepted domain and technical choices, their reasons, and meaningful
trade-offs. The [domain model](../../CONTEXT.md) owns terms and business rules;
[architecture](../architecture.md) owns technical mechanisms. Setup, validation,
and deployment procedures belong in their respective guides.

## Records

This documentation reorganization numbers the current topic records from 0001.
Old numbered citations must use the corresponding records below. A number
identifies a record, not an implementation priority or proof of completion.

| ADR                                                | Decision                                                                            |
| -------------------------------------------------- | ----------------------------------------------------------------------------------- |
| [0001](./0001-ownership-and-accounts.md)           | Ownership, selected Accounts, and structure versus mode                             |
| [0002](./0002-account-reports.md)                  | Aligned reports, Complete Set, and compatible stale values                          |
| [0003](./0003-valuation-and-catalog.md)            | USD valuation, reported equity, Asset mapping, and exclusions                       |
| [0004](./0004-capital-and-periods.md)              | Absolute Value Change, manual flows, and observed period endpoints                  |
| [0005](./0005-dual-investment.md)                  | Separate Dual Investment, shared completeness, and scope compatibility              |
| [0006](./0006-contract-first-modular-monolith.md)  | Contract-first modular monolith and typed boundaries                                |
| [0007](./0007-authentication.md)                   | CLI-provisioned Users and write-only credentials                                    |
| [0008](./0008-environments-and-migrations.md)      | Environment isolation and populated-database migrations                             |
| [0009](./0009-adapter-verification.md)             | Independent synthetic tests, recorded evidence, and validation claims               |
| [0010](./0010-process-logging.md)                  | Unified process logging, SQL visibility, and rotation ownership                     |
| [0011](./0011-display-ordering.md)                 | Shared business ordering and stable holdings pagination                             |
| [0012](./0012-position-presentation.md)            | Contract columns, normalized Size, margin provenance, and Observed                  |
| [0013](./0013-numeric-presentation.md)             | Shared numeric precision, zero-count shorthand, and chart exceptions                |
| [0014](./0014-holdings-header-filters.md)          | Linked header filters, substring search, and retired flat-list grouping             |
| [0015](./0015-mobile-native-refresh.md)            | Native mobile browser refresh and isolated overlay scrolling                        |
| [0016](./0016-overview-and-holdings-navigation.md) | Overview ownership Tree, flat Holdings navigation, and manual display refresh       |
| [0017](./0017-history-windows-and-pagination.md)   | Explicit history windows and independent pagination                                 |
| [0018](./0018-earn-position-presentation.md)       | Earn product granularity, qualified metadata, legacy summaries, and shared Observed |

| [0019](./0019-manual-balances-and-empty-accounts.md) | Fixed signed Manual
ledger, frozen cycle inputs, adjustment intervals, and confirmed empty display |

## Maintenance

Keep these identifiers stable after this reorganization. Update the relevant
record when a decision changes; add the next numbered record for a durable
choice that does not fit an existing topic. Keep accepted decisions distinct
from preferences and unresolved implementation gaps. Preserve useful reasons and
trade-offs without duplicating field mappings, commands, or runtime details.
