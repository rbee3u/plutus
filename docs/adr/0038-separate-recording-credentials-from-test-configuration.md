---
status: accepted
---

# Separate recording credentials from test configuration

Sharing gotest.env between the database test harness and the recorder made an offline test configuration carry live Platform secrets. Keep config/gotest.env for PLUTUS_GOTEST_* test settings (currently the throwaway MySQL DSN), and move explicit single-interface recording to gitignored config/record.env with PLUTUS_RECORD_<PLATFORM>_* credentials, including separate Kraken Spot/Futures pairs. Only adapter record loads that namespace/file; public requests, request listings and dry runs need no credentials. Each lookup searches only its own file within the Go module, with process variables taking precedence, and ordinary Adapter tests use independent local synthetic testdata and the network guard.

Do not fall back to the old PLUTUS_GOTEST_* Platform keys, gotest.env or runtime env files when recording keys are missing. This requires a one-time local credential migration instead of preserving a convenient but ambiguous shared source. Recording may use the remote test environment's read-only key set; prod keys remain separate and never appear on a development machine. Runtime serve/worker/collect credentials and temporary database behavior are unchanged.

This supersedes only the recording-key storage rule in [ADR 0022](./0022-separate-dev-test-and-prod-environments.md), updates the recording secret variable named in [ADR 0031](./0031-accept-binance-ed25519-private-keys.md), and complements [ADR 0037](./0037-separate-recorded-evidence-from-synthetic-tests.md). It does not create another runtime environment or change either response-data layer.
