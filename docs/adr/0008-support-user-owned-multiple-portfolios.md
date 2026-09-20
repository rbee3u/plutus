---
status: accepted
---

# Support user-owned multiple portfolios

Plutus supports authenticated Users, each of whom owns and directly manages one or more independent Portfolios. Portfolios are not shared, there is no application Administrator or role model in the first release, and each Platform Identity belongs to exactly one Portfolio so that observation history, valuation coverage, and Capital Flow boundaries remain unambiguous.

## Consequences

- Registration creates a default Portfolio, and a User can create, rename, switch, and archive additional Portfolios.
- An Identity cannot move after it has Observations. An empty Portfolio can be deleted, while a Portfolio with source facts can only be archived.
- User and Portfolio identifiers scope every application query. Filtering a Portfolio does not create another Portfolio, and one Identity cannot be reused to build overlapping Portfolios.
- The server operator can create, disable, and recover Users through the CLI but is not an application role and does not automatically own their Portfolios.
- The distinctions among User, Identity, Account, and Connection in [ADR 0002](./0002-separate-identities-accounts-assets-and-instruments.md) and [ADR 0006](./0006-configure-identity-access-and-discover-catalog-entries.md) continue to apply.
