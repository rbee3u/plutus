# Runtime proxy delivery verification

The original delivery below was verified on 2026-10-02. Its environment proxy
behavior is superseded by the
[2026-10-08 configuration revision](#configuration-source-revision-2026-10-08).

Verified on 2026-10-02. This delivery adds instance-wide runtime HTTP/SOCKS
proxy configuration for Binance, OKX, Bybit, and Kraken. It does not implement
Shadowsocks or establish successful access through a real proxy exit. See
[configuration](../development.md#platform-proxies) and
[delivery requirements](../testing.md#adapter-delivery-reports).

## Recorded interface evidence

No new responses were recorded for this transport change. Existing reviewed
interface evidence remains unchanged and does not establish proxy routing,
allowlisted exit IPs, or exchange-app reconciliation.

| Platform | Existing recorded evidence                                                                               | Proxy evidence gap                |
| -------- | -------------------------------------------------------------------------------------------------------- | --------------------------------- |
| Binance  | [34 responses](./binance/verification.md), recorded 2026-09-30 UTC and reviewed 2026-10-01 Asia/Shanghai | No real proxy or SS exit verified |
| OKX      | [20 responses](./okx/verification.md), recorded and reviewed 2026-10-01                                  | No real proxy or SS exit verified |
| Bybit    | [19 responses](./bybit/verification.md), recorded and reviewed 2026-10-01                                | No real proxy or SS exit verified |
| Kraken   | [11 responses](./kraken/verification.md), recorded and reviewed 2026-10-01                               | No real proxy or SS exit verified |

The linked Platform notes retain their product, response-shape, and exchange-app
validation gaps. No integration status was upgraded by this work.

## Independent synthetic tests and builds

The full validation sequence passed in order:
`make format && make web && make build && make test && make lint`. All Go
packages and seven frontend tests passed; golangci-lint reported zero issues.
The Linux amd64 static build also passed.

| Platform | Synthetic verification                                                                                                                                                                                                                                                           |
| -------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Binance  | Existing suite passed; factory receives the 10-second budget; explicit HTTP client wins; runtime validation reaches a local CONNECT proxy and normalizes refusal as temporary                                                                                                    |
| OKX      | Existing suite passed; factory receives the 10-second budget; explicit HTTP client wins; runtime validation reaches a local CONNECT proxy and normalizes refusal as temporary                                                                                                    |
| Bybit    | Existing suite passed; factory receives the 15-second budget; explicit HTTP client wins; runtime validation reaches a local CONNECT proxy and normalizes refusal as temporary                                                                                                    |
| Kraken   | Existing suite passed; factory receives the 15-second budget; explicit HTTP client wins; runtime validation reaches a local CONNECT proxy and normalizes refusal as temporary; authenticated synthetic balances WebSocket succeeds through CONNECT with certificate verification |

Shared loopback tests cover HTTP and HTTPS CONNECT, SOCKS5 and SOCKS5H,
proxy-side hostname forwarding, explicit proxy routing despite `NO_PROXY=*`,
direct routing despite configured environment proxies, unavailable proxy with no
direct fallback, and cancellation during a stalled CONNECT handshake.
Environment proxy and bypass tests use isolated processes to avoid Go's cached
proxy environment. Configuration tests cover defaults, TOML/env-file/process
precedence, modes, malformed URLs, unsupported schemes, forbidden credentials,
and errors that do not reveal rejected URL credentials.

The runtime wiring tests install the offline network guard before constructing
Adapters with production hosts. Proxy helpers only forward explicitly expected
targets to fixed loopback origins. Tests use synthetic `testdata`, not recorded
`evidence`, external requests, or live credentials. Recording list/dry-run tests
confirm that runtime proxy configuration and runtime config file flags do not
change recording's independent lifecycle.

## Agent runtime coverage

The final host build was exercised on `127.0.0.1:8081` against `plutus_agent`
with the walkthrough seed and the deterministic fake. The process received
`proxy` mode with an unavailable loopback entry point; startup, readiness, and
fake collection remained independent of proxy availability. Temporary runtime
configuration and logs stayed under `/tmp/plutus/`; the normal agent config
files were not changed.

Verified login, Portfolio reads, 12 Account valuations, the holdings tree, 25
Balance rows, eight Position rows, five Dual Position rows, populated raw
history with the default seven-day window, point detail, default daily closes
and Value Changes, and empty history for the unvalued trading Portfolio. The
embedded SPA and its JavaScript asset were served by :8081. A manual Collection
Cycle succeeded and produced the point returned by the overview. All
session-started server processes were stopped, and :8081 was released. The
human's :8080 instance was not used.

## Uncovered behavior

No real Shadowsocks server, sing-box process, remote server migration, actual
exit IPv4, exchange IP allowlist, or live credential was exercised. No live
latency or long-running proxy reliability claim is made. The agent fake does not
establish real Adapter collection through a proxy. The deployment owner must
separately verify the exit IP and successful Account Reports; readiness alone
does not establish either. UI interaction was not changed or exercised; this
runtime configuration change was verified through the JSON API and embedded
asset responses.

## Configuration source revision (2026-10-08)

This revision supersedes the environment proxy and ordinary environment override
behavior described in the original delivery above. Ordinary settings now use
TOML and defaults only; ignored environment names cannot override either source.
Both runtime and recording default to direct access. Explicit proxies use TOML,
and the removed `environment` mode fails with migration guidance. Recording uses
its own optional `config/record.toml` rather than runtime configuration.

### Synthetic verification and builds

The ordered gates passed: `make format`, `make web`, `make build`, `make test`,
and `make lint`. All Go packages passed, all 150 frontend tests in 18 files
passed, and golangci-lint reported zero issues. The Linux amd64 build passed.

Configuration tests cover ignored ordinary variables from both process and env
files, including empty/invalid values and previously removed names, while
preserving TOML validation, TLS checks, and secret precedence/isolation.
Recording tests cover optional/default and required/explicit paths, unknown
fields, invalid proxy policies, and configuration failures before credentials.
Listing and dry-run still skip configuration and credentials.

For each of Binance, OKX, Bybit, and Kraken, synthetic CLI recording requests
reached a loopback CONNECT proxy selected from both the default and explicit
recording TOML paths. Invalid runtime and credential files were not loaded for
these public requests. The existing runtime Adapter routing and Kraken WebSocket
CONNECT tests passed. Shared transport tests verify direct defaults without
inherited proxy callbacks, upper/lowercase proxy variables having no effect,
explicit HTTP/HTTPS/SOCKS routing despite `NO_PROXY`, cancellation, and proxy
failure without direct fallback. No external requests or live keys were used.

### Agent runtime and remaining gaps

The host binary ran against existing seeded `plutus_agent` on `127.0.0.1:8081`
with the deterministic fake and no proxy section. Invalid ordinary configuration
variables and unavailable process proxy URLs did not prevent startup or alter
the TOML listener/cookie settings. Liveness, readiness, the embedded SPA and its
assets, seed login/logout, Portfolio listing, Overview, Tree, Balances, the
default History window, and paginated Valuation Points all succeeded.

Verification used a temporary copy of agent TOML with logs under `/tmp/plutus`;
agent configuration and the human's :8080 instance were unchanged. The server
process group stopped, :8081 was released, and temporary files were removed.

No new exchange responses were recorded. Existing evidence and exchange-app
validation status are unchanged. This revision does not establish real proxy
exit routing, allowlisted IPs, live recording, or long-running reliability.
