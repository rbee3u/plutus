import { NumericText } from "@/shared/ui/NumericText";
import type { ValueChange } from "@/api";
import { formatAmount, formatUsd, parseDecimal } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { reasonText } from "@/shared/lib/text";
import { CompletenessBadge } from "@/shared/ui/Badge";
import { type DataRow } from "@/shared/ui/DataTable";
import shared from "@/shared/styles/page.module.css";

export const VALUE_CHANGE_COLUMNS = [
  { key: "label", header: "Period" },
  { key: "amountUsd", header: "Value change (USD)", numeric: true },
  { key: "start", header: "Start", numeric: true },
  { key: "end", header: "End", numeric: true },
  { key: "capitalFlows", header: "Capital flows", numeric: true },
  { key: "boundaryFlows", header: "Boundary flows", numeric: true },
  { key: "completeness", header: "Completeness" },
  { key: "reasons", header: "Limitations" },
];

export function valueChangeCell(amount: string | null | undefined) {
  const parsed = parseDecimal(amount);
  if (!parsed) {
    return <span className={shared.muted}>—</span>;
  }
  const isPositive = parsed.gt(0);
  const isNegative = parsed.lt(0);
  const sign = isPositive ? "+" : isNegative ? "-" : "";
  const className = isPositive
    ? shared.pnlPositive
    : isNegative
      ? shared.pnlNegative
      : shared.pnlNeutral;
  return (
    <span className={className}>
      <NumericText>{`${sign}$${formatAmount(parsed.abs().toFixed())}`}</NumericText>
    </span>
  );
}

function balanceCell(point: ValueChange["startPoint"]) {
  if (!point || !point.totalUsd) {
    return <span className={shared.muted}>—</span>;
  }
  return <NumericText>{formatUsd(point.totalUsd)}</NumericText>;
}

function flowsCell(inflow: string | null | undefined, outflow: string | null | undefined) {
  const inVal = parseDecimal(inflow);
  const outVal = parseDecimal(outflow);
  const hasIn = inVal && !inVal.isZero();
  const hasOut = outVal && !outVal.isZero();

  if (!hasIn && !hasOut) {
    return <span className={shared.muted}>—</span>;
  }
  if (hasIn && !hasOut) {
    return (
      <span className={shared.pnlPositive}>
        <NumericText>{`+$${formatAmount(inVal.toFixed())}`}</NumericText>
      </span>
    );
  }
  if (!hasIn && hasOut) {
    return (
      <span className={shared.pnlNegative}>
        <NumericText>{`-$${formatAmount(outVal.toFixed())}`}</NumericText>
      </span>
    );
  }
  return (
    <span className={shared.cellStack} style={{ alignItems: "flex-end" }}>
      <span className={shared.pnlPositive}>
        <NumericText>{`+$${formatAmount(inVal!.toFixed())}`}</NumericText>
      </span>
      <span className={shared.pnlNegative}>
        <NumericText>{`-$${formatAmount(outVal!.toFixed())}`}</NumericText>
      </span>
    </span>
  );
}

export function valueChangeRow(item: ValueChange, timezone?: string): DataRow {
  return {
    id: `${item.periodKind}-${item.from}`,
    label: (
      <span className={shared.cellStack}>
        <span>{item.label}</span>
        <span className={`${shared.cellMeta} ${shared.nowrap}`}>
          {formatInstant(item.from, timezone)} – {formatInstant(item.to, timezone)}
        </span>
      </span>
    ),
    amountUsd: valueChangeCell(item.amountUsd),
    start: balanceCell(item.startPoint),
    end: balanceCell(item.endPoint),
    capitalFlows: flowsCell(item.contributionsUsd, item.withdrawalsUsd),
    boundaryFlows: flowsCell(item.boundaryContributionsUsd, item.boundaryWithdrawalsUsd),
    completeness: <CompletenessBadge value={item.completeness} />,
    reasons: item.reasons.length > 0 ? item.reasons.map(reasonText).join(", ") : "—",
  };
}
