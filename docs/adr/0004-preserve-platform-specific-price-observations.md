---
status: accepted
---

# Preserve platform-specific price observations

Prices can differ between Platforms and between price types for the same underlying Asset. Discover the Assets and Instruments requiring prices from observed Balances and Positions, include any explicitly pinned catalog entries, then retain each Platform's relevant spot market, contract mark, and other available market or index prices with their type, denomination, and observation time. Preserve these normalized source Observations rather than replacing them with a single canonical price, allowing valuations and Exposure calculations to retain their provenance.

## Consequences

- Cross-platform reference prices are derived results. Whether they are needed, how prices qualify for combination, and any weighting method remain future decisions; source Price Observations are retained independently.
- Default portfolio display uses USD and allows CNY, while original asset quantities and denominations remain available. Currency conversion uses time-qualified exchange rates; stablecoins are separate Assets and are not assigned a permanent one-to-one USD rate.
- Historical values retain the prices and currency-conversion basis applicable to their observation times. A later price observation does not silently revalue an earlier state.
