---
status: accepted
---

# Treat boundary events as implicit capital flows and drop the capital baseline

Value Change must not report a gain when the owner adds a funded exchange account to Plutus, nor a loss when they deselect one. The earlier design declared such intervals incomparable and also kept a manually maintained Capital Baseline of cumulative contributed capital, which could not be valued when the starting holdings were in several assets. Because the owner does not need results from before Plutus began observing, both problems disappear: the Portfolio Boundary is the set of selected Accounts under enabled Identities, an Account entering the boundary is an implicit contribution equal to its first Account Valuation, an Account leaving it is an implicit withdrawal equal to its last Account Valuation, and Value Change between any two Valuation Points is end minus start minus contributions plus withdrawals, with implicit and manual Capital Flows treated alike.

## Consequences

- Capital Baseline, Contributed Capital, and "since inception" totals are removed. Value Change exists only between two Valuation Points.
- Boundary Events are derived at query time from consecutive Valuation Points; they are not stored as Capital Flow rows but are listed alongside manual Capital Flows so the owner can see why a total moved without a Value Change.
- The `incomparable` state is removed. `incomplete` remains for missing data.
- A transfer into an Account made before that Account was selected in Plutus is a real withdrawal from the boundary and must be recorded manually. The documented practice is to select an Account in Plutus before funding it.
- A manual Capital Flow in a non-USD Asset is converted with the nearest Price Observation from the Account's Platform at its effective time; when none exists, the affected Value Change is incomplete and names the flow.

[ADR 0033](./0033-separate-dual-investment-accounts-and-positions.md) treats splitting an observed combined Earn Account into Earn and Dual Investment as reclassification, not a new contribution. Cross-scope Value Changes are incomplete; ordinary later selections remain Boundary Events.
