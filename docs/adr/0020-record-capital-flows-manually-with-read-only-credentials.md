---
status: accepted
---

# Record Capital Flows manually and require only read-only credentials

Platforms can list deposits and withdrawals, but reading that history needs API permissions beyond balances and positions, and even with it Plutus could not tell a withdrawal to the owner's other exchange from a withdrawal to the outside world. The owner values the smaller credential surface more than the convenience, so Capital Flows are entered manually and a Connection is expected to hold a key with read-only balance, position, and market-data permissions. Plutus never calls an endpoint that would require trading, withdrawal, transfer, or funding-history permission.

## Consequences

- Adapters implement only account, balance, position, and price reads. Adding any other call class requires a new decision.
- Plutus does not inspect a key's permission set; using a read-only key is documented guidance.
- A forgotten manual Capital Flow shows up as an unexplained Value Change; Boundary Events under [ADR 0016](./0016-treat-boundary-events-as-implicit-capital-flows.md) remove the most common cause.
- Capital Flows are editable rows whose changes are recorded in `audit_events`; the separate revision-chain tables are removed.
