---
status: accepted
---

# Allow reviewed Portfolio Asset Listing exclusions for future calculated values

A Platform may keep reporting a Balance for an Asset Listing that the User has reviewed and chosen to leave out of their Portfolio value. Repeated `no_price` issues then make every calculated Account Valuation incomplete. An absent price alone does not prove that a Balance has no value, and an Account Report's reported equity cannot reliably be split by Asset Listing. The exclusion is therefore an explicit, reversible User decision rather than a fabricated zero price or a change to an Observation.

## Decision

- A User may ignore any Asset Listing for one Portfolio, with a required review reason. The rule applies to that Platform Listing across the Portfolio's Accounts, only when their Account Valuation is calculated from Balances. A reported-equity Account retains its reported total and Balance display.
- The rule starts with the next Collection Cycle created after the change. It persists until manually restored. Existing Valuation Points do not change; a later mapping recalculation evaluates each old point using the exclusion state effective when its cycle began.
- Ignored Balances remain stored as Observations but do not contribute to calculated Account Valuations or ordinary Balance reads, and do not create `no_price` or `invalid_equity` omissions. A calculated Account may be complete under this explicitly reviewed policy. Asset Listings and audit events expose the current rule and reason; ordinary holdings and overview pages show no exclusion indicator.
- A Value Change spanning a relevant exclusion or restoration is incomplete because its endpoint totals use different rules. The change is not a Capital Flow or Boundary Event. Position and Capital Flow treatment is unchanged.

## Consequences

- This extends [ADR 0018](./0018-merge-asset-listings-by-symbol-with-manual-overrides.md), whose “mapping override is the only catalog action” statement no longer applies. Its global mapping and historical recalculation rules remain.
- This preserves [ADR 0021](./0021-report-each-account-with-one-aligned-account-report.md): reported Account equity is never reduced by an estimated Balance value.
- Reviewers must use the Asset Listings screen to see exclusions; a `complete` calculated value can intentionally omit reviewed Balances.
