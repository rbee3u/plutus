---
status: accepted
---

# Test Platform adapters in three layers and record fixtures offline

Live Platform adapters cannot be verified by the deterministic fake, and real exchange calls cannot run inside `make test`: they need keys, network, and produce different numbers every minute. Adapter verification is therefore split into three layers, each answering one question.

| Layer | Question | Network | Runs in | Data |
| --- | --- | --- | --- | --- |
| Fixture replay | Do signing, field mapping, and error classification match the Platform's real responses? | none; `httptest` serves committed JSON | `make test` | `internal/adapters/<platform>/testdata/` |
| Fake adapter | Are the collection and valuation rules right once an Account Report exists? | none | `make test` | the deterministic `fake` |
| Live smoke | Do the real endpoint, real signature, and real read-only key work end to end? | yes | `make test-live` only | `PLUTUS_GOTEST_<PLATFORM>_APIKEY`, `_SECRET`, `_PASSPHRASE` |

- Live tests carry the `live` build tag, so `make test` never compiles them and never reaches the network. `make test-live` skips a Platform whose variables are missing and exits zero. Live assertions cover structure and success only, never specific numbers.
- Every database-backed test package creates its own `plutus_gotest_<unix>_<hex>` database from a server-level `PLUTUS_GOTEST_DSN` in `TestMain`, migrates it, and drops it afterwards. Nothing is created by hand, migration edits never leave stale tables behind, and packages run in parallel without sharing state. Databases older than an hour, left by killed runs, are swept on the next run.
- Fixtures are recorded, not hand-written, because documentation and real responses diverge in exactly the places adapters get wrong. Recording happens only through a dedicated CLI subcommand (`plutus adapter record`), never through `serve` or the worker: the rule that raw Platform payloads are never persisted applies to the runtime, and the recorder is not part of the runtime.
- The recorder writes to gitignored `tmp/`, trims list endpoints (exchange info, tickers) to a per-endpoint whitelist of a few symbols so files stay in the tens of kilobytes, and redacts identifiers. The owner reviews every fixture for user IDs, account names, real amounts, and key echoes before it is committed. Each fixture starts with a header naming the Platform, endpoint, Account Structure, Margin Mode where the structure has one, and recording date. A documentation-derived scenario ([ADR 0027](./0027-allow-documented-adapter-scenarios-without-live-coverage.md)) names its documentation instead of a recording date.
- The recorder also provokes the Platform's authentication refusals (an unknown key, a wrong signature, a stale timestamp or nonce) with deliberately invalid requests to a read-only endpoint, so error classification replays real refusal answers. Each is named by its scenario, such as `invalid_key`; a successful answer is `recorded`. The recorder runs once per recording, because repeated refusals can trigger a Platform's temporary lockout.
- The keys in `gotest.env` are the test environment's read-only set, never prod's.

## Consequences

- Delivering an adapter change means: the four make gates, a mandatory `make test-live` run (or an explicit statement of which Platform could not be reached and why), an agent-lane walkthrough only when the change reaches outside the adapter package, `make dist`, and a report naming fixture provenance, live result, lane coverage, and gaps.
- Human comparison of numbers against the exchange apps happens on the test environment, guided by a checklist in `docs/adapters/<platform>.md`.
- The `fake_adapter` switch stays boolean: `true` registers the fake for every offered Platform, `false` registers every implemented live adapter and leaves the rest unavailable.
