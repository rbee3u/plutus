---
status: accepted
---

# Immutable summaries, daily detail, and populated-database upgrades

Keep committed financial summaries and comparison evidence in a rolling ten-year
window, with recent full detail and daily snapshots. The
[domain retention policy](../domain-rules.md#historical-detail-retention) owns
exact selection and dependency rules. Committed valuations are immutable; there
is no rebuild API.

The target deployment has four cores, 8 GB RAM, and about 30 GB storage for ten
years. Retaining all intraday holdings for years makes detail tables dominate
that budget. Daily archives preserve useful long-term inspection while
sacrificing exact intraday reconstruction. Comparison facts are small enough to
retain throughout the window; deriving them from expiring detail would silently
change historical results. Current configuration and opening-boundary evidence
can precede the window; public history must not expose internal predecessors as
complete old history. Unusual historical corrections are deliberate operator
reconciliation, without a dedicated repair framework for rare cases.

Migrations are explicit. After prod's first migration, freeze the baseline and
append upgrades compatible with populated databases. Never reset prod or rewrite
source financial amounts to make a new presentation work. The prelaunch baseline
may be consolidated while development and test data remain disposable.

[Architecture](../architecture.md#persistence-and-retention) owns cleanup and
read consistency. [Deployment](../deployment.md#upgrades-and-resets) owns
upgrade, backup and reset procedures. Pruning frees reusable database space; it
is not a disk quota or a substitute for capacity monitoring and backups.
