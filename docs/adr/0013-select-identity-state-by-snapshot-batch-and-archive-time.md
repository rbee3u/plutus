---
status: accepted
---

# Select identity state by snapshot batch and archive time

An Identity Snapshot is the batch for one Identity. At a cutoff, each scope uses the latest snapshot completed at or before that cutoff in which the scope succeeded. Keys absent from that successful scope are no longer held. A failed scope keeps the previous successful batch and is stale. A wholly failed snapshot does not become the latest batch. Price observations stay time-qualified and are not selected as part of the identity batch.

Archive records `archived_at` and removes the Identity from current portfolio totals. Cutoffs before `archived_at` still use its batches. `disabled` does not end membership: existing batches still count, and a disabled Identity is not required coverage. An enabled Identity, and an archived Identity at a cutoff before `archived_at`, with no successful batch makes that valuation incomplete rather than zero. Archive is refused while a subordinate is still unarchived or a Connection is still enabled, and an archived row cannot be patched. The platform identity key is unique only among rows that are not archived, so the same external ref can be created again. The new row does not inherit the archived row's snapshots or Connections.

Per-key carry-forward treated a closed position that the platform omitted as still open. Dropping an archived Identity from every historical cutoff rewrote past portfolio value. Keeping one row forever blocked creating the same platform identity again.
