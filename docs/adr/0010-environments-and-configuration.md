---
status: accepted
---

# Isolated environments and explicit configuration

Dev uses the fake without live keys; remote test and prod use live read-only
keys without the fake. Databases, master keys, and cookies are isolated. Agents
use only the agent instance and throwaway test databases; prod keys stay off
development machines. Runtime, test, and recording credentials have separate
sources, and recording has no credential fallback.

Ordinary settings use TOML and code defaults; environment variables supply
secrets. DSNs and master keys stay outside TOML. Non-secret environment names
are silently ignored; unknown TOML fields and invalid values fail. There are no
legacy configuration aliases. Deployment policies are configurable, internal
scheduling defaults remain code constants, and portfolio intervals are editable.

Live adapters share an instance-wide IPv4/proxy policy. Runtime and recording
default to direct access and accept only explicit TOML proxies, never proxy
environment variables. A forced proxy never falls back to direct. Recording's
optional TOML contains only proxy policy and loads no runtime configuration or
secrets. External software owns tunnels and their credentials.

These boundaries protect credentials and make routing predictable without a
second configuration precedence system. Ignored non-secret environment values
receive no startup diagnostics; operators must configure TOML explicitly.
Configuration requires restart. Per-platform routing, authenticated proxy URLs,
and automatic direct fallback are not offered.

One deterministic dataset supplies seeds and ongoing fake collection. Real
valuation derives points; scripted Plutus events stop at the seed anchor.
Explicit reset replaces dev data rather than backfilling it. Bounded seed
history keeps reset practical; controlled-anchor tests cover long calendar and
DST boundaries without changing financial retention.

[Development](../development.md) owns configuration and proxy setup;
[testing](../testing.md#test-configuration) owns database isolation;
[seeds](../seeds.md) owns scenarios and performance evidence.
