import { useState } from "react";
import type { ExposureSummary, ReadContext } from "@/api";
import { formatCount } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { Badge, CompletenessBadge, FreshnessBadge } from "@/shared/ui/Badge";
import { NumericText } from "@/shared/ui/NumericText";
import { ExposureHowItWorks } from "./ExposureHowItWorks";
import styles from "../styles.module.css";

/** Portfolio-level counts instead of a misleading cross-coin USD total. */
export function ExposureSummaryBar({
  context,
  summary,
  filtered,
}: {
  context: ReadContext;
  summary: ExposureSummary;
  filtered: boolean;
}) {
  const [help, setHelp] = useState(false);
  const counts: string[] = [];
  if (summary.longCount > 0) counts.push(`${formatCount(summary.longCount)} long`);
  if (summary.shortCount > 0) counts.push(`${formatCount(summary.shortCount)} short`);
  if (summary.flatCount > 0) counts.push(`${formatCount(summary.flatCount)} flat`);
  return (
    <>
      <div className={styles.holdingsSummaryBar}>
        <div className={styles.holdingsSummaryAmount}>
          <span className={styles.holdingsSummaryLabel}>
            {filtered ? "FILTERED EXPOSURE" : "PORTFOLIO EXPOSURE"}
          </span>
          <span className={styles.holdingsSummaryValue}>
            <NumericText>{formatCount(summary.assetCount)}</NumericText>
          </span>
          <span className={styles.exposureSummaryUnit}>assets</span>
          <span className={styles.exposureSummaryCounts}>{counts.join(" · ")}</span>
          {summary.partialCount > 0 ? (
            <Badge tone="warn">{formatCount(summary.partialCount)} partial</Badge>
          ) : null}
          {summary.unknownCount > 0 ? (
            <Badge tone="warn">{formatCount(summary.unknownCount)} unknown</Badge>
          ) : null}
          <CompletenessBadge value={summary.completeness} />
          <FreshnessBadge value={summary.freshness} />
        </div>
        <div className={styles.holdingsSummaryEnd}>
          <div className={styles.holdingsSummaryMeta}>
            <span>
              {context.evaluatedAt
                ? `Evaluated ${formatInstant(context.evaluatedAt, context.timezone)} · ${context.timezone}`
                : `Times in ${context.timezone}`}
            </span>
          </div>
          <button type="button" className={styles.exposureHelpButton} onClick={() => setHelp(true)}>
            How it works
          </button>
        </div>
      </div>
      <ExposureHowItWorks open={help} onClose={() => setHelp(false)} />
    </>
  );
}
