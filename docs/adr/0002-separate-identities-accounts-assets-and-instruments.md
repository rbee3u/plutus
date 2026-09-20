---
status: accepted
---

# Separate identities, accounts, assets, and instruments

Platforms use account terminology for both user identities and financial subdivisions, while a future Plutus login system may have its own users. Use `Platform -> Identity -> Account`: each main or subordinate platform identity is an Identity, its actual funding or trading subdivisions are Accounts, and product categories remain separate from Accounts. This keeps Account close to platform financial terminology and reserves User for Plutus login users, while Asset identity is shared across Platforms and Instrument identity remains specific to a Platform and its market or contract characteristics.

## Consequences

- Each Identity belongs to one Platform. One person can have multiple Identities within or across Platforms; multiple API keys do not create additional Identities, and independent login capability is not required.
- Main and subordinate Identities each have their own Accounts, with a parent-child relationship expressing management. That relationship does not merge their asset scopes or establish the coverage of equity reports.
- Portfolio membership is defined by enabled Identities; selecting Accounts or other dimensions filters a view without changing the Portfolio boundary. An Identity Snapshot covers one Identity across its Accounts, retaining source observation times and completeness.
- Map platform fields by meaning: an external `subaccount` can refer to an Identity, while a funding or unified trading account refers to an Account.
- Asset Listings preserve platform-native identities and map them to Assets. BTC on two Platforms can contribute to the same Asset total; BTC and WBTC, and USD and USDT, remain separate Assets. Matching tickers alone do not establish identity.
- Each Balance describes one Asset in one Account and can preserve overlapping reported measures such as total, available, and locked quantities. Each Position describes an open directional exposure in one Instrument and one Account; opposing Positions remain distinct.
- A unified trading Account can contain Balances and Positions without separate Accounts for each product category. A spot trade changes Balances without creating a Position, while a derivative contract can create a Position.
- Basic Exposure distinguishes long, short, and net amounts in comparable underlying-asset units. A Balance of 1 BTC and a short Position equivalent to 1 BTC can produce zero net BTC Exposure while both remain visible and account equity remains nonzero.
- Connection enablement and automatic catalog discovery follow [ADR 0006](./0006-configure-identity-access-and-discover-catalog-entries.md).
