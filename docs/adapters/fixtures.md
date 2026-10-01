# Recorded evidence and synthetic Adapter tests

[ADR 0037](../adr/0037-separate-recorded-evidence-from-synthetic-tests.md) separates each Platform's flat `evidence/` and `testdata/` directories. Evidence records observed exchange responses to check official documentation; tests construct deliberate business scenes from those two references. No test loads evidence, no generation/dependency manifest connects the files, and updating evidence does not regenerate test data. Empty evidence directories are created when first needed and contain no placeholder response files.

The previous recordings and their `tmp/fixtures`/`tmp/offline` scratch directories have been removed. All committed test responses are **synthetic**; Binance has 34 reviewed responses covering PM One-way/Hedge, shared wallets/products, Classic Single-Asset One-way/Hedge and Classic Multi-Assets under this standard, and OKX has 20 reviewed Spot, Futures, MCM, PM and shared wallet/product/market responses recorded on 2026-10-01. Borrowing-disabled Spot evidence is owner-deferred because no separate disable control was found; Bybit has 19 reviewed PM One-way, Cross One-way/Hedge and shared wallet/product/market responses recorded on 2026-10-01, with Isolated recording owner-deferred because it is rarely used and nonempty Fixed-Term Saving/SellHigh owner-deferred for principal/product requirements. These real gaps retain documented support and independent synthetic coverage. Kraken has 11 owner-reviewed Unified wallet, position, Earn, credential and market responses recorded on 2026-10-01. Separate wallets are unavailable and the owner defers further account preparation; nonzero Auto Earn, inverse delivery positions and other unobserved states remain explicit gaps. Other unobserved Binance/OKX states also remain pending. Passing a synthetic test is not real coverage or a real asset-total reconciliation. Each Platform inventory links official documentation, test files and outstanding evidence: [Binance](./binance-fixtures.md), [OKX](./okx-fixtures.md), [Bybit](./bybit-fixtures.md), [Kraken](./kraken-fixtures.md).

## Names and response envelope

Use the path stem, replacing `/` and `-` with `_`: `/api/v3/account` becomes `api_v3_account.json`. Evidence adds a semantic variant only for necessary request or response differences, such as `.pm`, `.mcm` or `.swap`. Do not include recording dates, batches, default page sizes or `.doc`. Coexisting products/positions share one unspliced real response. Synthetic tests may split that contract into `.call`, `.put`, `.both`, failure or pagination examples; the layers need not share variants. Small test responses can remain inline.

| Field | Evidence | Synthetic tests |
| --- | --- | --- |
| `source` | Required `recorded` | Required `synthetic` |
| `request` | Actual service, method, path and nonempty query/form | Explicit expected business request |
| `status` | Actual HTTP status; omitted for WS snapshot | Constructed HTTP status; omitted for WS snapshot |
| `body` | Required sanitized response | Required constructed response |
| `recordedAt` | Required actual UTC RFC 3339 time | Forbidden |
| `notes` | Optional necessary redaction/trimming explanation | Optional construction explanation |
| `bodyFormat` | `"text"` for non-JSON text with a string body | Same rule |

A single query/form value is a string; multiple values are a nonempty string array. Preserve business values, pagination and literal cursor strings without a second decode. Exclude signatures, authentication timestamps, nonce and secrets. A WS request names its logical subscription (`method: "WS"`), not an HTTP success status. Legacy wrappers, `documented` source, embedded documentation URLs, source dates on synthetic files and invalid field/protocol combinations fail shared validation. Test loaders and response sequences additionally reject `recorded` inputs.

## Record one interface, prepare a group

The recorder is the only development-verification command that contacts exchanges. Runtime `serve`, worker and `collect` retain real collection. Listing and dry-run need no credentials or network:

```sh
plutus adapter record --platform binance --list-requests
plutus adapter record --platform okx --only api_v5_account_balance --variant pm --dry-run
plutus adapter record --platform okx --only api_v5_public_instruments \
  --query instType=SWAP --variant swap --keep-symbol BTC-USDT-SWAP --dry-run
plutus adapter record --platform binance --only sapi_v1_dci_product_positions \
  --query status=PURCHASE_SUCCESS --dry-run
plutus adapter record --platform kraken --only 0_private_Earn_Strategies \
  --form cursor=next%2Fpage --dry-run
```

