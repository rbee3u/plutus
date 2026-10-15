---
status: accepted
---

# Independent synthetic tests and recorded evidence

Keep automated adapter fixtures independent from owner-reviewed recorded
responses. Tests never read recordings. Documented support, observed response
coverage, and exchange-app reconciliation are distinct claims; none substitutes
for the others.

Ordinary verification is network-guarded and uses loopback replay, fake data,
and temporary MySQL. Only explicit single-interface recording contacts exchanges
for development evidence, using separate credentials and sanitized output under
`/tmp/plutus/`. Accepted evidence is frozen; replacement requires a coverage
gap, interface change, or error fix.

Synthetic scenarios give stable assertions without requiring matching real
holdings. Independent evidence exposes undocumented contracts, at the cost of
explicit review and separate regressions rather than replaying every recording.
Temporary artifacts stay outside the checkout in unique task directories; system
cleanup can remove unpromoted recordings, so review and promotion are required
before relying on retention.

[Evidence workflow](../platform/evidence-workflow.md) owns recording and review;
[testing](../testing.md) owns gates. Platform verification notes own coverage
and pending exchange-app checks. Deterministic [dev scenarios](../seeds.md)
exercise real valuation without establishing live integration coverage.
