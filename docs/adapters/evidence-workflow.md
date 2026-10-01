# Adapter evidence workflow

Use this procedure for explicit recording, review, and promotion of sanitized
Platform responses. Automated test requirements belong in
[testing](../testing.md); the reasons for separating tests and evidence are in
[the verification decision](../adr/0009-adapter-verification.md).

Each Platform has two independent flat directories: `evidence/` holds reviewed
recorded responses, and `testdata/` holds constructed synthetic test inputs.
Tests never load evidence, files need not correspond one-to-one, and new
evidence does not regenerate mocks. Current evidence, official contracts,
synthetic inputs, remaining gaps, and exchange-app checklists are maintained in
the Platform verification documents: [Binance](./binance/verification.md),
[OKX](./okx/verification.md), [Bybit](./bybit/verification.md), and
[Kraken](./kraken/verification.md).

## Names and response envelope

Use the path stem, replacing `/` and `-` with `_`: `/api/v3/account` becomes
`api_v3_account.json`. Evidence adds a semantic variant only for necessary
request or response differences, such as `.pm`, `.mcm` or `.swap`. Do not
include recording dates, batches, default page sizes or `.doc`. Coexisting
products/positions share one unspliced real response. Synthetic tests may split
that contract into `.call`, `.put`, `.both`, failure or pagination examples; the
layers need not share variants. Small test responses can remain inline.

| Field        | Evidence                                             | Synthetic tests                                  |
| ------------ | ---------------------------------------------------- | ------------------------------------------------ |
| `source`     | Required `recorded`                                  | Required `synthetic`                             |
| `request`    | Actual service, method, path and nonempty query/form | Explicit expected business request               |
| `status`     | Actual HTTP status; omitted for WS snapshot          | Constructed HTTP status; omitted for WS snapshot |
| `body`       | Required sanitized response                          | Required constructed response                    |
| `recordedAt` | Required actual UTC RFC 3339 time                    | Forbidden                                        |
| `notes`      | Optional necessary redaction/trimming explanation    | Optional construction explanation                |
| `bodyFormat` | `"text"` for non-JSON text with a string body        | Same rule                                        |

A single query/form value is a string; multiple values are a nonempty string
array. Preserve business values, pagination and literal cursor strings without a
second decode. Exclude signatures, authentication timestamps, nonce and secrets.
A WS request names its logical subscription (`method: "WS"`), not an HTTP
success status. Legacy wrappers, `documented` source, embedded documentation
URLs, source dates on synthetic files and invalid field/protocol combinations
fail shared validation. Test loaders and response sequences additionally reject
`recorded` inputs.

## Record one interface, prepare a group

The recorder is the only development-verification command that contacts
exchanges. Runtime `serve`, worker and `collect` retain real collection. Listing
and dry-run need no credentials or network:

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

`--only` selects one allowlisted read-only interface. `--query`/`--form`
override native defaults literally; repeat a key for multiple values. Required
parameters fail before keys/network. `--variant` affects the filename only and
neither switches nor verifies account settings. Repeat `--keep-symbol` for
prepared instruments that must survive local market trimming; these hints are
never sent to the exchange. Public requests read no keys; private requests read
only their selected `PLUTUS_RECORD_*` credential group from the process
environment or gitignored `config/record.env` (copy
[record-example.env](../../config/record-example.env)). Kraken Spot/Futures use
separate groups. Recording never falls back to `gotest.env` or old
`PLUTUS_GOTEST_*` keys; ordinary tests never load recording credentials
([Environment decision](../adr/0008-environments-and-migrations.md)).

Each invocation makes exactly one HTTP request without configuration, clock
synchronization, prices, products, totals, extra pages, retries, redirects or
refusal probes. Kraken WS permits the necessary challenge/subscription for one
logical snapshot without REST. An owner prepares one compatible group of
holdings/settings, reviews its exact request/parameter list, and authorizes the
sequential commands together. Stop on the first request/write failure; keep
successes and resume only the uncompleted commands after correction. There is no
full sweep or new batch engine.

## Trim, save and review

Market lists retain prepared/requested symbols and representatives of product
kind, settlement, quote, contract unit and field/type shape. BTC/ETH
representatives are preferred where available, not mandatory whitelisted coins.
Lists retain original order, whole rows and original aggregate/count fields. A
missing prepared symbol is reported during evidence review rather than
fabricated or fetched from another interface.

Only explicitly recognized balance/position interfaces trim zero rows. Known
financial fields (including wallet/position amount, debt, interest, frozen
funds, margin and PnL) must all be zero before a row can be redundant. Keep one
representative per field/type shape and meaningful position/margin/collateral
setting. Any nonzero financial measure, unknown field/type, null, empty string
or special state keeps the row. Product and configuration lists are not pruned
by this rule. Notes disclose original and retained counts and the selection
rule. Retained business values, precision, product terms, times and totals are
not rewritten.

Each command creates a unique directory under gitignored
`tmp/evidence/<platform>/`, atomically publishing one JSON without overwrites.
`--out` may change the parent only inside `tmp/`. No complete collection, batch
manifest or asset-total proof is generated. Identity fields and secrets are
masked; unsafe redaction leaves no file. Request/response/write failures return
nonzero even when a safe failed response is retained. Terminal output separately
reports the saved path, HTTP/WS result, Platform success and empty state. Saving
does not prove correct business mapping. Empty and failed responses remain in
`tmp/` by default; there is no automatic promotion.

Review request identity, sanitation, critical response shapes and differences
from official documentation; then the owner reviews before promoting the exact
response to `evidence/`. Full Adapter replay is not an evidence gate. If new
evidence exposes a parsing problem, fix it separately with an independently
constructed regression response. Update the inventory with the actual date,
observed or owner-declared settings, useful differences and remaining gaps. Do
not infer generic product applicability from the account used to record it.
Freeze reviewed evidence; replacements require a coverage-gap, interface-change
or error-fix reason. Unavailable critical real states remain gaps while other
work proceeds.

## Independent synthetic regression tests

When evidence exposes a parsing issue, construct an independent synthetic
regression input. Tests choose explicit responses and expected request steps; a
response directory is not a complete call sequence. Preserve meaningful
nonempty, mode, pagination, failure, storage, and valuation assertions. Do not
require artificial real states simply to reproduce a test scene.

Run the applicable [verification gates](../testing.md#validation-gates) and
[Adapter delivery checks](../testing.md#adapter-delivery-reports). Real evidence
acceptance does not establish exchange-app reconciliation or a complete
integration.
