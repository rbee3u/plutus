---
status: accepted
---

# ADR 0001: Ownership and selected Accounts

## Decision

Each authenticated User owns independent Portfolios. Each Identity belongs to
one Platform and one Portfolio and uses its own Connection. Main and sub-account
Identities are unrelated. The User selects Accounts from supported Account
Types; only selected Accounts under enabled Identities define the Portfolio
Boundary.

An Account Structure is declared at Identity creation and cannot change. A
Margin Mode is observed during validation and every Collection Run and can
change the report mapping without changing Accounts. Applicable structure checks
follow the Platform's available interfaces; mismatches and unknown modes fail
explicitly. Credentials cannot rebind an Identity to another external login.

## Reason and trade-offs

Explicit ownership and selection make it clear which value belongs in a total.
Separating structure from margin regime avoids making Users predict a mutable
exchange setting. Platform terminology must be mapped by meaning, and switching
structure requires a new Identity. Shared Portfolios, parent/sub-account
traversal, and overlapping reuse of an Identity are outside the product.

The creation Portfolio is undeletable; deleting another Portfolio removes its
scoped data. Current relationships and boundary behavior are in
[domain model](../../CONTEXT.md#ownership-and-the-portfolio-boundary); supported
Accounts are in [product scope](../product-scope.md).
