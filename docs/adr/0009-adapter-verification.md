---
status: accepted
---

# ADR 0009: Independent synthetic tests and recorded evidence

## Decision

Use independent synthetic responses for automated Adapter tests and
owner-reviewed recorded responses for interface evidence. Tests must not read
recordings. Documented support, observed response coverage, and exchange-app
reconciliation are separate claims; none substitutes for the others.

Ordinary verification is network-guarded and uses loopback replay, fake data,
and temporary MySQL. Only explicit single-interface recording contacts exchanges
for development evidence. Recording uses separate credentials and writes
sanitized responses into `/tmp/plutus/` for review. Accepted evidence is frozen;
replacement needs a coverage-gap, interface-change, or error-fix reason.

## Reason and trade-offs

Deliberate synthetic scenarios provide stable business assertions without
forcing real account preparation to match tests. Separate evidence preserves
actual contracts and exposes differences from documentation. This gives up
automatic replay of every recording and requires explicit review, named gaps,
and independent regression tests for parsing fixes.

Temporary artifacts stay outside the checkout under `/tmp/plutus/`, with unique
subdirectories for concurrent work. This keeps disposable files out of the
repository, but system cleanup can remove unpromoted recordings; review and
promote evidence before relying on its retention.

The [evidence workflow](../adapters/evidence-workflow.md) owns request,
sanitization, promotion, and replacement procedures. [Testing](../testing.md)
owns verification gates; Platform verification documents own current evidence
and pending exchange-app checks.