`--only` selects one allowlisted read-only interface. `--query`/`--form` override native defaults literally; repeat a key for multiple values. Required parameters fail before keys/network. `--variant` affects the filename only and neither switches nor verifies account settings. Repeat `--keep-symbol` for prepared instruments that must survive local market trimming; these hints are never sent to the exchange. Public requests read no keys; private requests read only their selected `PLUTUS_RECORD_*` credential group from the process environment or gitignored `config/record.env` (copy [record-example.env](../../config/record-example.env)). Kraken Spot/Futures use separate groups. Recording never falls back to `gotest.env` or old `PLUTUS_GOTEST_*` keys; ordinary tests never load recording credentials ([ADR 0038](../adr/0038-separate-recording-credentials-from-test-configuration.md)).

Each invocation makes exactly one HTTP request without configuration, clock synchronization, prices, products, totals, extra pages, retries, redirects or refusal probes. Kraken WS permits the necessary challenge/subscription for one logical snapshot without REST. An owner prepares one compatible group of holdings/settings, reviews its exact request/parameter list, and authorizes the sequential commands together. Stop on the first request/write failure; keep successes and resume only the uncompleted commands after correction. There is no full sweep or new batch engine.

## Trim, save and review

Market lists retain prepared/requested symbols and representatives of product kind, settlement, quote, contract unit and field/type shape. BTC/ETH representatives are preferred where available, not mandatory whitelisted coins. Lists retain original order, whole rows and original aggregate/count fields. A missing prepared symbol is reported during evidence review rather than fabricated or fetched from another interface.

Only explicitly recognized balance/position interfaces trim zero rows. Known financial fields (including wallet/position amount, debt, interest, frozen funds, margin and PnL) must all be zero before a row can be redundant. Keep one representative per field/type shape and meaningful position/margin/collateral setting. Any nonzero financial measure, unknown field/type, null, empty string or special state keeps the row. Product and configuration lists are not pruned by this rule. Notes disclose original and retained counts and the selection rule. Retained business values, precision, product terms, times and totals are not rewritten.

Each command creates a unique directory under gitignored `tmp/evidence/<platform>/`, atomically publishing one JSON without overwrites. `--out` may change the parent only inside `tmp/`. No complete collection, batch manifest or asset-total proof is generated. Identity fields and secrets are masked; unsafe redaction leaves no file. Request/response/write failures return nonzero even when a safe failed response is retained. Terminal output separately reports the saved path, HTTP/WS result, Platform success and empty state. Saving does not prove correct business mapping. Empty and failed responses remain in `tmp/` by default; there is no automatic promotion.

Review request identity, sanitation, critical response shapes and differences from official documentation; then the owner reviews before promoting the exact response to `evidence/`. Full Adapter replay is not an evidence gate. If new evidence exposes a parsing problem, fix it separately with an independently constructed regression response. Update the inventory with the actual date, observed or owner-declared settings, useful differences and remaining gaps. Do not infer generic product applicability from the account used to record it. Freeze reviewed evidence; replacements require a coverage-gap, interface-change or error-fix reason. Unavailable critical real states remain gaps while other work proceeds.

## Offline verification

All four Platforms' tests explicitly choose synthetic files and match service, method, path and business parameters. Missing files and unknown requests fail; explicit pagination/repeated-response sequences must be consumed. A response library is not itself an expected call sequence: individual tests declare their required steps. Synthetic amounts, identifiers, prices and aggregates are chosen together for stable success or deliberate failure, preserving useful nonempty/mode/error/completeness/storage/valuation assertions.

`make test` includes the deterministic fake, loopback HTTP/WS replay and throwaway MySQL. The network guard blocks exchange HTTP/HTTPS/WS, proxy and redirect attempts even when an Adapter converts the error into an Account Report. `make test-live` remains removed. Code deliveries pass ordered `make format && make build && make test && make lint`, then `make dist`; runtime/API/UI changes additionally require the agent lane. New evidence is acquired in Binance → OKX → Bybit → Kraken order, minimizing setting changes and never multiplying all modes, instruments and directions into a Cartesian matrix.
