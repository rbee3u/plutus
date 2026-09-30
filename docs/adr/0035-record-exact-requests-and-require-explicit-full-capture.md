---
status: superseded by ADR-0036
---

# Record exact requests and require explicit full capture

Historical decision. [ADR 0036](./0036-use-flat-interface-samples-and-single-request-recording.md) replaces the scenario corpus and batch recording/acceptance machinery; current commands and evidence rules follow that decision.

As a Platform gains endpoints, recording an entire Adapter for a small coverage gap consumes unnecessary requests and can exhaust shared IP limits. Account validation, repeated configuration reads, clock synchronization, retry and automatic pagination also made ADR 0034's `--only` recordings broader than their names implied. This supersedes that decision's default recording trigger and interface-sample behavior; its evidence, privacy, offline testing and freezing rules remain accepted.

The normal recording operation selects allowlisted read-only requests with `--only`. Resolve the complete selection before reading credentials or sending requests. Each selection sends one request with explicit business parameters, using native routing and signing. It performs no implicit Validate/Collect, configuration or key checks, clock synchronization, retry, redirect, preceding or subsequent page, price read, product read or aggregate check. An unexpected or repeated request is blocked before networking, and its failure cannot be swallowed into success. A failed selection stops the remaining work and preserves safe evidence. Kraken's one logical WebSocket snapshot includes the protocol's required challenge and subscription, but no REST preflight or second subscription.

`--query` and `--form` select literal page/cursor and other business values for one request. Public requests need no keys; Kraken private requests need only their service's key group. `--list-requests` and `--dry-run` work offline. Refusal probes remain an explicit additional operation behind `--include-refusals`.

A prepared scenario is a declared scope, separate from observed response provenance. If the selected response does not disclose Account Structure or Margin Mode, record that observation as unknown rather than fetching configuration or presenting the declaration as verified. Save the declaration and exact request plan with the batch. Owners can explicitly select configuration evidence when needed. Sample acceptance checks safe response capture, the Platform envelope and exact offline replay; it does not claim full Account Report, ownership, nonempty-product or aggregate acceptance.

Complete collection acceptance is a separate, optional operation requiring `--full` and a collection scenario. It follows production Validate/Collect, pagination, diagnostics and closing validation with a coherent window, as ADR 0034 requires. It cannot be combined with endpoint selectors. Full collection is useful for integration changes; it is not a prerequisite for every reviewed interface supplement. The trade-off is explicit: targeted evidence uses fewer requests and provides narrower guarantees; complete collection costs more requests and provides broader guarantees. Neither operation automatically promotes or freezes evidence.
