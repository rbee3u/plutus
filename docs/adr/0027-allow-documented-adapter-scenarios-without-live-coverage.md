---
status: accepted
---

# Distinguish documented adapter support from live coverage

[ADR 0037](./0037-separate-recorded-evidence-from-synthetic-tests.md) retains this evidence distinction: documentation-based synthetic tests show supported mappings; independent reviewed recordings show observed real response shapes. New captures never become ordinary test inputs.

Deliver Bybit and Kraken together at the existing Binance/OKX data-integration and smoke-test standard. Bybit initially supports the global UTA 2.0 service (including UTA 2.0 Pro), Unified, Funding, and flexible/fixed savings; on-chain staking and structured Earn products are outside this delivery. Kraken supports both wallet structures in [ADR 0026](./0026-support-kraken-separate-and-unified-wallets.md), with the available test credentials expected to use Unified Wallet.

When test credentials, permissions, or holdings cannot exercise a supported path, implement it from official documentation and test explicitly labeled documentation-derived or synthetic responses. This is a scoped exception to the recorded-only fixture rule in [ADR 0023](./0023-test-platform-adapters-in-three-layers.md), not evidence of a successful live smoke test. Record available real responses through the fixture recorder, keep their provenance distinct, and report unavailable or unexercised paths for later owner acceptance. An empty successful response verifies access and empty-state handling, not nonempty field mappings. Never fabricate recording dates or treat skipped tests as successful live coverage.

Implementation proceeds without intermediate owner intervention. Fixtures and the two adapter deliveries are reviewed together at the end; unreviewed recordings are never committed. All other validation, read-only access, raw-payload, completeness, and environment rules remain in force.
