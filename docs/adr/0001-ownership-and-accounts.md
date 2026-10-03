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

## Frontend display aliases

Identity lists and creation forms use compact aliases for Account Structures
and Margin Modes. These are accepted presentation choices, not new domain
terms or changes to supported Accounts, API names, identifiers, or stored data.

| Platform | Account Structure aliases | Margin Mode aliases |
| -------- | ------------------------- | ------------------- |
| Binance | Classic Trading → Classic; Portfolio Margin → Unified | none |
| OKX | Unified account → Unified | Spot mode → SPOT; Futures mode → FUTURES; Multi-currency margin → MCM; Portfolio margin → PM |
| Bybit | Unified Trading Account 2.0 → Unified | Isolated Margin → ISO; Cross Margin → MCM; Portfolio Margin → PM |
| Kraken | Separate wallets → Classic; Unified Wallet → Unified | none |

The list joins structure and mode with ` · `, using `UNKNOWN` while a mode is
unavailable for OKX or Bybit. Known aliases render before the Platform catalog
loads. Unrecognized identifiers retain their catalog name when available,
otherwise their raw identifier. No full-name disclosure is added. Position
Margin Setting labels and existing position Portfolio Margin descriptions keep
their current wording.

The shorter names make lists easier to scan, at the cost of hiding native
structure names and version details. Shared aliases do not imply identical
Account arrangements or risk models across Platforms. In particular, Binance
`Unified` still denotes its supported Portfolio Margin structure, and Bybit
`MCM` still denotes its canonical Cross Margin mode.
