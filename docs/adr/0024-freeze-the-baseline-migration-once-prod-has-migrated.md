---
status: accepted
---

# Freeze the baseline migration once prod has migrated

While no database held data worth keeping, the single baseline migration was edited in place: every environment was reset and re-migrated, and `db reset` was the upgrade path. The prod environment ([ADR 0022](./0022-separate-dev-test-and-prod-environments.md)) is never reset, so its first `migrate up` ends that period.

- Until prod runs `migrate up` for the first time, the baseline may still be edited in place and every environment is reset afterwards.
- From that moment the baseline is frozen. Every schema change is a new, sequentially numbered Goose migration that runs correctly on a populated database, and `ExpectedSchemaVersion` moves with it.
- Known future needs are pre-empted in the baseline where they cost nothing: `kraken` is already accepted by the `platform` CHECK constraints. The Kraken Account Structures ([ADR 0026](./0026-support-kraken-separate-and-unified-wallets.md)) first arrived as an appended migration. That migration was folded back into the baseline when the Account Structure and Margin Mode columns replaced the Account Mode ([ADR 0028](./0028-separate-account-structure-from-margin-mode.md)), still before prod's first `migrate up`.

## Consequences

- The technical design's schema catalog describes the current cumulative schema; the migration files describe how to get there.
- test and dev environments may still be reset freely; a migration that only works on an empty database is not acceptable once the freeze is in force.
