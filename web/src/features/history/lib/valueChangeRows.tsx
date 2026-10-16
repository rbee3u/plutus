import { NumericText } from "@/shared/ui/NumericText";
import type { ValueChange } from "@/api";
import { formatAmount, formatPercent, parseDecimal } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { IssueList } from "@/shared/ui/IssueList";
import { Badge, CompletenessBadge } from "@/shared/ui/Badge";
import { HoverDetails } from "@/shared/ui/HoverDetails";
import { type DataRow } from "@/shared/ui/DataTable";
import shared from "@/shared/styles/page.module.css";

export const VALUE_CHANGE_COLUMNS = [
  { key: "label", header: "Period" },
  { key: "amountUsd", header: "Value change (USD)", numeric: true, width: "180px" },
  { key: "end", header: "Close (USD)", numeric: true, width: "160px" },
  { key: "flows", header: "Flows", numeric: true, width: "140px" },
  { key: "issues", header: "Issues", width: "120px" },
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
    const pct = parsed.dividedBy(start.abs());
    const pctSign = pct.gt(0) ? "+" : pct.lt(0) ? "" : "";
    pctText = ` (${pctSign}${formatPercent(pct.toFixed())})`;
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
        <span className={shared.cellStack} style={{ alignItems: "flex-end" }}>
          <span className={shared.pnlPositive}>
            <NumericText>{`+$${formatAmount(inCap.toFixed())}`}</NumericText>
          </span>
          <span className={shared.cellMeta}>Capital flow</span>
        </span>
      ) : null}
      {outCap && !outCap.isZero() ? (
        <span className={shared.cellStack} style={{ alignItems: "flex-end" }}>
          <span className={shared.pnlNegative}>
            <NumericText>{`-$${formatAmount(outCap.toFixed())}`}</NumericText>
          </span>
          <span className={shared.cellMeta}>Capital flow</span>
        </span>
      ) : null}
      {inBnd && !inBnd.isZero() ? (
        <span className={shared.cellStack} style={{ alignItems: "flex-end" }}>
          <span className={shared.pnlPositive}>
            <NumericText>{`+$${formatAmount(inBnd.toFixed())}`}</NumericText>
          </span>
          <span className={shared.cellMeta}>Boundary event</span>
        </span>
      ) : null}
      {outBnd && !outBnd.isZero() ? (
        <span className={shared.cellStack} style={{ alignItems: "flex-end" }}>
          <span className={shared.pnlNegative}>
            <NumericText>{`-$${formatAmount(outBnd.toFixed())}`}</NumericText>
          </span>
          <span className={shared.cellMeta}>Boundary event</span>
        </span>
      ) : null}
    </span>
  );
}

export function valueChangeIssuesCell(item: ValueChange) {
  const isHealthy = item.completeness === "complete" && item.issues.length === 0;
  if (isHealthy) {
    return <span className={shared.muted}>—</span>;
  }

  const trigger =
    item.completeness !== "complete" ? (
      <CompletenessBadge value={item.completeness} silentWhenHealthy={false} />
    ) : (
      <Badge tone="warn">
        {item.issues.length === 1 ? "1 note" : `${item.issues.length} notes`}
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
        {item.issues.length > 0 ? (
          <div>
            <span className={shared.cellMeta}>Limitations</span>
            <IssueList issues={item.issues} />
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
          <NumericText>{formatAmount(item.endPoint.totalUsd)}</NumericText>
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
