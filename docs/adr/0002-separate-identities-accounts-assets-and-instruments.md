---
status: accepted, amended by ADR-0015, ADR-0018, and ADR-0021
---

# Separate identities, accounts, assets, and instruments

Platforms use account terminology for both user identities and financial subdivisions, while Plutus has its own Users. Use `Platform -> Identity -> Account`: each main or subordinate platform identity is an Identity, its actual funding or trading subdivisions are Accounts, and product categories remain separate from Accounts. This keeps Account close to platform financial terminology and reserves User for Plutus login identities, while Asset identity is shared across Platforms and Instrument identity remains specific to a Platform and its market or contract characteristics.

## Consequences

- Each Identity belongs to one Platform. One person can have multiple Identities within or across Platforms; multiple API keys do not create additional Identities, and independent login capability is not required.
- Main and sub-account Identities are unrelated rows with their own Connections; Plutus records no parent–child relationship ([ADR 0015](./0015-select-account-types-per-identity.md)).
- Portfolio membership is defined by the Accounts the User selects under enabled Identities ([ADR 0015](./0015-select-account-types-per-identity.md)); filtering a view does not change that boundary.
- Map platform fields by meaning: an external `subaccount` can refer to an Identity, while a funding or unified trading account refers to an Account.
- Asset Listings preserve platform-native symbols and map to Assets by normalized symbol, with manual overrides for exceptions ([ADR 0018](./0018-merge-asset-listings-by-symbol-with-manual-overrides.md)). BTC and WBTC, and USD and USDT, remain separate Assets.
- Each Balance describes one Asset in one Account and can preserve overlapping reported measures such as total, available, and locked quantities. Each Position describes an open directional exposure in one Instrument and one Account; opposing Positions remain distinct.
- A unified trading Account can contain Balances and Positions without separate Accounts for each product category. A spot trade changes Balances without creating a Position, while a derivative contract can create a Position.
- Exposure aggregation is outside the first release ([ADR 0021](./0021-report-each-account-with-one-aligned-account-report.md)); a Balance of 1 BTC and a short BTC Position are both shown as they are.
- Connection enablement follows [ADR 0015](./0015-select-account-types-per-identity.md) and [ADR 0020](./0020-record-capital-flows-manually-with-read-only-credentials.md).
- User ownership and the rule that each Identity belongs to one Portfolio follow [ADR 0008](./0008-support-user-owned-multiple-portfolios.md).
