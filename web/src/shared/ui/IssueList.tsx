import type { Issue } from "@/api";
import { capitalize, reasonText } from "@/shared/lib/text";
import { formatDecimal } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { Badge } from "@/shared/ui/Badge";
import { NumericText } from "@/shared/ui/NumericText";
import styles from "@/shared/styles/page.module.css";

const ISSUE_LABELS: Record<string, string> = {
  stale: "Stale",
  no_price: "Missing price",
  missing: "Missing report",
};

/** Machine codes (issue codes, reason codes, error classes) in one mono style. */
export function Codes({ values }: { values: string[] }) {
  return (
    <>
      {values.map((value, index) => (
        <span key={value}>
          {index > 0 ? " " : ""}
          <code className={styles.issueCode}>{value}</code>
        </span>
      ))}
    </>
  );
}

/** A page section for issues: a heading and list when there are any, hidden when empty. */
export function IssuesPanel({ issues, timezone }: { issues: Issue[]; timezone?: string }) {
  if (issues.length === 0) {
    return null;
  }
  return (
    <section className={styles.panel}>
      <h2>Issues</h2>
      <IssueList issues={issues} timezone={timezone} />
    </section>
  );
}

export function IssueList({
  issues,
  empty = "No issues for this view.",
  resolveScope,
  timezone,
}: {
  issues: Issue[];
  empty?: string;
  resolveScope?: (issue: Issue) => string | undefined;
  timezone?: string;
}) {
  if (issues.length === 0) {
    return <p className={styles.muted}>{empty}</p>;
  }
  return (
    <ul className={styles.issues}>
      {issues.map((issue, index) => {
        const customScope = resolveScope?.(issue);
        const scope = customScope ?? [issue.accountName, issue.symbol].filter(Boolean).join(" · ");
        return (
          <li key={`${issue.code}-${issue.accountId ?? ""}-${issue.symbol ?? ""}-${String(index)}`}>
            <span>
              <Badge tone="warn">
                {ISSUE_LABELS[issue.code] ?? capitalize(reasonText(issue.code))}
              </Badge>{" "}
              {issue.omittedBalance ? (
                <>
                  <NumericText>{formatDecimal(issue.omittedBalance.equity)}</NumericText>{" "}
                  {issue.omittedBalance.assetSymbol}
                  {issue.omittedBalance.canonicalAssetSymbol
                    ? ` (mapped to ${issue.omittedBalance.canonicalAssetSymbol})`
                    : ""}
                  {": "}
                </>
              ) : null}
              {issue.detail}
              {issue.reportObservedAt
                ? ` Observed at ${formatInstant(issue.reportObservedAt, timezone)}.`
                : ""}
            </span>
            {scope ? <span className={styles.issueMeta}>{scope}</span> : null}
          </li>
        );
      })}
    </ul>
  );
}
