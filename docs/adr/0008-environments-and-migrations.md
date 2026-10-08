---
status: accepted
---

# ADR 0008: Isolated environments and populated-database migrations

## Decision

Dev uses the fake without live keys; remote test/prod use live read-only keys
without the fake. Databases, master keys, and cookies are isolated. Agents use
only the agent instance and throwaway test databases. Prod keys stay off
development machines.

Runtime, test, and recording credentials have separate sources; recording has no
fallback. DSNs and master keys stay outside TOML. Configuration exposes
deployment policies; internal scheduling defaults remain code constants, and
Portfolio intervals remain editable. Unknown fields, removed overrides, and
invalid values fail explicitly. Configuration migrations have no legacy aliases.

Migrations are explicit. Freeze the baseline after prod's first migration and
append migrations compatible with populated databases. Never reset prod.
Test/prod use separate directories and minimal systemd units with weak MySQL
dependencies and paced retries. Root execution is an operator preference.

Live Adapters use one instance-wide IPv4/proxy policy; a forced proxy never
falls back to direct. External software owns tunnels and their credentials.
Recording keeps its separate process-environment proxy policy. See
[proxy configuration](../development.md#platform-proxies).

Dev uses one deterministic dataset shared by seeds and continued fake
collection. Real valuation code derives points; scripted Plutus events stop at
the seed anchor. Replace existing dev data by explicit reset, not backfill.

## Reason and trade-offs

Isolation protects retained data and credentials. Appended migrations preserve
Observations; manual upgrades and secret management remain operator duties. Weak
MySQL dependencies allow database restarts without stopping Plutus, so readiness
must still be checked. Strict configuration catches mistakes without maintaining
aliases for obsolete settings.

Instance-wide routing keeps deployment predictable without managing tunnels.
Restarting to apply configuration is accepted; per-Platform routing,
authenticated proxy URLs, and automatic direct fallback are not offered.

One seed simplifies startup; separate Portfolios isolate persistent anomalies.
Shared scenarios preserve coherent holdings after collection. The original
400-day history missed the 10-second reset target, so the accepted dataset
reduces density and uses controlled-anchor tests for year and DST boundaries.
[Seed timing evidence](../seeds.md#performance-revision-check-2026-10-05)
records the trade-off; it changes neither financial retention nor real coverage.

[Development](../development.md) owns setup and configuration,
[seeds](../seeds.md) owns scenarios and timing evidence,
[testing](../testing.md#test-configuration) owns test isolation, and
[deployment](../deployment.md) owns upgrades.
