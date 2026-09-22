---
status: accepted
---

# Support user-owned multiple portfolios

Plutus supports authenticated Users, each of whom owns and directly manages one or more independent Portfolios. Portfolios are not shared, there is no application Administrator or role model in the first release, and each Platform Identity belongs to exactly one Portfolio so that observation history, the Portfolio Boundary, and Capital Flows remain unambiguous. A Portfolio is one person's ledger; it is a data-isolation boundary, not a feature to extend.

## Consequences

- CLI user creation creates an undeletable Portfolio named `default`. A User can create, rename, and switch additional Portfolios. The birth Portfolio stays undeletable after rename.
- An Identity cannot move after it has Observations. Additional Portfolios can be deleted, which cascade-deletes every row scoped to them. The undeletable Portfolio cannot be deleted. Portfolio archive and restore are not part of the product.
- User and Portfolio identifiers scope every application query. Filtering a Portfolio does not create another Portfolio, and one Identity cannot be reused to build overlapping Portfolios.
- The server operator can create, disable, and recover Users through the CLI but is not an application role and does not automatically own their Portfolios.
- The distinctions among User, Identity, Account, and Connection in [ADR 0002](./0002-separate-identities-accounts-assets-and-instruments.md) and [ADR 0015](./0015-select-account-types-per-identity.md) continue to apply.
