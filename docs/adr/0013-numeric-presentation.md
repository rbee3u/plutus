---
status: accepted
---

# ADR 0013: Shared numeric presentation

## Decision

Cards, tables, current and historical details, explanatory text, counts, and
chart tooltips share one numeric presentation policy. Select precision by the
absolute value; retain the existing sign and unit. The limits below are maxima,
not padding requirements. Here `d` is the number of integer digits.

| Type                    | Absolute value at least 1         | Nonzero absolute value below 1 |
| ----------------------- | --------------------------------- | ------------------------------ |
| USD value               | 2 fractional digits               | 4 significant digits           |
| Asset quantity or price | `max(2, 6 - d)` fractional digits | 6 significant digits           |
| Unscaled ratio          | `max(2, 4 - d)` fractional digits | 4 significant digits           |
| Percentage or leverage  | 2 fractional digits               | 4 significant digits           |
| Count                   | Whole integer                     | Not applicable                 |

Round to nearest with midpoint ties away from zero. Remove fractional trailing
zeros, normalize negative zero to zero, and group integer digits with English
thousands commas. Missing, invalid, and non-finite values display as `—`; they
never become zero. Known zero displays as `0`, retaining a relevant unit.

After rounding, a fractional prefix of at least four consecutive zeros uses
zero-count shorthand: `0.00000444` becomes `0.0₅444`. The subscript counts all
zeros after the decimal point before the first nonzero digit. It may contain
multiple digits. This preserves nonzero values beyond eight fractional places;
there is no eight-place display ceiling. A rounding carry may remove shorthand:
USD `0.000099999` becomes `0.0001`.

APY multiplies the stored fractional rate by 100 before selecting percentage
precision. IMR, MMR, and UniMMR remain unscaled ratios, and leverage retains its
`x` suffix. Source Asset amounts use quantity precision even when the Asset is
USD-equivalent; derived USD valuations use USD precision. Price precision is
independent of the quoting Asset.

Valuation amounts, flows, and differences express currency exclusively using the
canonical asset code `USD` (via column headers, adjacent unit markers, or
trailing unit text). Currency symbols such as `$` are prohibited across all user
interface views.

Chart tooltips use source decimal strings and the USD policy. Numeric axes are
the explicit exception: values at least 1 use English compact notation with at
most one fractional digit. Smaller nonzero ticks round to four significant
digits; rounded absolute values below `0.0001` use scientific notation. A carry
to 1 uses compact notation, and zero stays zero.

Screen readers receive the rounded ordinary decimal rather than the zero-count
shorthand. No full-precision disclosure or copy-value interaction is offered.
Position details retain units, contract terms, margin provenance, conversion,
and data issues, using the same precision as the main display.

Inputs, submitted values, diagnostic JSON, API decimals, storage, calculations,
aggregation, and ordering retain source precision. Dates, identifiers, protocol
values, and versions are not numeric amounts. Totals are formatted after
aggregation; adding rounded rows can differ from a displayed total.

## Reason and trade-offs

The accepted preference favors concise amounts while preserving more detail in
quantities and prices. Adaptive precision avoids treating a small holding as
zero; zero-count notation avoids making users count long zero prefixes. Fixed
fractional limits and significant-digit limits solve different problems, so
small values use significant digits rather than an eight-place cutoff.

The choices were confirmed for Plutus, not adopted as a universal industry
standard.
[Bybit Spot Cost](https://www.bybit.com/en/help-center/article/How-to-Calculate-View-or-Adjust-Spot-Cost-in-Unified-Trading-Account)
provides a precedent for adaptive two-place/four-significant-digit display;
[Capital.com](https://help.capital.com/hc/en-us/articles/33093412785170-Why-do-some-crypto-prices-look-different)
documents zero-count subscripts. Neither establishes this project's combined
thresholds or type-specific precision.

Variable precision sacrifices fixed decimal alignment. Removing source-value
disclosure simplifies the interface and means rounded displays alone cannot
reconstruct exact source values. Preserving source precision in data and forms
keeps presentation from changing financial meaning.
