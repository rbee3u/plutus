import { NumericText } from "@/shared/ui/NumericText";
import type { ValueChange } from "@/api";
import { formatAmount, formatUsd, parseDecimal } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { reasonText } from "@/shared/lib/text";
import { Badge, CompletenessBadge } from "@/shared/ui/Badge";
import { HoverDetails } from "@/shared/ui/HoverDetails";
import { type DataRow } from "@/shared/ui/DataTable";
import shared from "@/shared/styles/page.module.css";

export const VALUE_CHANGE_COLUMNS = [
  { key: "label", header: "Period" },
  { key: "amountUsd", header: "Value change (USD)", numeric: true },
  { key: "end", header: "Close (USD)", numeric: true },
  { key: "flows", header: "Flows", numeric: true },
  { key: "issues", header: "Issues" },
];

export function valueChangeCell(amount: string | null | undefined, startAmount?: string | null) {
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

  const start = parseDecimal(startAmount);
  let pctText = "";
  if (start && !start.isZero()) {
    const pct = parsed.dividedBy(start.abs()).times(100);
    const pctSign = pct.gt(0) ? "+" : pct.lt(0) ? "" : "";
    pctText = ` (${pctSign}${pct.toFixed(2)}%)`;
  }

  return (
    <span className={className}>
      <NumericText>{`${sign}$${formatAmount(parsed.abs().toFixed())}${pctText}`}</NumericText>
    </span>
  );
}

export function combinedFlowsCell(item: ValueChange) {
  const inCap = parseDecimal(item.contributionsUsd);
  const outCap = parseDecimal(item.withdrawalsUsd);
  const inBnd = parseDecimal(item.boundaryContributionsUsd);
  const outBnd = parseDecimal(item.boundaryWithdrawalsUsd);

  const hasCapIn = inCap && !inCap.isZero();
  const hasCapOut = outCap && !outCap.isZero();
  const hasBndIn = inBnd && !inBnd.isZero();
  const hasBndOut = outBnd && !outBnd.isZero();

  if (!hasCapIn && !hasCapOut && !hasBndIn && !hasBndOut) {
    return <span className={shared.muted}>—</span>;
  }

  return (
    <span className={shared.cellStack} style={{ alignItems: "flex-end" }}>
      {inCap && !inCap.isZero() ? (
        <span className={shared.pnlPositive}>
          <NumericText>{`+$${formatAmount(inCap.toFixed())} Cap`}</NumericText>
        </span>
      ) : null}
      {outCap && !outCap.isZero() ? (
        <span className={shared.pnlNegative}>
          <NumericText>{`-$${formatAmount(outCap.toFixed())} Cap`}</NumericText>
        </span>
      ) : null}
      {inBnd && !inBnd.isZero() ? (
        <span className={shared.pnlPositive}>
          <NumericText>{`+$${formatAmount(inBnd.toFixed())} Bnd`}</NumericText>
        </span>
      ) : null}
      {outBnd && !outBnd.isZero() ? (
        <span className={shared.pnlNegative}>
          <NumericText>{`-$${formatAmount(outBnd.toFixed())} Bnd`}</NumericText>
        </span>
      ) : null}
    </span>
  );
}

export function valueChangeIssuesCell(item: ValueChange) {
  const isHealthy = item.completeness === "complete" && item.reasons.length === 0;
  if (isHealthy) {
    return <span className={shared.muted}>—</span>;
  }

  const trigger =
    item.completeness !== "complete" ? (
      <CompletenessBadge value={item.completeness} silentWhenHealthy={false} />
    ) : (
      <Badge tone="warn">
        {item.reasons.length === 1 ? "1 note" : `${item.reasons.length} notes`}
      </Badge>
    );

  return (
    <HoverDetails name={`Issues for ${item.label}`} label={trigger}>
      <div className={shared.metaStack}>
        <div>
          <span className={shared.cellMeta}>Completeness</span>
          <div>
            <CompletenessBadge value={item.completeness} silentWhenHealthy={false} />
          </div>
        </div>
        {item.reasons.length > 0 ? (
          <div>
            <span className={shared.cellMeta}>Limitations</span>
            <ul style={{ margin: "4px 0 0", paddingLeft: "16px" }}>
              {item.reasons.map((reason, i) => (
                <li key={i}>{reasonText(reason)}</li>
              ))}
            </ul>
          </div>
        ) : null}
      </div>
    </HoverDetails>
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
    amountUsd: valueChangeCell(item.amountUsd, item.startPoint?.totalUsd),
    end: (
      <span className={shared.cellStack} style={{ alignItems: "flex-end" }}>
        {item.endPoint?.totalUsd ? (
          <NumericText>{formatUsd(item.endPoint.totalUsd)}</NumericText>
        ) : (
          <span className={shared.muted}>—</span>
        )}
        {item.startPoint?.totalUsd ? (
          <span className={shared.cellMeta}>from {formatAmount(item.startPoint.totalUsd)}</span>
        ) : null}
      </span>
    ),
    flows: combinedFlowsCell(item),
    issues: valueChangeIssuesCell(item),
  };
}
