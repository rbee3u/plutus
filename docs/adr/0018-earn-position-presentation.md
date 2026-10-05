---
status: accepted
---

# ADR 0018: Earn descriptions and product observation health

## Decision

Holdings offers Earn between Futures and Dual Investment, covering the existing
four-Platform product scope. Overview's expanded Earn Account shares the table,
without repeated Platform and Identity columns. Balances remain independently
visible. Both product tables put source observation time and row-local stale or
data issues in the final Observed column; Status contains product phases alone.

Earn columns are Platform, Identity, Asset, Type, Maturity, Rate, Amount,
Rewards, Subscribed, Status, Observed. Existing numeric precision, timezone,
filter, pagination, and horizontally scrolling table conventions apply. Details
are available through pointer, keyboard focus, and click/touch disclosures.

Rows preserve source granularity: individual positions where supplied, product
or strategy aggregates otherwise. Multi-Asset products remain one row, with
Asset amounts and separately qualified rewards. Type uses Flexible, Fixed,
Bonded, Timed, On-chain; product names and lock/redemption rules remain in
details. The page counts Positions rather than pretending all rows are
subscriptions. Missing product descriptions make its count explicitly partial.

Amount retains its principal, total, or allocated basis. Rewards retain their
currency and scope rather than forming one cumulative-interest number. Rate
replaces APY in both Earn and Dual tables: preserve APR/APY and estimated,
range, tier, and additional-reward conventions. A configured minimum lending
rate and rewards with unverified scope belong in details. No compounding
conversion or original-principal reconstruction is accepted.

Maturity means a reported term end. Binance Locked uses `rewardsEndDate`, an
explicit inference from its API field and Help's Interest end date definition;
principal `deliverDate` remains separate. Binance's API labels its locked rate
APY while Help describes APR: retain the API label and explain the discrepancy.
OKX estimated redemption settlement and Kraken lifecycle expiries cannot become
investment maturity. Do not calculate dates from term duration.

Bybit FlexibleSaving's public product catalogue supplies estimated APR through
one optional read per Collection Run. It is read only for selected Earn with
nonzero flexible holdings, matched by product and coin. Failure, absent
products, or ambiguous matches produce metadata issues without changing known
funds or Connection validity. Catalogue availability is not position lifecycle
Status.

New reports distinguish collected descriptions from unavailable descriptions.
Old source reports show known Balance summaries separately with their original
Observed time; they are not fabricated product positions. No evidence-file or
current-product backfill is accepted. Description availability changes neither
report scope, Account valuation, nor financial completeness. Successful reports
alone remove holdings, using existing compatible stale fallback on failure.

## Reason and trade-offs

Earn product terms answer where funds are held and when they may return.
Aggregating everything by Asset loses independent maturities and product stages;
forcing every row into a subscription invents detail flexible APIs do not have.
The shared table is wider, but preserves direct comparison while keeping less
comparable rules and provenance in disclosures.

Keeping descriptive quantities and rewards separate from valuation prevents
double counting and preserves existing financial history. An explicit legacy
summary preserves known value at the cost of incomplete product-level browsing
until a new successful Collection. Normal unavailable fields remain quiet;
actual metadata faults and stale reports remain discoverable.

## Sources

- [Binance Flexible and Locked API](https://developers.binance.com/en/docs/catalog/investment-and-services-simple-earn/api/rest-api/flexible-locked)
- [Binance Locked products](https://www.binance.com/en/support/faq/detail/af607a50de614350ae57be3e6f616699)
- [OKX Savings balance](https://my.okx.com/docs-v5/en/#financial-product-simple-earn-flexible-get-saving-balance)
- [OKX On-chain orders](https://my.okx.com/docs-v5/en/#financial-product-on-chain-earn-get-active-orders)
- [Bybit product information](https://bybit-exchange.github.io/docs/v5/finance/earn/easy-onchain/product-info)
- [Kraken allocations](https://docs.kraken.com/api-reference/earn/list-earn-allocations)
