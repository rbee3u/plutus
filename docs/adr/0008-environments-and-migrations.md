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
loads it, while bare `db reset` leaves an empty database for remote test setup.
The old `--profile` flag remains rejected. Seed generation requires the fake and
a master key and fails explicitly for invalid definitions or serialization.

The single Alice User owns healthy, edge-case, large and empty Portfolios. Long
histories cover three months with daily older points and two-hour points in the
last 14 local days; large-list history covers seven days. Event instants retain
surrounding points. All supported structures and observed Margin Modes have
examples. Complex anomalies are now visible in the standard walkthrough,
superseding the earlier test-only decision. They remain separated from the
healthy default Portfolio and have explicit read-level expectations.

Seeds and runtime fake collection share pure, versioned scenario definitions.
The original anchor is carried in sealed fake credentials; the external login
reference depends only on scenario identity. Platform prices are coherent across
Identities. Scripted Capital Flows and configuration changes end at the anchor;
subsequent collection evolves only Platform facts. Source fields, Earn detail
availability, report scopes and exclusion policy history are preserved; real
valuation code derives all Valuation Points. Source INSERT batches commit in one
transaction; Connection setup and each derived point commit separately.
Connection initialization uses the normal credential lifecycle with explicit
expected states. Ordinary fake keys retain their previous behavior.

Existing dev data is replaced by explicit reset rather than migrations or seed
backfills. No product capability, HTTP interface, schema or seed profile is
added.

## Reason and trade-offs

A single seed keeps startup predictable, while theme Portfolios isolate
persistent omissions from everyday browsing. Shared scenario generation prevents
rich seeded holdings from disappearing at the next Collection Cycle. Coherent
financial events reveal Value Change defects that disconnected flow records
hide. The original 400-day history and 45-day dense window remained too slow
after batch rebuild optimization: a reset with normal SQL logging took about 17
seconds on the development machine. The accepted startup target is at most 10
seconds there, so three months and a 14-day dense window supersede that volume
decision. Daily older points retain complete calendar walkthroughs; nearby event
samples and seven-day large-list history preserve failures, transitions and
pagination. Year and DST boundaries remain controlled-anchor test cases rather
than mandatory coverage at every current anchor. The BTC deposit moves to D−60
so its flow and before/after holdings remain in range. This reduces long-range
chart density while keeping the business scenarios. The scenario index and
differential acceptance tests make the trade-off reviewable. This seed horizon
is not a retention policy for collected data. Synthetic data remains distinct
from exchange-app validation. Disposable resets are accepted for dev data, not
retained production Observations.

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
