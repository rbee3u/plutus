---
status: accepted
---

# Contract quantities and margin provenance

Normalize position size as base quantity for linear contracts and quote face
amount for inverse contracts. This preserves fixed contract size and meaningful
units instead of displaying incomparable native counts or mark-dependent
settlement quantities. Position mode remains distinct from direction.

Margin interpretation is platform-specific: cross initial requirements and
isolated allocations are different measures. Preserve source amount, currency,
and basis; distinguish reported and calculated display values. Verified platform
formulas may supply missing requirements, but shared collateral and portfolio
risk-unit requirements are never distributed across positions.

Supported Binance cross requirements can use initial leverage, including retail
portfolio margin. Reported Bybit fees and hedge treatment take precedence over
that formula, and Kraken effective leverage is not inverted. OKX non-PM inverse
margin needs unambiguous unit verification against native and USD requirements.
Missing provenance and ambiguous amounts remain unknown; the source report, not
current identity settings, establishes historical account mode.

This accepts partial optional fields in exchange for trustworthy units and
history. The read side may derive presentation without rewriting source amounts.
Historical recovery requires exact platform references and compatible direction;
screenshots or synthetic checks alone do not establish precise exchange-app
reconciliation.

[Contract rules](../domain-rules.md#contract-quantities-and-margin) own
formulas; [table presentation](../presentation.md#futures) owns disclosures and
columns. [platform mappings](../../README.md#documentation) own field evidence,
including
[OKX provenance](../platform/okx/mapping.md#position-presentation-provenance).
