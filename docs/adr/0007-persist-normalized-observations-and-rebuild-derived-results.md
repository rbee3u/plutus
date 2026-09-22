---
status: superseded by ADR-0014, ADR-0016, and ADR-0020
---

# Persist normalized observations and rebuild derived results

Persist normalized Observations rather than Platform response payloads as the authoritative external data. Treat configuration, catalog mappings, the Capital Baseline, and Capital Flows as manual source facts, while latest state, valuations, exposure, contributed capital, value changes, and period summaries are replaceable projections rebuilt from current source data. This supersedes [ADR 0001](./0001-read-only-snapshot-based-portfolio-observation.md) by adding manual external capital records without introducing a transaction ledger, trading behavior, or investment-return attribution.

## Consequences

- Each Observation retains its Platform, Identity and Account scope where applicable, source observation time, collection time, coverage, completeness, and adapter or schema version. An Identity Snapshot groups Observations collected during one refresh even when their source times differ.
- Platform response payloads are not retained. A normalization error can be repaired in place or corrected by recollection, after which all affected projections are rebuilt; information omitted during normalization cannot otherwise be recovered from Plutus.
- Derived projections have no authoritative calculation history. Materialized results can be discarded and replaced after an Observation, mapping, Capital Baseline, or Capital Flow changes.
- A Portfolio has one logical Capital Baseline with its initial value and edit history retained. Capital Flows are editable external contributions or withdrawals recorded with an Asset quantity, effective time, and the entry or exit Account when known; movements between Accounts inside the Portfolio are not Capital Flows.
- Capital Flows retain their original Asset quantities. Contributed Capital and Value Change convert them at their effective times using time-qualified Price Observations and exchange rates; missing conversion inputs leave the result incomplete rather than zero.
- Contributed Capital derives from the current Capital Baseline and subsequent Capital Flows. A Value Change subtracts contributions and adds withdrawals between comparable Portfolio Valuations, or uses the Capital Baseline as its starting basis; it is an absolute amount rather than profit, P&L, or return.
- Value Changes apply only to the complete Portfolio and use calendar days, ISO weeks, calendar months, quarters, and years in the Portfolio timezone. An active period can expose a provisional period-to-date result; percentage returns are outside the initial scope.
- Evaluation cutoffs use each Identity's latest successful scope snapshot at or before the cutoff, as defined in [ADR 0013](./0013-select-identity-state-by-snapshot-batch-and-archive-time.md). Missing or stale coverage makes a result incomplete rather than zero; a period spanning a coverage change is incomparable, while later periods can resume once their endpoints have the same complete coverage. A Capital Baseline correction cannot fabricate missing historical Observations.
- Internal transfers are not recorded in the initial scope. Asynchronous observations of their source and destination Accounts can create temporary Value Change discrepancies that later refreshes resolve.
