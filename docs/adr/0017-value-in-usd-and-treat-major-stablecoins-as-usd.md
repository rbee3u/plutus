---
status: accepted
---

# Value in USD only and treat USDT and USDC as USD

Supported Platforms report equity in USD and quote nearly every market in USDT, while none publishes a usable USDT/USD or USD/CNY rate. Requiring an observed stablecoin rate for every calculated value would leave every Valuation Point incomplete, and CNY display needs a third-party FX source. Plutus therefore has a single Valuation Asset, USD, and by explicit decision values USDT and USDC at exactly 1 USD. Prices for other Assets come from the same Platform's USDT market or the Position's mark price and are treated as USD through that equivalence.

## Consequences

- The Portfolio display Asset setting, the `exchange_rate` price type, and CNY support are removed. Original Asset quantities remain visible.
- A stablecoin de-peg is a known, accepted distortion that Plutus does not detect or flag.
- Reintroducing CNY or real stablecoin rates requires a chosen data source and a new decision; nothing in the schema assumes it will never happen, but nothing prepares for it either.
- This supersedes the currency rules in [ADR 0004](./0004-preserve-platform-specific-price-observations.md). Its rule that each Platform's own prices are retained with type and time still applies; cross-Platform price fallback is still not performed.
