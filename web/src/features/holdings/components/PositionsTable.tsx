import type { PositionRow } from "@/api";
import Decimal from "decimal.js";
import {
  presentNumber,
  formatDecimal,
  formatLeverage,
  parseDecimal,
  plainNumberText,
  type NumberKind,
} from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { Badge } from "@/shared/ui/Badge";
import { HoverCard, HoverDetails } from "@/shared/ui/HoverDetails";
import type { DataRow } from "@/shared/ui/DataTable";
import { NumericText } from "@/shared/ui/NumericText";
import { ObservedCell } from "./ObservedCell";
import shared from "@/shared/styles/page.module.css";

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
  { key: "issues", header: "Issues", nowrap: true },
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

function pnlCell(value: string | null | undefined, currency?: string | null) {
  if (value == null || currency === null) {
    return <span title="Amount or currency is unavailable.">—</span>;
  }
  const parsed = parseDecimal(value);
  if (!parsed) {
    return <span title="Amount is unavailable.">—</span>;
  }
  const unit = currency ? ` ${currency}` : "";
  const sign = parsed.gt(0) ? "+" : "";
  const className = parsed.gt(0)
    ? shared.pnlPositive
    : parsed.lt(0)
      ? shared.pnlNegative
      : shared.pnlNeutral;
  return (
    <span className={className}>
      <NumericText>{`${sign}${presentNumber(value, "quantity").text}${unit}`}</NumericText>
    </span>
  );
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
  const marginLabel =
    item.settleMargin != null && item.settlementSymbol
      ? `${formatDecimal(item.settleMargin)} ${item.settlementSymbol}`
      : "—";
  return {
    id: `${item.accountId}-${item.futuresContractId}-${item.platformPositionRef}`,
    symbol: (
      <HoverDetails
        label={item.futuresSymbol}
        name={`Contract details for ${item.futuresSymbol}`}
        preferredPlacement="right"
      >
        <HoverCard
          title={item.futuresSymbol}
          tags={[inverse ? "Inverse" : "Linear", delivery ? "Delivery" : "Perpetual"]}
        >
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
            {delivery && item.expiryAt ? (
              <div>
                <dt>Delivery</dt>
                <dd>
                  {formatInstant(item.expiryAt, timezone)} {timezone}
                </dd>
              </div>
            ) : null}
          </dl>
        </HoverCard>
      </HoverDetails>
    ),
    side: (
      <Badge
        tone={item.positionMode === "one_way" ? "neutral" : item.side === "long" ? "good" : "bad"}
      >
        {item.positionMode === "one_way" ? "Both" : item.side === "long" ? "Long" : "Short"}
      </Badge>
    ),
    size: amountCell(item.size, item.sizeSymbol ?? null),
    entryPrice: amountCell(item.entryPrice, undefined, "price"),
    markPrice: amountCell(item.markPrice, undefined, "price"),
    liquidationPrice: amountCell(item.liquidationPrice, undefined, "price"),
    margin: (
      <HoverDetails
        className={shared.positionMarginTrigger}
        label={<NumericText>{marginLabel}</NumericText>}
        name={`Margin details for ${item.futuresSymbol}: ${plainNumberText(marginLabel)}`}
      >
        <p>
          <NumericText>{positionMarginDescription(item)}</NumericText>
        </p>
      </HoverDetails>
    ),
    unrealizedPnl: pnlCell(item.unrealizedPnl, item.settlementSymbol ?? null),
    notionalUsd: amountCell(item.notionalUsd, undefined, "usd"),
    issues: (
      <ObservedCell
        observedAt={item.reportObservedAt}
        freshness={item.freshness}
        issues={item.issues}
        name={item.futuresSymbol}
        timezone={timezone}
      />
    ),
  };
}
