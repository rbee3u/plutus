import type { PositionRow } from "@/api/generated/types.gen";
import {
  presentNumber,
  formatDecimal,
  formatPrice,
  formatLeverage,
  formatCount,
  plainNumberText,
  type NumberKind,
} from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { Badge } from "@/shared/ui/Badge";
import { HoverDetails } from "@/shared/ui/HoverDetails";
import type { DataRow } from "@/shared/ui/DataTable";
import { NumericText } from "@/shared/ui/NumericText";
import styles from "@/features/pages.module.css";

export const POSITION_COLUMNS = [
  { key: "symbol", header: "Symbol", nowrap: true },
  { key: "side", header: "Side", nowrap: true },
  { key: "size", header: "Size", numeric: true, nowrap: true },
  { key: "entryPrice", header: "Entry Price", numeric: true, nowrap: true },
  { key: "markPrice", header: "Mark Price", numeric: true, nowrap: true },
  { key: "liquidationPrice", header: "Liq. Price", numeric: true, nowrap: true },
  { key: "margin", header: "Margin", numeric: true, nowrap: true },
  { key: "unrealizedPnl", header: "Unrealized PnL", numeric: true, nowrap: true },
  { key: "notionalUsd", header: "Notional (USD)", numeric: true, nowrap: true },
  { key: "observed", header: "Observed", nowrap: true },
];

function amountCell(
  value: string | null | undefined,
  currency?: string | null,
  kind: NumberKind = "quantity",
) {
  if (value == null || currency === null)
    return <span title="Amount or currency is unavailable.">—</span>;
  const unit = currency ? ` ${currency}` : "";
  return <NumericText>{`${presentNumber(value, kind).text}${unit}`}</NumericText>;
}

function unavailableMargin(item: PositionRow): string {
  if (item.portfolioMargin && item.marginMode !== "isolated")
    return "Portfolio risk is not allocated to individual positions.";
  if (!item.marginMode) return "The position's Cross or Isolated setting was not recorded.";
  if (item.marginMode === "isolated" && item.marginBasis !== "allocated")
    return "Allocated margin excluding PnL was not provided. An initial requirement cannot be used as the allocated amount.";
  if (item.marginMode === "cross" && item.marginBasis !== "initial")
    return "A position initial requirement excluding orders was not recorded.";
  if (item.marginMode === "cross" && item.portfolioMargin == null)
    return "The source Report's Account Margin Mode is unknown, so individual cross risk cannot be verified.";
  if (item.margin == null) return "The Platform did not provide this position margin amount.";
  if (!item.marginCurrency || !item.settlementSymbol)
    return "The reported margin currency or Settle currency is unknown.";
  return "The reported amount cannot be converted to Settle: conversion requires compatible currencies and, for a Base-settled contract, a positive Mark Price with a USD-equivalent Quote.";
}

