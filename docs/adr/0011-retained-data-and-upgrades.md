---
status: accepted
---

# Retained source facts and populated-database upgrades

Keep Observations and financial history without downsampling or a retention
limit. Source facts have no editing API; derived results can be rebuilt while
preserving observation and evaluation times. Missing historical metadata stays
unknown unless retained facts establish it; current configuration cannot invent
historical provenance.

Migrations are explicit. After prod's first migration, freeze the baseline and
append upgrades compatible with populated databases. Never reset prod or rewrite
source financial amounts to make a new presentation work. Preserve compatible
scope fallback and incomplete comparisons across scope changes.

Retaining sources supports reviewable recalculation and safe interpretation as
mappings evolve, at the cost of growing storage and explicit upgrade work.
Appended migrations protect retained data; backup, upgrades, and master-key
management remain operator duties. Partial historical descriptions are
preferable to fabricated facts.

[Architecture](../architecture.md#persistence-and-rebuilds) owns rebuild
mechanisms; [deployment](../deployment.md#upgrades-and-resets) owns upgrade and
reset procedures. [Domain rules](../domain-rules.md#report-scope-compatibility)
own compatibility, independently of how a database reaches the current schema.
