---
status: superseded by ADR-0007
---

# Read-only snapshot-based portfolio observation

Plutus exists to observe personal assets, positions, and their changes over time. Its historical record consists of Identity Snapshots and related Price Observations rather than a transaction ledger from which balances must be reconstructed, keeping initial integrations focused on the information needed for portfolio observation. Trading, transfers, and arbitrage execution are outside the product boundary, and changes in equity are not described as investment returns because snapshots alone cannot distinguish performance from deposits, withdrawals, or other cash flows.