export function positionRow(item: PositionRow, timezone?: string): DataRow {
  const inverse = item.kind.startsWith("inverse_");
  const delivery = item.kind.endsWith("_delivery");
  const factorUnit = inverse ? item.quoteSymbol : item.baseSymbol;
  const mode =
    item.marginMode === "cross" ? "Cross" : item.marginMode === "isolated" ? "Isolated" : null;
  const anomalies = (item.issues ?? []).filter((issue) => issue.code !== "stale");
  const stale = (item.issues ?? []).find((issue) => issue.code === "stale");
  const basis =
    item.marginBasis === "allocated"
      ? "Allocated margin, excluding PnL"
      : item.marginBasis === "initial"
        ? "Position initial requirement, excluding orders"
        : "Margin basis is unavailable";
  const marginLabel =
    item.settleMargin != null && item.settlementSymbol
      ? `${formatDecimal(item.settleMargin)} ${item.settlementSymbol}`
      : "—";
  return {
    id: `${item.accountId}-${item.instrumentId}-${item.platformPositionRef}`,
    symbol: (
      <HoverDetails label={item.symbol} name={`Contract details for ${item.symbol}`}>
        <dl>
          <div>
            <dt>Base</dt>
            <dd>{item.baseSymbol ?? "—"}</dd>
          </div>
          <div>
            <dt>Quote</dt>
            <dd>{item.quoteSymbol ?? "—"}</dd>
          </div>
          <div>
            <dt>Settle</dt>
            <dd>{item.settlementSymbol ?? "—"}</dd>
          </div>
          <div>
            <dt>Leverage</dt>
            <dd>
              <NumericText>{formatLeverage(item.leverage)}</NumericText>
            </dd>
          </div>
          {item.contractSize ? (
            <div>
              <dt>Contract Size</dt>
              <dd>
                1 contract = <NumericText>{formatDecimal(item.contractSize)}</NumericText>{" "}
                {factorUnit ?? "—"}
              </dd>
            </div>
          ) : null}
          <div>
            <dt>Payoff</dt>
            <dd>{inverse ? "Inverse" : "Linear"}</dd>
          </div>
          <div>
            <dt>Term</dt>
            <dd>{delivery ? "Delivery" : "Perpetual"}</dd>
          </div>
          {delivery ? (
            <div>
              <dt>Delivery</dt>
              <dd>
                {formatInstant(item.expiryAt, timezone)} {timezone}
              </dd>
            </div>
          ) : null}
        </dl>
      </HoverDetails>
    ),
    side: item.positionMode === "one_way" ? "Both" : item.side === "long" ? "Long" : "Short",
    size: amountCell(item.size, item.sizeSymbol ?? null),
    entryPrice: amountCell(item.entryPrice, undefined, "price"),
    markPrice: amountCell(item.markPrice, undefined, "price"),
    liquidationPrice: amountCell(item.liquidationPrice, undefined, "price"),
    margin: (
      <span className={styles.cellStack}>
        <HoverDetails
          label={<NumericText>{marginLabel}</NumericText>}
          name={`Margin details for ${item.symbol}: ${plainNumberText(marginLabel)}`}
        >
          <p>{basis}</p>
          <p>
            Reported: <NumericText>{formatDecimal(item.margin)}</NumericText>{" "}
            {item.marginCurrency ?? "(currency unavailable)"}
          </p>
          {item.settleMargin != null ? (
            <p>
              Displayed: <NumericText>{formatDecimal(item.settleMargin)}</NumericText>{" "}
              {item.settlementSymbol}
            </p>
          ) : (
            <p>{unavailableMargin(item)}</p>
          )}
          {item.settleMargin != null && item.marginCurrency !== item.settlementSymbol ? (
            <p>
              Converted {item.marginCurrency} to {item.settlementSymbol}
              {["USD", "USDT", "USDC"].includes(item.settlementSymbol ?? "") ? (
                " at 1:1."
              ) : (
                <NumericText>{` using Mark Price ${formatPrice(item.markPrice)} ${item.quoteSymbol}/${item.baseSymbol}.`}</NumericText>
              )}
            </p>
          ) : null}
        </HoverDetails>
        <span className={styles.statusGroup}>
          {mode ? <Badge>{mode}</Badge> : null}
          {item.portfolioMargin ? <Badge>Portfolio</Badge> : null}
        </span>
      </span>
    ),
    unrealizedPnl: amountCell(item.unrealizedPnl, item.settlementSymbol ?? null),
    notionalUsd: amountCell(item.notionalUsd, undefined, "usd"),
    observed: (
      <span className={styles.statusGroup}>
        <span
          title={`Account Report observed at ${item.reportObservedAt ?? "an unknown time"}; times in ${timezone ?? "the Portfolio timezone"}.`}
        >
          {formatInstant(item.reportObservedAt, timezone)}
        </span>
        {item.freshness === "stale" ? (
          <HoverDetails
            label={<Badge tone="warn">Stale</Badge>}
            name={`Stale report for ${item.symbol}`}
          >
            <p>{stale?.detail ?? "An earlier successful Account Report is carried forward."}</p>
          </HoverDetails>
        ) : null}
        {anomalies.length ? (
          <HoverDetails
            label={
              <Badge tone="warn">
                {formatCount(anomalies.length)} {anomalies.length === 1 ? "issue" : "issues"}
              </Badge>
            }
            name={`Data issues for ${item.symbol}`}
          >
            {anomalies.map((issue, index) => (
              <p key={`${issue.code}-${index}`}>{issue.detail}</p>
            ))}
          </HoverDetails>
        ) : null}
      </span>
    ),
  };
}
