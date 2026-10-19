import { ChevronRight } from "lucide-react";
import type { PeriodClose, ValuationPointSummary } from "@/api";
import { formatAmount, formatPercent, parseDecimal } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { IssueList } from "@/shared/ui/IssueList";
import { Badge, CompletenessBadge } from "@/shared/ui/Badge";
import { type DataRow } from "@/shared/ui/DataTable";
import { HoverDetails } from "@/shared/ui/HoverDetails";
import { IconButton } from "@/shared/ui/IconButton";
import { NumericText } from "@/shared/ui/NumericText";
import { iconSize } from "@/shared/icons/iconSizes";
import { POINT_DETAIL_ID } from "../components/PointDetail";
import shared from "@/shared/styles/page.module.css";

export const PERIOD_CLOSE_COLUMNS = [
  { key: "label", header: "Period" },
  { key: "totalUsd", header: "Close (USD)", numeric: true, width: "160px" },
  { key: "change", header: "Change (USD)", numeric: true, width: "180px" },
  { key: "issues", header: "Issues", width: "120px" },
];

export const RAW_POINT_COLUMNS = [
  { key: "evaluatedAt", header: "Evaluated at", nowrap: true, width: "170px" },
  { key: "totalUsd", header: "Total (USD)", numeric: true, width: "160px" },
  { key: "change", header: "Change (USD)", numeric: true, width: "180px" },
  { key: "issues", header: "Issues", width: "120px" },
  { key: "actions", header: "", width: "80px" },
];

export function deltaChangeCell(
  currentUsd: string | null | undefined,
  previousUsd: string | null | undefined,
) {
  const current = parseDecimal(currentUsd);
  const previous = parseDecimal(previousUsd);
  if (!current || !previous) {
    return <span className={shared.muted}>—</span>;
  }
  const diff = current.minus(previous);
  if (diff.isZero()) {
    return (
      <span className={shared.pnlNeutral}>
        <NumericText>0 (0%)</NumericText>
      </span>
    );
  }
  const isPositive = diff.gt(0);
  const sign = isPositive ? "+" : "-";
  const pct = previous.isZero() ? null : diff.abs().dividedBy(previous.abs());

  const pctText = pct !== null ? ` (${sign}${formatPercent(pct.toFixed())})` : "";
  const text = `${sign}${formatAmount(diff.abs().toFixed())}${pctText}`;

  return (
    <span className={isPositive ? shared.pnlPositive : shared.pnlNegative}>
      <NumericText>{text}</NumericText>
    </span>
  );
}

function pointChangeCell(
  current: ValuationPointSummary | null | undefined,
  previous: ValuationPointSummary | null | undefined,
) {
  const delta = deltaChangeCell(current?.totalUsd, previous?.totalUsd);
  if (!current || !previous) return delta;

  const reasons = [];
  if (previous.completeness === "incomplete") {
    reasons.push("The previous valuation point is incomplete.");
  }
  if (current.completeness === "incomplete") {
    reasons.push("The current valuation point is incomplete.");
  }
  if (reasons.length === 0) return delta;

  return (
    <span className={shared.cellStack} style={{ alignItems: "flex-end" }}>
      {delta}
      <HoverDetails
        name={`Incomplete change: ${reasons.join(" ")}`}
        label={<Badge tone="warn">Incomplete</Badge>}
      >
        <div className={shared.metaStack}>
          {reasons.map((reason) => (
            <p key={reason}>{reason}</p>
          ))}
          <p>The difference uses known totals and may include recovered missing value.</p>
        </div>
      </HoverDetails>
    </span>
  );
}

export function periodCloseIssuesCell(item: PeriodClose) {
  const isHealthy = item.completeness === "complete" && item.issues.length === 0;
  if (isHealthy) {
    return <span className={shared.muted}>—</span>;
  }

  const trigger =
    item.completeness !== "complete" ? (
      <CompletenessBadge value={item.completeness} silentWhenHealthy={false} />
    ) : (
      <Badge tone="warn">
        {item.issues.length === 1 ? "1 issue" : `${item.issues.length} issues`}
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
            <span className={shared.cellMeta}>Missing data</span>
            <IssueList issues={item.issues} />
          </div>
        ) : null}
      </div>
    </HoverDetails>
  );
}

