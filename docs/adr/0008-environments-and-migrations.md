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
is an operator preference. Live Adapters use IPv4 for direct access and proxy
first hops. Runtime configuration chooses an instance-wide environment, direct,
or forced proxy policy. Environment is the compatible default; explicit policies
ignore environment proxy settings, including bypass rules. Forced proxy failure
never falls back to direct. HTTP/HTTPS/SOCKS entry points are supported without
URL credentials; SS protocols and tunnel lifecycle remain external. Adapter
recording keeps its separate process-environment proxy behavior. Migrations are
explicit: freeze the baseline after prod's first migration and append migrations
compatible with populated databases. Never reset prod.

Dev has one deterministic walkthrough seed rather than selectable profiles.
`db seed` loads it into an empty migrated database; `db reset --seed` resets and
loads it, while bare `db reset` still leaves an empty database for remote test
setup. The old `--profile` flag is removed without aliases. Seed data retains
the normalized Position and Account Report source fields used by collection. Its
latest point has one stale COIN-M Futures Account with one Position; other
Accounts are healthy. Later normal fake collection recovers that Account.
Historical failures remain available, and complex anomaly fixtures belong only
to regression tests. Existing dev data is replaced by an explicit reset rather
than rewritten through migrations or seed backfills.

## Reason and trade-offs

A single seed makes dev startup predictable and keeps its financial source facts
aligned with the fake Adapter. One recoverable stale example demonstrates
failure handling without making every Position look defective. Keeping complex
anomalies in tests preserves regression coverage without a second public data
choice. Replacing seed history through reset is accepted for disposable dev
data; it is not a policy for changing retained production Observations.

Isolation protects production; appended migrations preserve observations and
configuration. Minimal units inherit host defaults; weak MySQL dependencies
allow database restarts without stopping Plutus. Readiness checks remain
necessary. Fewer configuration choices reduce operator burden while retaining
meaningful policies; strict loading exposes mistakes. Manual upgrades, secret
management, and direct configuration migration require operator care. IPv4
simplifies exchange IP allowlisting without controlling external proxies.
Explicit policies make deployment routing predictable while retaining existing
environment behavior by default. External tunnels avoid coupling protocol
versions, credentials, and process lifecycle to Plutus. Restarting to apply
configuration is accepted; per-Platform routing, authenticated proxy URLs, and
automatic direct fallback are not offered.

[Development](../development.md#configuration),
[testing](../testing.md#test-configuration), and [deployment](../deployment.md)
own configuration details, setup, validation, and upgrade procedures.
