import type { AccountValuation, BoundaryEvent, Issue, Platform } from "@/api";
import { formatAmount, formatCount, parseDecimal } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { platformLabel } from "@/shared/lib/usePlatforms";
import { CompletenessBadge, DirectionBadge } from "@/shared/ui/Badge";
import type { DataRow } from "@/shared/ui/DataTable";
import { NumericText } from "@/shared/ui/NumericText";
import styles from "@/shared/styles/page.module.css";

export function isAccountReportMissing(item: AccountValuation): boolean {
  // Successful sources retain an observation time, even outside the valuation boundary.
  return item.method === "missing" && !item.reportObservedAt;
}

export type AccountBadge = {
  key: string;
  tone: "warn" | "bad";
  label: string;
  title?: string;
};

/**
 * Health labels for a collapsed account-tree node. Supporting explanations stay
 * in the expanded account detail; these are plain labels with hover titles.
 */
export function accountValuationBadges(item: AccountValuation, timezone?: string): AccountBadge[] {
  const missingReport = isAccountReportMissing(item);
  const badges: AccountBadge[] = [];
  if (item.status === "failed") {
    badges.push({
      key: "failed",
      tone: "bad",
      label: "Failed",
      title:
        [item.errorClass, item.errorDetail].filter(Boolean).join(": ") ||
        "Account valuation failed.",
    });
  }
  if (missingReport) {
    badges.push({
      key: "missing",
      tone: "warn",
      label: "Missing report",
      title: "No compatible successful account report; holdings and value are unknown.",
    });
  } else if (item.freshness === "stale") {
    badges.push({
      key: "stale",
      tone: "warn",
      label: "Stale",
      title: item.reportObservedAt
        ? `Earlier successful account report carried forward. Observed ${formatInstant(item.reportObservedAt, timezone)}.`
        : "Earlier successful account report carried forward.",
    });
  }
  if (item.completeness === "incomplete") {
    badges.push({
      key: "incomplete",
      tone: "warn",
      label: "Incomplete",
      title: "Some or all of the account value is unknown.",
    });
  }
  if (item.errorClass && item.status !== "failed") {
    badges.push({
      key: "error",
      tone: "warn",
      label: item.errorClass,
      title: item.errorDetail ?? "Account issue.",
    });
  }
  if (item.omitted.length > 0) {
    badges.push({
      key: "omitted",
      tone: "warn",
      label: `${formatCount(item.omitted.length)} omitted`,
      title: item.omitted.map((om) => `${om.assetSymbol}: ${om.reason}`).join("\n"),
    });
  }
  return badges;
}

/** Resolve cross-account issues from the metadata belonging to the displayed view. */
export function accountIssueScope(
  accounts:
    Pick<AccountValuation, "accountId" | "accountName" | "identityName" | "platform">[] | undefined,
  platforms: Pick<Platform, "id" | "name">[] | undefined,
) {
  const scopes = new Map(
    (accounts ?? []).map((account) => [
      account.accountId,
      [platformLabel(platforms, account.platform), account.identityName, account.accountName]
        .filter(Boolean)
        .join(" · "),
    ]),
  );
  return (issue: Issue): string | undefined => {
    const scope = issue.accountId ? scopes.get(issue.accountId) : undefined;
    return scope ? [scope, issue.symbol].filter(Boolean).join(" · ") : undefined;
  };
}

export type AccountValuationGroup = {
  key: string;
  platform: string;
  platformLabel: string;
  identityId: string;
  identityName: string;
  totalUsd: string | null;
  incomplete: boolean;
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
        incomplete: false,
        accounts: [],
        issuesCount: 0,
      };
      map.set(key, group);
      groups.push(group);
    }
    group.accounts.push(item);
    if (item.completeness === "incomplete" || item.valueUsd == null) {
      group.incomplete = true;
    }
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

export type AccountValuationPlatformGroup = {
  key: string;
  platform: string;
  label: string;
  totalUsd: string | null;
  incomplete: boolean;
  identities: AccountValuationGroup[];
};

/** Platform → identity grouping over the same account order the API returned. */
export function groupAccountValuationsByPlatform(
  items: AccountValuation[],
  platforms?: Platform[],
): AccountValuationPlatformGroup[] {
  const groups: AccountValuationPlatformGroup[] = [];
  const map = new Map<string, AccountValuationPlatformGroup>();

  for (const identity of groupAccountValuations(items, platforms)) {
    let group = map.get(identity.platform);
    if (!group) {
      group = {
        key: identity.platform,
        platform: identity.platform,
        label: identity.platformLabel,
        totalUsd: null,
        incomplete: false,
        identities: [],
      };
      map.set(identity.platform, group);
      groups.push(group);
    }
    group.identities.push(identity);
    if (identity.incomplete) {
      group.incomplete = true;
    }
  }

  for (const group of groups) {
    let sum = parseDecimal("0");
    let hasAnyVal = false;
    for (const identity of group.identities) {
      if (!identity.totalUsd) continue;
      const d = parseDecimal(identity.totalUsd);
      if (!d) continue;
      sum = sum ? sum.plus(d) : d;
      hasAnyVal = true;
    }
    group.totalUsd = hasAnyVal && sum ? sum.toFixed() : null;
  }

  return groups;
}

export const BOUNDARY_COLUMNS = [
  { key: "evaluatedAt", header: "Evaluated at", nowrap: true, width: "170px" },
  { key: "platform", header: "Platform", nowrap: true, width: "110px" },
  { key: "identity", header: "Identity", nowrap: true, width: "130px" },
  { key: "account", header: "Account", nowrap: true, width: "120px" },
  { key: "direction", header: "Direction", width: "110px" },
  { key: "valueUsd", header: "Value (USD)", numeric: true, width: "140px" },
  { key: "issues", header: "Issues", nowrap: true, width: "110px" },
];

/** API cycles are chronological; reverse batches without reversing their accounts. */
export function newestBoundaryEvents(items: BoundaryEvent[]): BoundaryEvent[] {
  const batches = new Map<string, BoundaryEvent[]>();
  for (const item of items) {
    const batch = batches.get(item.cycleId) ?? [];
    batch.push(item);
    batches.set(item.cycleId, batch);
  }
  return [...batches.values()].reverse().flat();
}

export function boundaryEventRow(
  item: BoundaryEvent,
  timezone?: string,
  platforms?: Platform[],
): DataRow {
  const dec = parseDecimal(item.valueUsd);
  return {
    id: `${item.cycleId}-${item.accountId}-${item.direction}`,
    evaluatedAt: (
      <span className={styles.tabularDate}>{formatInstant(item.evaluatedAt, timezone)}</span>
    ),
    platform: item.platform ? (
      platformLabel(platforms, item.platform)
    ) : (
      <span className={styles.muted}>—</span>
    ),
    identity: item.identityName ?? <span className={styles.muted}>—</span>,
    account: item.accountName ?? <span className={styles.muted}>—</span>,
    direction: <DirectionBadge value={item.direction} />,
    valueUsd: dec ? (
      <NumericText>{formatAmount(dec.abs().toFixed())}</NumericText>
    ) : (
      <span className={styles.muted}>—</span>
    ),
    issues:
      item.completeness === "incomplete" ? (
        <CompletenessBadge value={item.completeness} />
      ) : (
        <span className={styles.muted}>—</span>
      ),
  };
}
