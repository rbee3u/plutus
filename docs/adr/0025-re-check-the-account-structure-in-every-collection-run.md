---
status: accepted, narrowed to the Account Structure by ADR-0028
---

# Re-check the Account Structure in every Collection Run

An Identity's Account Structure is chosen by the User when the Identity is created, cannot be edited afterwards, and is checked by the Adapter at validation ([ADR 0015](./0015-select-account-types-per-identity.md)). Some Platforms let the owner switch the account's structure in the exchange app, and keep answering the same read-only calls afterwards, often with the same fields under different rules. A check at validation alone would let collection continue under a label that no longer describes the account.

Every Adapter therefore reads the account's structure in one read-only call at the start of every Collection Run and compares it with the Identity's Account Structure, the same way validation does:

- A different structure, or one Plutus does not support, fails the whole Collection Run with class `unsupported` before any Account is collected. The detail names the Platform's structure and the configured one.
- `unsupported` is neither retried nor a reason to stop scheduling: the Connection stays enabled, and every selected Account carries its last successful report as Stale ([ADR 0019](./0019-carry-a-failed-account-as-stale-and-treat-success-as-complete.md)) until the owner switches the structure back or replaces the Identity with one in the new Account Structure.

This decision first covered one "Account Mode" that mixed the structure with the margin regime. [ADR 0028](./0028-separate-account-structure-from-margin-mode.md) split them: the Margin Mode is observed in the same call and stored rather than compared with a declaration, so switching it no longer fails collection.

## Consequences

- Each Collection Run costs one more read-only call: Binance `GET /sapi/v1/account/info`, OKX `GET /api/v5/account/config`, Bybit `GET /v5/account/info`, and Kraken's Futures balances when Main is selected.
- A structure switch shows up as a failed cycle at the next collection instead of as silently relabeled values.
