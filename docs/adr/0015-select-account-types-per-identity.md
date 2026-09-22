---
status: accepted
---

# Select Account Types per Identity instead of discovering Accounts

Platforms expose several wallets per Identity (unified trading, funding, spot, earn), and some exist only after the owner enables them on the Platform. Automatic discovery would have to probe every wallet type, silently skip failures, and still could not know which wallets the owner wants counted. The owner knows their wallet structure, so each Adapter declares the Account Types it supports for each Account Mode, and the User multi-selects the Account Types to collect when configuring an Identity. Plutus collects, values, and displays only selected Accounts; unselected Account Types are never probed.

## Consequences

- `accounts` rows are User configuration, not discovery results. A selected Account whose Platform call fails is shown as failed with the Platform's reason; it does not disappear.
- The `account_mode` on an Identity is a declared expectation. The Adapter reports a mismatch as a Connection validation failure.
- Sub-accounts are configured as their own Identities with their own Connections. There is no parent–child relationship and no collection of sub-account data through a main-account credential.
- Deselecting an Account removes it from the Portfolio Boundary, which is a Boundary Event under [ADR 0016](./0016-treat-boundary-events-as-implicit-capital-flows.md).
- This supersedes the discovery rules of [ADR 0006](./0006-configure-identity-access-and-discover-catalog-entries.md).
