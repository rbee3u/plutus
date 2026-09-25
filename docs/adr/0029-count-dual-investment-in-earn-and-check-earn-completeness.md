---
status: accepted
---

# Count dual investment in Earn and check Earn for unread products

Dual investment (Binance Dual Investment, OKX Dual Investment, Bybit Dual Asset) takes principal in one coin and settles it at expiry in that coin or the other one, depending on the settlement price. While a subscription runs, its principal is in none of the wallets Plutus reads, and each Platform shows it under Earn. More generally, Platforms keep adding Earn products, and a successful Earn report is a Complete Set: a product Plutus cannot read would vanish from the total without a trace.

- Dual investment is read into the existing `earn` Account on Binance, OKX, and Bybit. Each subscription counts as its principal in the invested coin: Binance `subscriptionAmount` of `investCoin`, OKX `notionalSz` of `notionalCcy`, Bybit `amount` of `investCoin`. Expected yield is not counted before the Platform pays it.
- Only running subscriptions count. Binance: `PENDING`, `PURCHASE_SUCCESS`, `SETTLING`, `REFUNDING`. OKX: `initial`, `pending_book`, `live`, `pending_settle`, `pending_redeem_booking`, `pending_redeem`, `redeeming`. Bybit: `Active`, `Redeeming`. Finished states are skipped, and a state the Platform has not documented fails the Earn report with `normalization`.
- Every Earn report is also checked for completeness at runtime. The Adapter reads the Platform's own USD valuation of Earn: Binance's `Earn` wallet from `GET /sapi/v1/asset/wallet/balance`, the `earn` bucket of OKX's `GET /api/v5/asset/asset-valuation`, and the Earn bucket of Bybit's asset overview. It compares that figure with the itemized Earn Balances valued at the Adapter's own prices. If the Platform's figure is higher by more than 1% of it and by at least 1 USD, the Earn report fails with `unsupported`, naming both amounts. The Earn Account is then carried Stale ([ADR 0019](./0019-carry-a-failed-account-as-stale-and-treat-success-as-complete.md)) instead of losing that value.
- If an itemized Balance has no price, the comparison is skipped, and valuation already marks the Account Incomplete. Kraken has no such Earn valuation and is not checked.

## Consequences

- Products settle at 08:00 UTC. Binance pays out within about six hours and Bybit within 5 to 30 minutes, into a wallet the Platform chooses. A Collection Run in that window can see the principal in neither place, a dip, or in both, a double count. This is accepted: the next run after payout is correct again, and the History shows the blip rather than hiding it.
- The check catches value Plutus fails to read, not value it counts twice, and its tolerance absorbs the difference between the Platform's prices and the Adapter's.
- An Earn product Plutus does not read yet, such as OKX Shark Fin or Snowball, shows up as a stale Earn Account whose detail names the gap, rather than as a silent drop in value.
- If a Platform refused its dual investment endpoint to a read-only key, dual investment would be left out for that Platform, and the check would turn a held subscription into a stale Earn Account.
