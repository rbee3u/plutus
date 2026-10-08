---
status: accepted
---

# ADR 0002: Aligned reports, Complete Set, and stale values

## Decision

Each selected Account has one aligned Account Report per Collection Run.
Reported equity and fixed Plutus-defined measures sit alongside Balances,
Positions, Earn Positions, and Dual Positions. Unsupported measures remain null.

Only a successful report defines a Complete Set and removes absent holdings.
Failed Accounts use their latest earlier successful report with the same scope,
as stale; without compatible history their value is unknown. Known values are
summed, omitted value is named, and affected totals are incomplete.

Source observations have no editing API. Each completed cycle writes one
Valuation Point. Derived valuations and point results may be rebuilt from
retained source facts; financial history has no downsampling or retention limit.

## Reason and trade-offs

Aligned reports preserve the Platform's account boundary and prevent overlapping
equity from being added twice. Carry-forward preserves useful last-known value
without pretending it is fresh. Scope compatibility prevents reuse of reports
with different product coverage. These choices require explicit stale ages,
missing Accounts, omissions, and rebuild rules rather than a single number.

See [reports](../../CONTEXT.md#collection-and-account-reports) and
[persistence](../architecture.md#persistence-and-rebuilds) for current behavior.
