---
status: accepted
---

# Manual quantities within exchange identities

Each identity has a fixed, always-selected manual account for signed quantities
missing from the platform API, independent of connection health. Identity
archive removes it from the boundary. This fills API coverage gaps without
adding external bank assets or transaction accounting.

Use immutable revisions and guarded edits. Before platform reads, each cycle
records manual reports that reference the current revisions, and retries keep
them; eligible own-platform public prices do not depend on private collection
success. Current and pending edits remain distinct from collected values. First
valuation applies the current local mapping with the source price cutoff, then
freezes the adopted result. Effective quantity changes make crossing comparisons
incomplete without creating capital flows.

A signed ledger can represent liabilities and offsets without guessing missing
interfaces. Users must maintain quantities and remove settled amounts once the
API includes them; duplicate counting remains possible. Revision references and
version checks add state but prevent inconsistent retries and silent overwrite.
Collection time does not imply user reconfirmation.

[Manual rules](../domain-rules.md#manual-balances) own continuity and valuation;
[architecture](../architecture.md#manual-persistence-and-collection) owns
persistence and [presentation](../presentation.md#manual-editing) owns editing.
