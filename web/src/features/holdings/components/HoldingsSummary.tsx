import type { ReactNode } from "react";
import type { BalanceList, Issue, NullableDecimal, ReadContext } from "@/api";
import { formatUsd } from "@/shared/lib/amount";
import { CompletenessBadge, FreshnessBadge } from "@/shared/ui/Badge";
import { NumericText } from "@/shared/ui/NumericText";
import { formatInstant } from "@/shared/lib/datetime";
import shared from "@/shared/styles/page.module.css";
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
  detail,
  action,
}: {
  context: ReadContext;
  label: string;
  total: Pick<
    BalanceList,
    "filteredTotalUsd" | "filteredTotalCompleteness" | "filteredTotalFreshness"
  >;
  detail?: ReactNode;
  action?: ReactNode;
}) {
  return (
    <div className={styles.holdingsSummaryBar} data-breakdown={detail ? "true" : undefined}>
      <div className={styles.holdingsSummaryMain}>
        <div className={styles.holdingsSummaryAmount}>
          <span className={styles.holdingsSummaryLabel}>{label}</span>
          <span className={styles.holdingsSummaryValue}>
            <NumericText>{formatUsd(total.filteredTotalUsd)}</NumericText>
          </span>
          <CompletenessBadge value={total.filteredTotalCompleteness} />
          <FreshnessBadge value={total.filteredTotalFreshness} />
        </div>
        {detail}
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

function NotionalValue({ value }: { value: NullableDecimal }) {
  if (value == null) {
    return (
      <span className={styles.notionalUnknown}>
        <span aria-hidden="true">—</span>
        <span className={shared.srOnly}>Unknown</span>
      </span>
    );
  }
  return <NumericText>{formatUsd(value)}</NumericText>;
}

/** Futures notional split by contract family. Both figures are absolute USD sums. */
export function NotionalBreakdown({
  linearUsd,
  inverseUsd,
}: {
  linearUsd: NullableDecimal;
  inverseUsd: NullableDecimal;
}) {
  return (
    <dl className={styles.notionalBreakdown} aria-label="Notional by contract type">
      <div>
        <dt>Linear</dt>
        <dd>
          <NotionalValue value={linearUsd} />
        </dd>
      </div>
      <div>
        <dt>Inverse</dt>
        <dd>
          <NotionalValue value={inverseUsd} />
        </dd>
      </div>
    </dl>
  );
}
