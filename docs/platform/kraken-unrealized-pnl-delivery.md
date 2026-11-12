# Kraken unrealized PnL delivery: 2026-11-05

This delivery lets Kraken balances report the wallet's unrealized result, the
same way Binance's margin balance and OKX's equity already include it. No
exchange integration is declared complete, and the exchange-app checklist
remains pending.

## Financial behavior

The multi-collateral wallet (main in a unified wallet, Multi-M in separate
wallets) reports its `total_unrealized` on the USD row: that row's `Equity` is
the collateral quantity plus the result and its `Unrealized PnL` is the result.
Its other collateral currencies report `0`, because only flexible linear
contracts belong to that wallet and they settle in USD. A settlement row is
created when the result is nonzero and the wallet holds no USD quantity.

Each Coin-M wallet's `portfolio_value` already holds its unrealized result, so
its row does not add it again; the row reports `pnl + unrealized_funding`, which
makes `Equity = balance + Unrealized PnL` hold without a hidden funding
residual.

Main in separate wallets adds `TradeBalance.n` to its USD row and stops writing
`spot_margin_unrealized_pnl_usd`. Reports collected before this mapping keep
that measure and a USD row without the result, and valuation adds either shape
exactly once. The derivatives holding wallet never trades, so its rows report
`0`. Earn rows are not margin positions and keep a null result.

Reported account equity, API fields, database columns, and `CalculationVersion`
are unchanged. Removing the Spot margin account measure from new reports
rearranges the same total from an account-level adjustment into the balance row,
so no stored valuation total changes.

## Verification

### Ordered gates

- `make format && make web && make build && make test && make lint` passed. All
  31 Go packages passed, including the temporary-database integration suite;
  frontend unit tests passed in 33 files, 322 tests; `golangci-lint` reported no
  issues, and the frontend typecheck and ESLint passed.
- `make build GOOS=linux GOARCH=amd64` passed and wrote
  `dist/plutus-linux-amd64`.
- `make generate` was not run: no sqlc query, API schema, or generated binding
  source changed.

### Recorded evidence

The recorded `ws_v1_balances_snapshot` from 2026-10-01 05:56 UTC carries
`flex_futures.total_unrealized`, `pnl`, and `unrealized_funding`, and the same
`pnl` and `unrealized_funding` on every `futures` wallet, including wallets with
no position. It establishes `portfolio_value = balance_value + total_unrealized`
on one account (91.4874 − 0.233 = 91.2544) and that Kraken reports zero values
explicitly rather than omitting them.

Gaps: the capture is one account at one instant. It has no separate-wallet main,
no multi-collateral wallet without USD collateral, no Coin-M wallet with a
nonzero result, and no response that omits these fields.

### Synthetic tests

- `internal/platform/kraken` `TestUnrealizedPnlAttribution` covers a result
  without USD collateral (the settlement row is created), a reported zero that
  stays zero, and an absent result that fails with `normalization` instead of
  reading as zero.
- `TestReplayUnifiedWallet` covers USD 500 + 200 = 700 with BTC collateral `0`,
  the derivatives holding `0`, Coin-M `0`, and a null earn result.
- `TestReplaySeparateWallets` covers USD 25445.21 − 10.0232 = 25435.1868 with
  the same value as its `Unrealized PnL`, an empty
  `spot_margin_unrealized_pnl_usd`, and null results on other coins.
- `TestSeparateMainRules` covers a margin result folded into the USD row and a
  reported zero result.
- The integration sample replay keeps Kraken's reported account values (main
  1000, Coin-M 10) and a portfolio total of 1330; descriptive balance values
  changed without changing any valuation.
- The demo seed sets the column on settlement-coin balances and reports `0` on
  the other collateral coins of a margin account; `internal/seed` passes.

### Agent environment

The documented `:8081` agent instance was reset with the walkthrough seed
(`scripts/agent-wipe-and-serve.sh`) and exercised through its HTTP API after
login as the seed user:

- `GET /health/live` and the embedded SPA at `/` returned 200.
- `GET /api/v1/portfolios/{id}/balances?platform=kraken` returned the unified
  main's USD row with `unrealizedPnl 769.30386` against a −850 seeded balance,
  the collateral rows BTC, DOGE, ETH, and LINK with `0`, Multi-M's USD row with
  `151.71975`, Coin-M's BTC row with `0.0009137030567…`, the separate-wallet
  main's USD row with `12.34` and equity `912.34` against a 900 seeded balance,
  and null earn rows.
- The filtered Kraken total equalled the sum of the returned rows' `equityUsd`
  and included the unrealized results.
- The SPA was not clicked through: this session has no browser, so the read path
  was exercised with `curl`. The server processes were terminated and `:8081`
  has no listener.

### Not covered

- Live Kraken keys and exchange-app validation: checklist items 3, 5, and 8 in
  [Kraken verification](./kraken/verification.md) remain the acceptance
  procedure.
- Non-USD Spot margin results in a separate-wallet main: the adapter reads one
  USD result, and other coins keep a null unrealized result.
- A live response that omits `total_unrealized`, `pnl`, or `unrealized_funding`:
  the contract requires them, and the synthetic test covers the failure.
- Coin-M wallets whose unrealized result is nonzero and whose `pnl` and
  `unrealized_funding` signs differ: the arithmetic is covered, but no recorded
  response exercises it.
