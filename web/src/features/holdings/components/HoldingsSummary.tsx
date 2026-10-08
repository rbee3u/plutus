import type { ReactNode } from "react";
import type { ReadContext } from "@/api";
import { formatInstant } from "@/shared/lib/datetime";
import styles from "../styles.module.css";

/** Products supply their own value and completeness semantics. */
export function HoldingsSummary({
  context,
  label,
  value,
  badge,
  children,
}: {
  context: ReadContext;
  label: string;
  value: ReactNode;
  badge?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <div className={styles.holdingsSummaryBar}>
      <div className={styles.holdingsSummaryAmount}>
        <span className={styles.holdingsSummaryLabel}>{label}</span>
        <span className={styles.holdingsSummaryValue}>{value}</span>
        {badge}
      </div>
      <div className={styles.holdingsSummaryMeta}>
        {children}
        <span>
          {context.evaluatedAt
            ? `Evaluated ${formatInstant(context.evaluatedAt, context.timezone)} · ${context.timezone}`
            : `Times in ${context.timezone}`}
        </span>
      </div>
    </div>
  );
}