export function periodCloseRow(
  item: PeriodClose,
  previousItem: PeriodClose | undefined,
  timezone?: string,
): DataRow {
  return {
    id: `${item.periodKind}-${item.periodStart}`,
    label: (
      <span className={shared.cellStack}>
        <span>{item.label}</span>
        <span className={`${shared.cellMeta} ${shared.nowrap}`}>
          {formatInstant(item.periodStart, timezone)} – {formatInstant(item.periodEnd, timezone)}
        </span>
      </span>
    ),
    totalUsd: item.point ? (
      <span className={shared.cellStack} style={{ alignItems: "flex-end" }}>
        <NumericText>{formatAmount(item.point.totalUsd)}</NumericText>
        <span className={`${shared.cellMeta} ${shared.nowrap}`}>
          at {formatInstant(item.point.evaluatedAt, timezone)}
        </span>
      </span>
    ) : (
      <span className={shared.cellMeta}>No valuation point</span>
    ),
    change: pointChangeCell(item.point, previousItem?.point),
    issues: periodCloseIssuesCell(item),
  };
}

export function rawPointIssuesCell(item: ValuationPointSummary) {
  const isHealthy =
    item.completeness === "complete" &&
    item.staleAccountCount === 0 &&
    item.incompleteAccountCount === 0;

  if (isHealthy) {
    return <span className={shared.muted}>—</span>;
  }

  const trigger = (
    <span className={shared.statusGroup}>
      {item.completeness !== "complete" && item.incompleteAccountCount === 0 ? (
        <CompletenessBadge value={item.completeness} silentWhenHealthy={false} />
      ) : null}
      {item.staleAccountCount > 0 ? (
        <Badge tone="warn">{item.staleAccountCount} stale</Badge>
      ) : null}
      {item.incompleteAccountCount > 0 ? (
        <Badge tone="bad">{item.incompleteAccountCount} incomplete</Badge>
      ) : null}
    </span>
  );

  return (
    <HoverDetails name={`Issues for valuation point at ${item.evaluatedAt}`} label={trigger}>
      <div className={shared.metaStack}>
        <div>
          <span className={shared.cellMeta}>Completeness</span>
          <div>
            <CompletenessBadge value={item.completeness} silentWhenHealthy={false} />
          </div>
        </div>
        <div>
          <span className={shared.cellMeta}>Accounts summary</span>
          <p style={{ margin: "4px 0 0" }}>
            {item.accountCount} total
            {item.staleAccountCount > 0 ? ` · ${item.staleAccountCount} stale` : ""}
            {item.incompleteAccountCount > 0 ? ` · ${item.incompleteAccountCount} incomplete` : ""}
          </p>
        </div>
      </div>
    </HoverDetails>
  );
}

export function rawPointRow(
  item: ValuationPointSummary,
  previousItem: ValuationPointSummary | undefined,
  timezone: string | undefined,
  selectedId: string,
  update: (next: Record<string, string>) => void,
): DataRow {
  const open = selectedId === item.cycleId;
  const evaluatedAt = formatInstant(item.evaluatedAt, timezone);
  return {
    id: item.cycleId,
    evaluatedAt,
    totalUsd: <NumericText>{formatAmount(item.totalUsd)}</NumericText>,
    change: pointChangeCell(item, previousItem),
    issues: rawPointIssuesCell(item),
    actions: (
      <span className={shared.rowActions}>
        <IconButton
          label={`${open ? "Collapse" : "Expand"} the valuation point at ${evaluatedAt}`}
          aria-expanded={open}
          aria-controls={open ? POINT_DETAIL_ID : undefined}
          onClick={() => update({ point: open ? "" : item.cycleId })}
        >
          <ChevronRight
            size={iconSize.sm}
            aria-hidden="true"
            className={open ? shared.chevronOpen : shared.chevron}
          />
        </IconButton>
      </span>
    ),
  };
}
