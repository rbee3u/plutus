import type { ReactNode } from "react";
import type { BalanceList, Issue, ReadContext } from "@/api";
import { formatUsd } from "@/shared/lib/amount";
import { CompletenessBadge, FreshnessBadge } from "@/shared/ui/Badge";
import { NumericText } from "@/shared/ui/NumericText";
import { formatInstant } from "@/shared/lib/datetime";
import styles from "../styles.module.css";

export function holdingsIssues(data: { filteredTotalIssues: Issue[]; issues: Issue[] }): Issue[] {
  const key = (issue: Issue) => JSON.stringify([issue.code, issue.accountId, issue.symbol]);
  const totalIssues = data.filteredTotalIssues;
  const seen = new Set(totalIssues.map(key));
  return [...totalIssues, ...data.issues.filter((issue) => !seen.has(key(issue)))];
}

/** All holdings tabs share the same USD and data-state presentation. */
export function HoldingsSummary({
  context,
  label,
  total,
  action,
}: {
  context: ReadContext;
  label: string;
  total: Pick<
    BalanceList,
    "filteredTotalUsd" | "filteredTotalCompleteness" | "filteredTotalFreshness"
  >;
  action?: ReactNode;
}) {
  return (
    <div className={styles.holdingsSummaryBar}>
      <div className={styles.holdingsSummaryAmount}>
        <span className={styles.holdingsSummaryLabel}>{label}</span>
        <span className={styles.holdingsSummaryValue}>
          <NumericText>{formatUsd(total.filteredTotalUsd)}</NumericText>
        </span>
        <CompletenessBadge value={total.filteredTotalCompleteness} />
        <FreshnessBadge value={total.filteredTotalFreshness} />
      </div>
      <div className={styles.holdingsSummaryEnd}>
        <div className={styles.holdingsSummaryMeta}>
          <span>
            {context.evaluatedAt
              ? `Evaluated ${formatInstant(context.evaluatedAt, context.timezone)} · ${context.timezone}`
              : `Times in ${context.timezone}`}
          </span>
        </div>
        {action}
      </div>
    </div>
  );
}
