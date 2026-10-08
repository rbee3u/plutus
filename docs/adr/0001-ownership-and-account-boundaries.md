---
status: accepted
---

# Ownership and Account boundaries

Each User owns independent Portfolios, and each Identity belongs to one Platform
and Portfolio. Main and sub-account logins are separate Identities with their
own Connections. Selected Accounts under enabled Identities define the Portfolio
Boundary; the fixed Manual Account also belongs to it.

Account Structure is declared and immutable; Margin Mode is observed and can
change report interpretation. Credentials bind to one external login rather than
rebinding an Identity when keys change. This preserves ownership and historical
meaning without requiring Users to predict mutable exchange modes. Changing
structure requires a new Identity and archiving the old one.

Shared Portfolios, main-key traversal of sub-accounts, and moving an Identity
between Portfolios after Observations exist are not supported. These boundaries
keep totals attributable but require explicit configuration for each login. See
[ownership rules](../domain-rules.md#ownership-and-the-portfolio-boundary),
[access rules](../domain-rules.md#account-structure-margin-mode-and-connection),
and [offered Accounts](../product-scope.md#supported-accounts).
