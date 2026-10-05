---
status: accepted
---

# ADR 0011: Shared business ordering and stable holdings pagination

## Decision

Lists, groups, and selectors displaying Platform, Identity, or Account use the
same hierarchy: Platform, then Identity, then Account. Views containing only
some of those dimensions use their corresponding part of the order.

Platforms use the fixed business order Binance, OKX, Bybit, Kraken. Identities
use display names in case-insensitive alphabetical order, then immutable IDs for
ties. Account display names do not affect ordering: Accounts follow the Account
Type order of their declared Account Structure, then immutable IDs. Margin Mode
does not affect the order.

The accepted Account orders are:

| Platform / Account Structure        | Account Types in order                                                                              |
| ----------------------------------- | --------------------------------------------------------------------------------------------------- |
| Binance / Classic Trading           | Spot, Cross Margin, Isolated Margin, USDⓈ-M Futures, COIN-M Futures, Funding, Earn, Dual Investment |
| Binance / Portfolio Margin          | Spot, Portfolio Margin, Funding, Earn, Dual Investment                                              |
| OKX / Unified account               | Trading, Funding, Earn, Dual Investment                                                             |
| Bybit / Unified Trading Account 2.0 | Unified, Funding, Earn, Dual Investment                                                             |
| Kraken / Separate wallets           | Main, Multi-M, Coin-M, Derivatives Holding, Earn                                                    |
| Kraken / Unified Wallet             | Main, Coin-M, Derivatives Holding, Earn                                                             |

Inside an Account, futures Positions use Base Symbol alphabetically, then linear
before inverse, and perpetual before delivery within each family. Delivery
contracts use expiry from nearest to furthest before the native Symbol; missing
expiry follows known expiry in that kind. Native Symbol, long before short, and
the Platform Position reference complete the order. Missing Base follows all
known Bases in that Account and falls back to the native Symbol, side, and
Position reference. Do not infer a Base from a Symbol or merge distinct Assets
such as SHIB and 1000SHIB.

Balances inside an Account use Asset Symbol, native listing Symbol, and listing
ID. Earn Positions use their sorted invested Asset sequence (native listing
breaks Asset ties), then Flexible, Fixed, Bonded, Timed, On-chain, maturity with
unknown last, and stable source reference. Multi-Asset products stay together.
Dual Positions inside an Account use expiry, missing expiry last, and Position
reference. Account hierarchy groups retain that order regardless of their value.
Overview's Tree presents Platform and Identity totals in the ownership
hierarchy. Flat Balance grouping is retired by ADR 0014. History and Activity
timelines retain their time order; their Account detail lists use the shared
hierarchy.

Ordering happens before pagination and does not use financial measures,
freshness, or collection arrival order. The global and Account-detail futures
views use the same within-Account order. No user-selectable column sorting is
introduced.

## Reason and trade-offs

Symbol-first global ordering scattered related Accounts across the page, while
creation-time Identity ordering made management and holdings disagree. A shared
business hierarchy gives each view a predictable browsing path. Using explicit
Base metadata groups an underlying Asset's different contract types without
depending on Platform-specific contract spelling.

Trading Accounts precede funds and product Accounts. Kraken Derivatives Holding
is a non-trading holding wallet, so it precedes Earn; it is not a structured
product. Renaming an Identity deliberately moves its alphabetical position;
renaming an Account does not. Stable ID ties and explicit missing-metadata rules
make repeated reads predictable without inventing missing facts.

This is an accepted display policy. It adds no Account Types, Asset aggregation,
or exchange-app validation claims. Implementation mechanisms belong in
[architecture](../architecture.md#api-numbers-and-time).
