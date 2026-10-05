import type { AccountValuation, BoundaryEvent, Platform } from "@/api";
import { formatAmount, parseDecimal } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { platformLabel } from "@/shared/lib/usePlatforms";
import { CompletenessBadge, DirectionBadge, FreshnessBadge, StatusBadge } from "@/shared/ui/Badge";
import type { DataRow } from "@/shared/ui/DataTable";
import { NumericText } from "@/shared/ui/NumericText";
import styles from "@/shared/styles/page.module.css";

export const ACCOUNT_VALUATION_COLUMNS = [
  { key: "account", header: "Account" },
  { key: "platform", header: "Platform" },
  { key: "valueUsd", header: "Value (USD)", numeric: true },
  { key: "method", header: "Method" },
  { key: "freshness", header: "Freshness" },
  { key: "completeness", header: "Completeness" },
  { key: "status", header: "Status" },
  { key: "error", header: "Error" },
  { key: "reportObservedAt", header: "Report observed", nowrap: true },
];

export function accountValuationRow(
  item: AccountValuation,
  timezone?: string,
  platforms?: Platform[],
): DataRow {
  return {
    id: item.accountId,
    account: (
      <span className={styles.cellStack}>
        <span>{item.accountName}</span>
        <span className={styles.cellMeta}>{item.identityName}</span>
      </span>
    ),
    platform: platformLabel(platforms, item.platform),
    valueUsd: formatAmount(item.valueUsd),
    method: item.method,
    freshness: <FreshnessBadge value={item.freshness} />,
    completeness: <CompletenessBadge value={item.completeness} />,
    status: <StatusBadge value={item.status} silentWhenHealthy />,
    error: item.errorClass
      ? `${item.errorClass}${item.errorDetail ? `: ${item.errorDetail}` : ""}`
      : "—",
    reportObservedAt: formatInstant(item.reportObservedAt, timezone),
  };
}

export const BOUNDARY_COLUMNS = [
  { key: "evaluatedAt", header: "Evaluated at", nowrap: true },
  { key: "account", header: "Account" },
  { key: "platform", header: "Platform" },
  { key: "direction", header: "Direction" },
  { key: "valueUsd", header: "Value (USD)", numeric: true },
  { key: "method", header: "Method" },
];

export function boundaryEventRow(
  item: BoundaryEvent,
  timezone?: string,
  platforms?: Platform[],
): DataRow {
  const dec = parseDecimal(item.valueUsd);
  const isContribution = item.direction.toLowerCase() === "contribution";
  return {
    id: `${item.pointId}-${item.accountId}-${item.direction}`,
    evaluatedAt: formatInstant(item.evaluatedAt, timezone),
    account: (
      <span className={styles.cellStack}>
        <span>{item.accountName}</span>
        {item.identityName ? <span className={styles.cellMeta}>{item.identityName}</span> : null}
      </span>
    ),
    platform: item.platform ? platformLabel(platforms, item.platform) : "—",
    direction: <DirectionBadge value={item.direction} />,
    valueUsd: dec ? (
      <span className={isContribution ? styles.pnlPositive : styles.pnlNegative}>
        <NumericText>{`${isContribution ? "+" : "-"}$${formatAmount(dec.abs().toFixed())}`}</NumericText>
      </span>
    ) : (
      <span className={styles.muted}>—</span>
    ),
    method: <StatusBadge value={item.method} />,
  };
}
