import type { ReactNode } from "react";
import type { PositionRow } from "@/api";
import {
  presentNumber,
  formatAmount,
  formatDecimal,
  formatLeverage,
  parseDecimal,
  plainNumberText,
  type NumberKind,
} from "@/shared/lib/amount";
import { formatContractSize } from "@/shared/lib/contract";
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
  { key: "unrealizedPnl", header: "Unrealized PnL", numeric: true, nowrap: true, width: "130px" },
  { key: "margin", header: "Margin", numeric: true, nowrap: true, width: "118px" },
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

/** Contract facts in Catalog futures column order, with the Catalog values. */
export function contractDetailRows(item: PositionRow, timezone?: string) {
  const delivery = item.kind.endsWith("_delivery");
  const rows: Array<{ label: string; value: ReactNode }> = [
    { label: "Base", value: item.baseAssetSymbol ?? "—" },
    { label: "Quote", value: item.quoteAssetSymbol ?? "—" },
    { label: "Settle", value: item.settlementAssetSymbol ?? "—" },
  ];
  if (delivery) {
    rows.push({
      label: "Expiry",
      value: item.expiryAt ? formatInstant(item.expiryAt, timezone) : "—",
    });
  }
  rows.push({
    label: "Contract size",
    value: <NumericText>{formatContractSize(item)}</NumericText>,
  });
  return rows;
}

export function entryNotionalUsd(item: PositionRow): string | null {
  if (item.entryPrice == null) return null;
  const size = parseDecimal(item.size);
  if (!size) return null;
  if (item.kind.startsWith("inverse_")) {
    return size.abs().toFixed();
  }
  const price = parseDecimal(item.entryPrice);
  if (!price || price.lte(0)) return null;
  return size.abs().times(price).toFixed();
}

function priceCell(
  price: string | null | undefined,
  notionalValue: string | null | undefined,
  label: string,
  symbol: string,
) {
  if (price == null) {
    return <span title="Price is unavailable.">—</span>;
  }
  const priceText = presentNumber(price, "price").text;
  const notional = formatAmount(notionalValue);
  const notionalText = notional === "—" ? "—" : `$${notional}`;
  return (
    <HoverDetails
      label={<NumericText>{priceText}</NumericText>}
      name={`${label} for ${symbol}: ${plainNumberText(priceText)}`}
    >
      <p>
        <NumericText>{`Notional: ${notionalText}`}</NumericText>
      </p>
    </HoverDetails>
  );
}

export function positionRow(item: PositionRow, timezone?: string): DataRow {
  const delivery = item.kind.endsWith("_delivery");
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
          tags={[
            item.kind.startsWith("inverse_") ? "Inverse" : "Linear",
            delivery ? "Delivery" : "Perpetual",
          ]}
        >
          <dl>
            {contractDetailRows(item, timezone).map(({ label, value }) => (
              <div key={label}>
                <dt>{label}</dt>
                <dd>{value}</dd>
              </div>
            ))}
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
    entryPrice: priceCell(
      item.entryPrice,
      entryNotionalUsd(item),
      "Entry price details",
      item.futuresSymbol,
    ),
    markPrice: priceCell(
      item.markPrice,
      item.notionalUsd,
      "Mark price details",
      item.futuresSymbol,
    ),
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
