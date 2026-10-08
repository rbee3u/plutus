# Margin presentation correction: 2026-10-03

This follow-up implements the revised
[ADR 0014](../adr/0014-position-provenance.md). It restores known Binance retail
Portfolio Margin Futures initial requirements, verifies OKX inverse source
units, and uses a single-line Margin amount with supporting details in its
disclosure. No exchange integration is declared complete.

## Financial behavior

Verified reported amounts take precedence. Supported Binance Cross futures,
including retail PM, can use the documented initial-leverage formula when no
usable source requirement is available. The API distinguishes `reported` from
`calculated` through `marginSource`; unknown amounts have a
`marginUnavailableReason`. Original margin, quantity, prices, and Report times
remain unchanged. Old Binance PM Reports use their own retained facts and the
immutable supported retail structure to establish Cross.

Isolated displays allocated funds excluding unrealized PnL, including actual
additions and deductions. No initial requirement is relabeled allocation. Bybit
source fees and hedge treatment remain in use. OKX and Bybit Portfolio risk-unit
requirements are not distributed across positions. Kraken effective leverage is
not used to estimate position collateral.

Reviewed OKX non-PM inverse `imr` values match native settlement requirements,
contrary to the REST USD description. Reads compare native and USD candidates
within relative `1e-8` rounding tolerance, requiring compatible specifications
and positive source inputs. Neither or ambiguous matches stay unknown. Old
typed-USD reads use the same validation and the source Report's saved regime;
missing historical provenance is not inferred. No database changes or new
exchange calls are required.

## Presentation

Unrealized PnL precedes Margin in aggregate, Account, and historical tables.
Margin displays only its amount and currency, aligned to the same right edge as
unknown `—`, with no reserved tag slot. It fits its longest displayed value and
does not receive extra space from automatic table layout. The amount's
hover/focus/click disclosure is a short `1.88x isolated`, `3x cross`, or
`portfolio margin` line. Cross uses reported leverage. Isolated uses absolute
USD notional divided by allocated margin valued in USD, from unrounded source
decimals; absent conversion inputs leave the mode alone. Leverage is removed
from Symbol details. Margin provenance and diagnostics remain in the API. The
amount is one keyboard target.

## Verification

### Initial margin correction

- The ordered `make format && make web && make build && make test && make lint`
  gates passed. All Go packages passed, including temporary-database historical
  and stale-read tests; frontend unit tests passed in six files, 47 tests.
  Linters reported no issues. A test expectation for inverse face-value margin
  was corrected before repeating the complete sequence.
- `make build GOOS=linux GOARCH=amd64` passed. Repeating `make generate` left
  all 21 generated Go, SQL-query, and frontend files byte-identical.
- Chromium passed all four tests. The position test covered the column order,
  hover/focus/click disclosure, Escape, stale and missing-data explanations,
  delivery timezone, small amounts, narrow-table scrolling, and calculated
  Margin provenance. Mixed `crx`, `iso`, `pm`, and unknown rows shared the
  amount right edge within 0.5 CSS pixels; an unknown Margin had no visible tag.
- The embedded SPA on :8081 used only fake data and `plutus_agent`. Actual
  navigation exercised the aggregate Futures table, expanded Binance PM account
  details, and a pre-change historical point. Binance PM amounts appeared with
  one compact `pm` tag and the calculation disclosure. All eight aggregate
  amount right edges were identical. Historical reads used their saved prices,
  leverage, and 14:55 Report time rather than the latest 17:27 Report.
- `git diff --check`, the repository Markdown formatting check, and 101 local
  Markdown path/anchor checks passed. No Git index changes were made.

Isolated and unavailable-Portfolio tag variants were verified by Chromium; they
were not present in this fake runtime walkthrough. No real exchange app was
exercised. The agent server process group was stopped, its health request failed
afterward, and :8081 and the Chromium :4173 server had no listeners.

### Compact layout follow-up

The layout-only follow-up removes inline mode tags and their width allocation,
and moves Unrealized PnL before Margin. No Adapter, API, or financial
calculation changes are included in this follow-up.

- The ordered format, web, build, test, and lint gates passed again, including
  all Go packages and the 47 frontend unit tests. All four Chromium tests
  passed.
- Chromium verified amount-only labels for Cross, Isolated, Portfolio, and
  unknown Margin; mode remained available through the amount disclosure and
  leverage through Symbol details. Amount right edges matched within 0.5 CSS
  pixels. At 1280px and 1920px viewport widths, the Margin column needed only
  its longest displayed amount and cell padding, with less than 2px residual.
