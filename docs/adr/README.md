# Architecture decision records

These records describe current accepted decisions, their reasons, and meaningful
trade-offs. [Glossary](../../GLOSSARY.md) owns terms,
[domain rules](../domain-rules.md) own business behavior,
[presentation](../presentation.md) owns display and interaction rules, and
[architecture](../architecture.md) owns technical mechanisms. Operational guides
own setup, validation, and deployment procedures.

## Records

| ADR                                                | Decision                                                |
| -------------------------------------------------- | ------------------------------------------------------- |
| [0001](./0001-ownership-and-account-boundaries.md) | Ownership and account boundaries                        |
| [0002](./0002-reports-and-compatible-history.md)   | Aligned reports and compatible history                  |
| [0003](./0003-valuation-and-asset-policy.md)       | USD valuation and asset policy                          |
| [0004](./0004-value-change-and-boundary-flows.md)  | Absolute value change and boundary flows                |
| [0005](./0005-earn-and-dual-products.md)           | Earn and dual product boundaries and descriptions       |
| [0006](./0006-manual-balances.md)                  | Manual quantities within exchange identities            |
| [0007](./0007-backend-architecture.md)             | Contract-first monolith and explicit backend boundaries |
| [0008](./0008-authentication-and-credentials.md)   | CLI-provisioned users and write-only credentials        |
| [0009](./0009-adapter-verification.md)             | Independent synthetic tests and recorded evidence       |
| [0010](./0010-environments-and-configuration.md)   | Isolated environments and explicit configuration        |
| [0011](./0011-retained-data-and-upgrades.md)       | Retained source facts and populated-database upgrades   |
| [0012](./0012-process-logging.md)                  | Unified process logging and sink ownership              |
| [0013](./0013-frontend-architecture.md)            | Feature ownership and shared browser mechanisms         |
| [0014](./0014-position-provenance.md)              | Contract quantities and margin provenance               |
| [0015](./0015-prelaunch-schema.md)                 | Prelaunch schema and retained financial provenance      |

## Maintenance

Update the relevant record when an accepted decision changes. Add a record only
when the choice is hard to reverse, surprising without its context, and the
result of a real trade-off; all three conditions must hold. Routine rules and
reversible presentation preferences belong in their owning guides.

Describe the current choice and why it is appropriate, with non-obvious costs or
rejected alternatives where useful. Do not preserve change narratives, retired
designs, or duplicate specifications. Distinguish accepted policy from
implementation gaps and evidence limits.

Use the next sequential four-digit number for a new record and keep identifiers
stable during ordinary maintenance. Link to canonical rules, mechanisms, and
procedures rather than copying them.
