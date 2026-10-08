---
status: accepted
---

# Manual quantities within exchange Identities

Each Identity has a fixed, always-selected Manual Account for signed quantities
missing from the Platform API, independent of Connection health. Identity
archive removes it from the boundary. This fills API coverage gaps without
adding external bank assets or transaction accounting.

Use immutable revisions and guarded edits. Each cycle freezes Manual inputs
before Platform reads and reuses them on retries; eligible own-Platform public
prices do not depend on private collection success. Current and pending edits
remain distinct from collected values. Effective quantity changes make crossing
comparisons incomplete without creating Capital Flows.

A signed ledger can represent liabilities and offsets without guessing missing
interfaces. Users must maintain quantities and remove settled amounts once the
API includes them; duplicate counting remains possible. Frozen inputs and
version checks add state but prevent inconsistent retries and silent overwrite.
Collection time does not imply User reconfirmation.

[Manual rules](../domain-rules.md#manual-balances) own continuity and valuation;
[architecture](../architecture.md#manual-persistence-and-collection) owns
persistence and [presentation](../presentation.md#manual-editing) owns editing.