- The :8081 embedded SPA walkthrough used fake data in `plutus_agent` and
  exercised aggregate Futures, expanded PM Account details, and the 14:55
  historical point. All three placed Unrealized PnL before Margin. The aggregate
  Margin column measured 133.58 CSS pixels, including 24px cell padding; all
  eight amounts shared an identical right edge. Mode, calculation provenance,
  and leverage disclosures remained available. Historical amounts and Report
  times matched the earlier walkthrough.
- Markdown formatting, 35 affected local path/anchor checks, and
  `git diff --check` passed. The user's existing staged changes remained
  byte-identical. The agent script and all child processes were stopped; the
  :8081 health request failed afterward, with no :8081 or :4173 listeners.

Unknown and Isolated variants used synthetic Chromium responses rather than
exchange-app verification. No live exchange calls were made for this UI change.

### Margin mode and leverage follow-up

The compact Margin disclosure replaces calculation and provenance paragraphs
with its mode and leverage. This changes only display leverage, leaving source
leverage, Margin calculation, and API facts intact.

- The ordered format, web, build, test, and lint gates passed. All Go packages,
  51 frontend unit tests, and four Chromium tests passed.
- Unit tests covered stablecoin and Base-settled isolated margin, unrounded
  source inputs near a display-rounding boundary, configured leverage differing
  from actual isolated leverage, zero notional, and missing or invalid margin,
  price, currency, and leverage inputs. Isolated leverage does not fall back to
  configured leverage. Source amounts and leverage remained unchanged.
- Chromium asserted exactly `3x cross`, `1.88x isolated`, and `portfolio margin`
  in the Margin overlay, with no leverage in Symbol details. The existing
  keyboard, click, Escape, width, amount alignment, and narrow scrolling checks
  passed.
- The :8081 fake walkthrough verified `5x cross` and `portfolio margin` on the
  aggregate table, plus `portfolio margin` in Account and historical details.
  The Symbol overlay contained only contract data. Historical amounts and
  observed times remained unchanged. Actual isolated leverage was verified by
  synthetic tests; this fake walkthrough had no isolated Futures row.
- Markdown formatting, 44 affected local path/anchor checks, and
  `git diff --check` passed. Existing staged changes were preserved. The agent
  script and child processes were stopped; its health request failed afterward,
  with no :8081 or :4173 listeners. No live exchange verification was performed.

## Evidence and remaining gaps

No live credentials or new recording are used. Existing accepted evidence is
unchanged: [Binance](./binance/verification.md) from 2026-09-30 UTC,
[OKX](./okx/verification.md), [Bybit](./bybit/verification.md), and
[Kraken](./kraken/verification.md) from 2026-10-01. It establishes separately
observed fields and scenarios rather than atomic Reports or complete app
reconciliation.

Binance PM responses include initial leverage in one-way and hedged UM/CM
positions. The owner's screenshots approximately match the documented formula:
ZEC gives 219.635 USDT against displayed 219.68; the other position gives
27.717333... against 27.71. Different field update times or presentation could
explain the residuals, but that explanation has not been established.

OKX reviewed Futures/Multi-currency inverse ETH rows support native units; the
published REST discrepancy and other hedge/tier configurations still require
exchange confirmation. Existing Bybit Cross hedge records have asymmetric
`positionIM`, supporting preservation of the source requirement. Bybit actual
isolated allocation remains unobserved and unavailable. Kraken Cross position
allocation is not established. PM Pro is outside the offered Binance scope.

Primary sources used for the revised policy:

- [Binance PM initial-margin and leverage rules](https://www.binance.com/en/support/faq/detail/174861644e7049aaa64845f1db2e4376).
- [Binance PM versus PM Pro](https://www.binance.com/en/support/faq/detail/ee2d73ec5efc4cd6b75c28a9f5a31026).
- [OKX native Futures margin rules](https://www.okx.com/en-sg/help/futures-margin-calculation-rules).
- [OKX REST position fields](https://app.okx.com/docs-v5/en/#rest-api-trading-account-get-positions).
- [Bybit fees and hedged initial margin](https://www.bybit.com/en/help-center/article/Understanding-the-Adjustment-and-Impact-of-the-New-Margin-Calculation).
- [Kraken effective leverage](https://support.kraken.com/articles/4871775312276-portfolio-management-derivatives).
