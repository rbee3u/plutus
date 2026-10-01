---
status: accepted
---

# ADR 0008: Isolated environments and populated-database migrations

## Decision

Development uses a deterministic fake and no live exchange keys. Remote test and
prod use live read-only keys and no fake. Each environment has separate
database, master key, and cookies. Agent sessions use only the isolated agent
instance or throwaway test databases, never the human, remote test, or prod
instances.

Runtime configuration, automated-test configuration, and explicit recording
credentials are separate. Recording never falls back to test or runtime keys.
Prod keys do not belong on a development machine.

Migrations are explicit. After prod's first migration, freeze the baseline and
append changes that work on populated databases. Prod is never reset. Live
Adapter connections use IPv4; proxy behavior is external to the application.

## Reason and trade-offs

Isolation makes local resets, synthetic tests, real recording, and production
operation distinguishable. Appended migrations preserve valuable observations
and configuration. Manual deployment and key management require operator care;
IPv4 simplifies exchange IP allowlisting but does not control an external proxy.

[Development](../development.md#environments),
[testing](../testing.md#test-configuration), and [deployment](../deployment.md)
own setup and upgrade procedures.
