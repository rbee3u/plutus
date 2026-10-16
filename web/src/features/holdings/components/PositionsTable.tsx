import type { PositionRow } from "@/api";
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
  { key: "side", header: "Side", nowrap: true, width: "80px" },
  { key: "size", header: "Size", numeric: true, nowrap: true, width: "110px" },
  { key: "entryPrice", header: "Entry price", numeric: true, nowrap: true, width: "120px" },
  { key: "markPrice", header: "Mark price", numeric: true, nowrap: true, width: "120px" },
  { key: "liquidationPrice", header: "Liq. price", numeric: true, nowrap: true, width: "120px" },
  { key: "unrealizedPnl", header: "Unrealized PnL", numeric: true, nowrap: true, width: "130px" },
  { key: "margin", header: "Margin", numeric: true, nowrap: true, width: "118px" },
  { key: "notionalUsd", header: "Notional (USD)", numeric: true, nowrap: true, width: "130px" },
  { key: "issues", header: "Issues", nowrap: true, width: "100px" },
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

export function positionMarginDescription(item: PositionRow): string {
  const leverage = item.displayLeverage;
  if (item.marginSetting === "isolated") {
    return leverage == null ? "isolated" : `${formatLeverage(leverage)} isolated`;
  }
  if (item.portfolioMargin) return "portfolio margin";
  if (item.marginSetting === "cross") {
    return leverage == null ? "cross" : `${formatLeverage(leverage)} cross`;
  }
  return "margin mode unavailable";
}

export function positionRow(item: PositionRow, timezone?: string): DataRow {
  const inverse = item.kind.startsWith("inverse_");
  const delivery = item.kind.endsWith("_delivery");
  const factorUnit = inverse ? item.quoteAssetSymbol : item.baseAssetSymbol;
  const marginLabel =
    item.settlementMargin != null && item.settlementAssetSymbol
      ? `${formatDecimal(item.settlementMargin)} ${item.settlementAssetSymbol}`
      : "—";
  return {
    id: `${item.accountId}-${item.futuresContractId}-${item.positionKey}`,
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
              <dd>{item.baseAssetSymbol ?? "—"}</dd>
            </div>
            <div>
              <dt>Quote</dt>
              <dd>{item.quoteAssetSymbol ?? "—"}</dd>
            </div>
            <div>
              <dt>Settle</dt>
              <dd>{item.settlementAssetSymbol ?? "—"}</dd>
            </div>
            {item.contractSize ? (
              <div>
                <dt>Contract size</dt>
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
    unrealizedPnl: pnlCell(item.unrealizedPnl, item.settlementAssetSymbol ?? null),
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
