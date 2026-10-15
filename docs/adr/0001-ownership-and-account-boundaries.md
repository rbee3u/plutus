---
status: accepted
---

# Ownership and account boundaries

Each user owns independent portfolios, and each identity belongs to one platform
and portfolio. Catalogs and prices are local to that portfolio; duplicate market
facts buy independent correction and deletion. Main and sub-account logins are
separate identities with their own connections. Selected accounts under enabled
identities define the portfolio boundary; the fixed manual account also belongs
to it.

Account structure is declared and immutable; margin mode is observed and can
change report interpretation. Credentials bind to one external login rather than
rebinding an identity when keys change. This preserves ownership and historical
meaning without requiring users to predict mutable exchange modes. Changing
structure requires a new identity and archiving the old one.

Shared portfolios, main-key traversal of sub-accounts, and moving an identity
between portfolios after observations exist are not supported. These boundaries
keep totals attributable but require explicit configuration for each login. See
[ownership rules](../domain-rules.md#ownership-and-the-portfolio-boundary),
[access rules](../domain-rules.md#account-structure-margin-mode-and-connection),
and [offered accounts](../product-scope.md#supported-accounts).
