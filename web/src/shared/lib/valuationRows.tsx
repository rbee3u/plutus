import type { AccountValuation, BoundaryEvent, Platform } from "@/api";
import { formatAmount, formatCount, parseDecimal } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { platformLabel } from "@/shared/lib/usePlatforms";
import { Badge, DirectionBadge, StatusBadge } from "@/shared/ui/Badge";
import type { DataRow } from "@/shared/ui/DataTable";
import { HoverDetails } from "@/shared/ui/HoverDetails";
import { NumericText } from "@/shared/ui/NumericText";
import styles from "@/shared/styles/page.module.css";

export const ACCOUNT_VALUATION_COLUMNS = [
  { key: "account", header: "Account" },
  { key: "valueUsd", header: "Value (USD)", numeric: true },
  { key: "issues", header: "Issues", nowrap: true },
];

export function accountValuationRow(item: AccountValuation, timezone?: string): DataRow {
  const isHealthy =
    item.status === "ok" &&
    item.freshness === "fresh" &&
    item.completeness === "complete" &&
    !item.errorClass &&
    item.omitted.length === 0;

  return {
    id: item.accountId,
    account: item.accountName,
    valueUsd: item.valueUsd ? formatAmount(item.valueUsd) : "—",
    issues: isHealthy ? (
      <span className={styles.muted}>—</span>
    ) : (
      <span className={styles.statusGroup}>
        {item.status === "failed" ? (
          <HoverDetails
            label={<Badge tone="bad">Failed</Badge>}
            name={`Valuation failure for ${item.accountName}`}
          >
            <p>{item.errorClass ?? "Account valuation failed."}</p>
            {item.errorDetail ? <p className={styles.cellMeta}>{item.errorDetail}</p> : null}
          </HoverDetails>
        ) : null}
        {item.freshness === "stale" ? (
          <HoverDetails
            label={<Badge tone="warn">Stale</Badge>}
            name={`Stale report for ${item.accountName}`}
          >
            <p>Earlier successful Account Report carried forward.</p>
            {item.reportObservedAt ? (
              <p className={styles.cellMeta}>
                Observed {formatInstant(item.reportObservedAt, timezone)}
              </p>
            ) : null}
          </HoverDetails>
        ) : null}
        {item.completeness === "incomplete" ? <Badge tone="warn">Incomplete</Badge> : null}
        {item.errorClass && item.status !== "failed" ? (
          <HoverDetails
            label={<Badge tone="warn">{item.errorClass}</Badge>}
            name={`Issue for ${item.accountName}`}
          >
            {item.errorDetail ? <p>{item.errorDetail}</p> : null}
          </HoverDetails>
        ) : null}
        {item.omitted.length > 0 ? (
          <HoverDetails
            label={<Badge tone="warn">{formatCount(item.omitted.length)} omitted</Badge>}
            name={`Omitted balances for ${item.accountName}`}
          >
            {item.omitted.map((om) => (
              <p key={om.platformAssetSymbol}>
                {om.platformAssetSymbol}: {om.reason}
              </p>
            ))}
          </HoverDetails>
        ) : null}
      </span>
    ),
  };
}

export type AccountValuationGroup = {
  key: string;
  platform: string;
  platformLabel: string;
  identityId: string;
  identityName: string;
  totalUsd: string | null;
  accounts: AccountValuation[];
  issuesCount: number;
};

export function groupAccountValuations(
  items: AccountValuation[],
  platforms?: Platform[],
): AccountValuationGroup[] {
  const groups: AccountValuationGroup[] = [];
  const map = new Map<string, AccountValuationGroup>();

  for (const item of items) {
    const key = `${item.platform}:${item.identityId}`;
    let group = map.get(key);
    if (!group) {
      group = {
        key,
        platform: item.platform,
        platformLabel: platformLabel(platforms, item.platform),
        identityId: item.identityId,
        identityName: item.identityName,
        totalUsd: null,
        accounts: [],
        issuesCount: 0,
      };
      map.set(key, group);
      groups.push(group);
    }
    group.accounts.push(item);
    const hasIssue =
      item.status === "failed" ||
      item.freshness === "stale" ||
      item.completeness === "incomplete" ||
      Boolean(item.errorClass) ||
      item.omitted.length > 0;
    if (hasIssue) {
      group.issuesCount += 1;
    }
  }

  for (const group of groups) {
    let sum = parseDecimal("0");
    let hasAnyVal = false;
    for (const acc of group.accounts) {
      if (acc.valueUsd) {
        const d = parseDecimal(acc.valueUsd);
        if (d) {
          sum = sum ? sum.plus(d) : d;
          hasAnyVal = true;
        }
      }
    }
    group.totalUsd = hasAnyVal && sum ? sum.toFixed() : null;
  }

  return groups;
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
