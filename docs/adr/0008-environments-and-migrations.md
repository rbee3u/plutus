---
status: accepted
---

# ADR 0008: Isolated environments and populated-database migrations

## Decision

Dev uses the fake without live keys; remote test/prod use live read-only keys
without the fake. Databases, master keys, and cookies are isolated. Agents use
only the agent instance or throwaway test databases, never human, remote test,
or prod instances. Prod keys stay off development machines.

Runtime, automated-test, and recording sources are separate. Recording never
falls back to runtime or test keys. DSN and master-key sources stay outside
TOML, in environment values or referenced secret files.

Configuration exposes deployment policies: Session lifetimes, finished-job
retention, and Connection validation budgets. Internal scheduling and Portfolio
creation defaults are code constants; Portfolio intervals remain editable. One
suffix names paired cookies; certificate paths determine TLS mode. Examples show
environment differences and permissions. Reject unknown TOML fields, removed
environment overrides, and invalid values. Migrate schema changes without
aliases.

Test/prod use separate deployment directories and configs, with minimal systemd
units, weak local-MySQL dependencies, and paced failure retries. Root execution
is an operator preference. Live Adapters use IPv4; proxies remain external.
Migrations are explicit: freeze the baseline after prod's first migration and
append migrations compatible with populated databases. Never reset prod.

## Reason and trade-offs

Isolation protects production; appended migrations preserve observations and
configuration. Minimal units inherit host defaults; weak MySQL dependencies
allow database restarts without stopping Plutus. Readiness checks remain
necessary. Fewer configuration choices reduce operator burden while retaining
meaningful policies; strict loading exposes mistakes. Manual upgrades, secret
management, and direct configuration migration require operator care. IPv4
simplifies exchange IP allowlisting without controlling external proxies.

[Development](../development.md#configuration),
[testing](../testing.md#test-configuration), and [deployment](../deployment.md)
own configuration details, setup, validation, and upgrade procedures.
