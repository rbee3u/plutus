---
status: superseded by ADR-0036
---

# Freeze scenario fixtures and accept recordings offline

Historical decision. [ADR 0036](./0036-use-flat-interface-samples-and-single-request-recording.md) replaces the scenario corpus and batch recording/acceptance machinery; current commands and evidence rules follow that decision.

The default recording trigger and interface-sample behavior are superseded by [ADR 0035](./0035-record-exact-requests-and-require-explicit-full-capture.md). The remaining evidence, full collection acceptance, privacy, offline verification and freezing rules stay accepted.

Account Structures, Margin Modes, and product states can change response shapes. A single recorded baseline cannot prove their coverage. Use explicit scenario manifests, shared responses with declared applicability, and separate recorded, documented, and synthetic evidence. Preserve old dates and values; historical combinations are interface samples, not complete Collection Runs. This replaces ADR 0023's online smoke layer, recorder behavior, and adapter delivery gates; its fake-adapter and temporary-database rules remain. ADR 0027's evidence distinction remains accepted.

Only explicit `plutus adapter record --scenario <id>` may contact Platforms during development verification. It follows production Validate/Collect requests and pagination, saves each occurrence by service, method, path, business query/form and phase, and checks structure, Margin Mode, credential ownership, reports, required holdings and existing aggregate tolerances. Refusal probes require `--include-refusals`. Normal runtime collection remains live. Remove `make test-live`; ordinary tests fail on forbidden exchange connections, even when an Adapter would classify them as failed reports.

Capture into fresh, staged `tmp/` batches. Sanitize identities and secrets before any response is written; preserve business values and terms. Mapping or acceptance failures retain safe evidence with a failed status and nonzero exit. Incomplete disk writes cannot be read as completed batches. `adapter verify` replays a saved batch entirely offline with synthetic credentials, requires all phase responses to be consumed, and never rewrites its capture status or grants owner acceptance.

Owner-reviewed accepted evidence is frozen. Replace a specified fixture only for a coverage gap, interface change, or error fix, with the reason recorded. Full collection evidence needs a single capture window, including products, prices and aggregates; Platform responses are not an atomic snapshot. Smaller-page samples remain interface evidence. Each delivery lists recorded nonempty and empty coverage separately from documentation, synthesis and missing evidence, runs the four make gates and `make dist`, and exercises the agent lane when runtime/API/UI scope requires it. Migrate and acquire missing evidence in Binance → OKX → Bybit → Kraken order without expanding product scope.
