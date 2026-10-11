---
status: accepted
---

# Prelaunch schema and retained financial provenance

Each Collection Cycle row is both retained history and its own queue entry:
attempts, availability, and the lease live beside status and the last error.
Source Account Reports stay separate from their committed valuations. A Futures
Contract is one symbol with one immutable set of observed terms; no feature
reads a symbol across term changes, so a separate per-symbol parent would add a
join without a reader. Manual revisions and Asset Exclusions use versioned
children without duplicate current-state tables; their actors are recorded once,
in audit events. A Manual report references the revision it reported instead of
a per-cycle input table: revisions are immutable, and the reports commit with
the cycle's marker.

Preserve these responsibilities rather than merging entities with different
lifetimes. A separate job table would duplicate cycle state on its own expiry
clock, while Points stay separate because a cycle changes until first commit and
a Point never changes after it. Core financial references need scoped foreign
keys. Every Portfolio-owned primary key starts with `portfolio_id`, so key
prefixes cover those foreign keys and cluster rows by owner; every join must
name the Portfolio. Earn's variable descriptions stay in bounded typed JSON;
optional product lists do not require separate entities. Mappings stay inline on
Assets.

The cycle is the sole identity of an immutable Valuation Point. First commit
includes summaries, evidence, and cycle completion. Retries return it unchanged.
Algorithm versions remain provenance; result revisions and rebuilds add no
meaning to an immutable result. Manual adjustments and sparse calculated Asset
changes are stored because long-term comparisons must survive detail pruning.
Boundary Events freeze their completeness and amount independently of expiring
Account Valuations. Facts derivable from retained rows, such as calculation
version changes and boundary totals, are computed on read rather than copied;
Account issues follow the detail they describe. A Balance Valuation's pricing
method alone records policy exclusion; a copied exclusion version would restate
it.

Manual edit audits retain the single changed Asset and its old/new quantity;
revision numbers alone become dangling explanations after detail cleanup.
Exclusion audits instead resolve their retained version rows, avoiding a second
copy of policy facts. Sparse calculated Asset history remains appropriate for
mostly stable holdings; a per-point encoded Asset set would add storage and
encoding rules without a demonstrated capacity need at this workload.

All market catalogs and price observations belong to a Portfolio. Duplicating
markets across Portfolios is intentional: local mappings, collection, retention,
and deletion cannot change another Portfolio's evidence. One observation per
Platform and Canonical Asset per cycle attempt suffices: Identities on one
Platform read the same public market minutes apart at most, well inside the
price staleness window. Only access resources and User-scoped idempotency
records have shared lifetimes.

Use names that distinguish stored facts from interpretations: Position source
quantity and unit, Asset mapping version, and initial Portfolio. Extensible
Platform, Account Structure, Account Type and Margin Mode catalogs are validated
by runtime domain rules rather than repeated SQL enums. Stable signedness,
shape, quote currency, uniqueness, and ownership constraints remain in SQL.

Portfolio deletion is asynchronous bounded work driven only by the root's
deletion request. Writers lock the root, so a request already collecting data
cannot repopulate rows after deletion starts. The root remains until cleanup
completes. This avoids giant transactions without adding a recycle bin, a task
table, or generic task payloads.

[Architecture](../architecture.md#persistence-and-retention) owns mechanisms;
[domain rules](../domain-rules.md#assets-and-valuation) own eligibility;
[ADR 0011](./0011-retained-data-and-upgrades.md) owns the storage trade-off and
production migration freeze.
