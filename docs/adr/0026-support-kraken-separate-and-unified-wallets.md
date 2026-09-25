---
status: accepted
---

# Support Kraken Separate wallets and Unified Wallet with one credential bundle

Kraken offers Separate wallets and Unified Wallet, which merges Main Spot, Spot Margin, and Multi-M derivatives while leaving Coin-M and allocated Earn separate. Support both as `kraken_separate` and `kraken_unified`. Offer Main, Earn, Coin-M, and Derivatives Holding in both Account Structures, and Multi-M only in Separate wallets. Main includes Auto Earn; allocated Earn excludes Auto Earn. Each selected Account produces one complete, non-overlapping Account Report, including net margin effects; as with OKX, Spot Margin trades are not itemized as Positions. Support linear/inverse perpetual and delivery contracts, not options or independent lending products.

One Identity has one Connection containing separate Spot and Futures key pairs, replaced atomically. Main requires both pairs: the documented `flex_futures.unified_balances` flag on Futures WebSocket balances verifies the structure at validation and collection. Earn alone requires only Spot credentials; independent derivatives Accounts require only Futures credentials. Missing required credentials prevent validation and selection on an enabled Connection. Authentication failures retain the existing whole-Connection invalidation rule; ordinary Account failures retain Account-level stale carry-forward. Account selection and credential requirements must remain consistent after edits.

The additional key pair extends the encrypted document compatibly. The two structures first needed an appended migration for the identity constraint; before prod's first `migrate up` that migration was folded into the edited baseline ([ADR 0024](./0024-freeze-the-baseline-migration-once-prod-has-migrated.md), [ADR 0028](./0028-separate-account-structure-from-margin-mode.md)). This extends the initial Platform scope in [ADR 0005](./0005-support-explicitly-selected-account-structures-and-margin-modes.md) and follows [ADR 0025](./0025-re-check-the-account-structure-in-every-collection-run.md). Kraken structures have no Margin Mode. Haircut-adjusted collateral is not nominal equity; shared balances, funding, margin profit, and Earn receipts must never be counted twice.

Sources: [wallet boundaries](https://support.kraken.com/articles/understanding-wallets-kraken-pro), [Unified Wallet](https://support.kraken.com/articles/introduction-unified-wallet), [Futures balances and wallet flag](https://docs.kraken.com/exchange/api-reference/futures-websocket/balances).
