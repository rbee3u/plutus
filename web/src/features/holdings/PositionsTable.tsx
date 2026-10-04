import type { PositionRow } from "@/api/generated/types.gen";
import Decimal from "decimal.js";
import {
  presentNumber,
  formatDecimal,
  formatLeverage,
  formatCount,
  parseDecimal,
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
  { key: "unrealizedPnl", header: "Unrealized PnL", numeric: true, nowrap: true },
  { key: "margin", header: "Margin", numeric: true, nowrap: true },
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

// Keep source-decimal multiplication and division ahead of display rounding.
const MarginDecimal = Decimal.clone({ precision: 200 });

function usdEquivalent(currency: string | null | undefined): boolean {
  return currency === "USD" || currency === "USDT" || currency === "USDC";
}

export function positionMarginDescription(item: PositionRow): string {
  if (item.marginMode === "isolated") {
    const margin = parseDecimal(item.settleMargin);
    const notional = parseDecimal(item.notionalUsd);
    if (!margin || margin.lte(0) || !notional || !item.settlementSymbol) return "isolated";
    let marginUsd = new MarginDecimal(margin);
    if (!usdEquivalent(item.settlementSymbol)) {
      const mark = parseDecimal(item.markPrice);
      if (
        item.settlementSymbol !== item.baseSymbol ||
        !usdEquivalent(item.quoteSymbol) ||
        !mark ||
        mark.lte(0)
      )
        return "isolated";
      marginUsd = marginUsd.times(mark);
    }
    const leverage = new MarginDecimal(notional).abs().div(marginUsd);
    return `${formatLeverage(leverage.toString())} isolated`;
  }
  if (item.portfolioMargin) return "portfolio margin";
  if (item.marginMode === "cross") {
    const leverage = parseDecimal(item.leverage);
    return leverage && leverage.gt(0) ? `${formatLeverage(item.leverage)} cross` : "cross";
  }
  return "margin mode unavailable";
}

export function positionRow(item: PositionRow, timezone?: string): DataRow {
  const inverse = item.kind.startsWith("inverse_");
  const delivery = item.kind.endsWith("_delivery");
  const factorUnit = inverse ? item.quoteSymbol : item.baseSymbol;
  const anomalies = (item.issues ?? []).filter((issue) => issue.code !== "stale");
  const stale = (item.issues ?? []).find((issue) => issue.code === "stale");
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
      <HoverDetails
        className={styles.positionMarginTrigger}
        label={<NumericText>{marginLabel}</NumericText>}
        name={`Margin details for ${item.symbol}: ${plainNumberText(marginLabel)}`}
      >
        <p>
          <NumericText>{positionMarginDescription(item)}</NumericText>
        </p>
      </HoverDetails>
    ),
    unrealizedPnl: amountCell(item.unrealizedPnl, item.settlementSymbol ?? null),
    notionalUsd: amountCell(item.notionalUsd, undefined, "usd"),
    observed: (
      <span className={styles.statusGroup}>
        <span
          className={styles.muted}
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
