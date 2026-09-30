# Flat Adapter interface samples

[ADR 0036](../adr/0036-use-flat-interface-samples-and-single-request-recording.md) defines this library and recording boundary. Each Platform's `testdata/` contains only flat JSON response files. Go tests explicitly choose files and match service, method, path and business query/form parameters. A required file missing or a request without a response fails; there is no source or account fallback. Pagination and state changes use explicit responses, with expected steps checked for consumption.

The library verifies response contracts, parsing, mapping and failure isolation. It may combine samples from different dates; it does not prove a single-window real asset total. All current committed samples are **documented**. Previous recordings were removed, never relabeled or copied into the new library. Existing `tmp/` history is unchanged. Inline test responses are synthetic evidence. No new-standard recording or live acceptance is claimed.

## Names

Use the path stem, replacing `/` and `-` with `_`: `/api/v3/account` becomes `api_v3_account.json`. A semantic variant identifies necessary differences, such as `api_v5_account_balance.mcm.json`, `api_v5_account_balance.pm.json` or `api_v5_public_instruments.swap.json`. Do not include dates, capture IDs, default page sizes or `.doc`. A single real response should cover coexisting positions/products; never splice real bodies to fabricate coverage. A mode suffix describes the selected response, not a recorder operation that switches account configuration.

## JSON contract

| Field | Requirement |
| --- | --- |
| `source` | Required: `recorded`, `documented`, `synthetic` |
| `request` | Required `service`, `method`, `path`; nonempty `query`/`form` only |
| `status` | Real HTTP status; omitted for WS snapshots |
| `body` | Required response body, preserving types and numeric precision |
| `recordedAt` | Recorded only: actual UTC RFC 3339 recording time |
| `documentation` | Documented only: corresponding official documentation URL |
| `notes` | Optional necessary sanitation, trimming and construction explanation |
| `bodyFormat` | `"text"` for non-JSON text, with a string body; otherwise omitted |

One parameter value is a string; multiple values are a string array. Cursors, pages and exact business values remain; keys, signatures, timestamps, nonce and other authentication parameters do not. Decode HTTP query/form encoding once. The shared validator rejects legacy metadata, invalid sources, parameter types and protocol combinations. Small exceptional responses are constructed in tests; larger synthetic responses may have their own flat files.

## Record exactly one interface

Run from the repository root. Listing and dry-run require neither credentials nor network:

```sh
go run ./cmd/plutus adapter record --platform okx --list-requests
go run ./cmd/plutus adapter record --platform okx \
  --only api_v5_account_balance --variant pm --dry-run
go run ./cmd/plutus adapter record --platform okx \
  --only api_v5_account_balance --variant pm
go run ./cmd/plutus adapter record --platform okx \
  --only api_v5_public_instruments --query instType=SWAP --variant swap --dry-run
go run ./cmd/plutus adapter record --platform binance \
  --only sapi_v1_dci_product_positions --query status=PURCHASE_SUCCESS \
  --variant purchase_success_call --dry-run
go run ./cmd/plutus adapter record --platform kraken \
  --only 0_private_Earn_Strategies --form cursor=next%2Fpage --dry-run
```

`--only` selects exactly one allowlisted read-only interface. Identifiers exclude parameter values. `--query`/`--form` override native defaults; repeat a key for multiple values. Missing required parameters fail before keys or network. Bybit positions require `category`; linear positions also need `settleCoin` or `symbol`. OKX instruments require `instType`; Binance Dual positions require `status`. Kraken signed Spot parameters use `--form`. Public interfaces read no keys; Kraken reads only the selected service's credential group. Copy `config/gotest-example.env` to gitignored `config/gotest.env` for test-environment read-only credentials.

The recorder sends one HTTP request with native routing/signing and the local clock. It never adds validation, time synchronization, prices, products, totals, another page, retries, redirects or refusal probes. Clock drift or refusal returns a failed response; correct the local setup before another explicit command. Kraken WS uses one challenge/subscription/snapshot and no REST. Removed commands/flags: `adapter verify`, `--scenario`, `--full`, `--include-refusals` and scenario listing.

## Save, review and freeze

One command creates a unique directory under gitignored `tmp/fixtures/<platform>/` and publishes one JSON atomically without replacing a file. `--out` changes the parent only within `tmp/`. No batch manifest, summary or complete collection result is produced. Sanitation masks customer references and secrets; preserves business values, product identifiers, prices, terms and times; and trims market responses to the BTC/ETH/USD whitelist, explicitly requested symbols and representative contract types. Notes disclose trimming. Private account/product responses are not trimmed. A redaction failure produces no file.

Terminal output separates the saved path, HTTP or WS response, Platform success and empty state. Request, Platform response or write failures return nonzero. Safe failure responses can be retained for offline diagnosis; successful writing never means business mapping acceptance. Empty responses remain in `tmp/` by default and do not replace a populated sample.

Prepare only the selected interface's required account state. Review the response and its offline test, then promote that exact file and update the Platform inventory with evidence and the replacement reason. Freeze reviewed samples; update only for a coverage gap, interface change or error fix. Real access, account ownership and aggregate totals require separate evidence and are not inferred from a mapped sample. New recordings proceed Binance → OKX → Bybit → Kraken.

## Verification

`make test` runs local replay, the deterministic fake and throwaway MySQL databases. Exchange HTTP/HTTPS/WS, proxy and redirect attempts fail independently of Account Report error handling. `make test-live` is removed. Runtime collection remains live in test/prod. Deliver code through `make format && make build && make test && make lint`, then `make dist`. API/UI changes additionally require the agent lane; adapter sample/recorder-only changes do not.
