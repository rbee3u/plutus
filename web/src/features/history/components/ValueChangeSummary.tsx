import { NumericText } from "@/shared/ui/NumericText";
import { getCustomValueChange, keys } from "@/api";
import type { HistoryWindow, ValueChange } from "@/api";
import { combinedFlowsCell, valueChangeCell } from "../lib/valueChangeRows";
import {
  BOUNDARY_COLUMNS,
  boundaryEventRow,
  newestBoundaryEvents,
} from "@/shared/lib/valuationRows";
import { formatUsd } from "@/shared/lib/amount";
import { usePlatforms } from "@/shared/lib/usePlatforms";
import { formatInstant } from "@/shared/lib/datetime";
import { boundaryEventCountText } from "@/shared/lib/text";
import { Badge, CompletenessBadge } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { IssueList } from "@/shared/ui/IssueList";
import styles from "@/features/history/styles.module.css";
import shared from "@/shared/styles/page.module.css";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";

export function ValueChangeSummary({ window }: { window?: HistoryWindow }) {
  const from = window?.from ?? "";
  const to = window?.to ?? "";
  const comparison = usePortfolioQuery<ValueChange>({
    key: (userId, portfolioId) => keys.customValueChange(userId, portfolioId, from, to),
    ready: Boolean(window),
    load: (portfolioId) =>
      getCustomValueChange({
        path: { portfolioId },
        query: { from, to },
      }),
  });

  return (
    <section className={styles.rangeSummary} aria-label="Selected range summary">
      <div className={shared.headingRow}>
        <h2 className={styles.summaryTitle}>Range summary</h2>
        {comparison.data?.completeness === "incomplete" ? (
          <CompletenessBadge value="incomplete" />
        ) : comparison.data?.issues.length ? (
          <Badge tone="warn">Notes</Badge>
        ) : null}
      </div>
      <QueryRecovery query={comparison} fallback="The range summary could not be loaded." />
      {comparison.isPending ? <p className={shared.muted}>Loading range summary.</p> : null}
      {comparison.data ? (
        <RangeResult
          key={`${from}-${to}`}
          change={comparison.data}
          timezone={window?.context.timezone}
        />
      ) : null}
    </section>
  );
}

function RangeResult({ change, timezone }: { change: ValueChange; timezone?: string }) {
  const platforms = usePlatforms();
  const unavailableReasons = [
    !change.startPoint && "No start valuation point at or before the selected start.",
    !change.endPoint && "No end valuation point at or before the selected end.",
    change.startPoint &&
      change.endPoint &&
      (change.startPoint.cycleId === change.endPoint.cycleId || !change.comparable) &&
      "Two distinct valuation points are required for comparison.",
  ].filter(Boolean);
  const unavailable = unavailableReasons.length > 0;
  const unknownFlows = change.flowsCompleteness === "incomplete";
  return (
    <>
      <div className={styles.pointStatsGrid}>
        <div className={styles.pointStatCard}>
          <span className={styles.pointStatLabel}>Value change</span>
          <div className={styles.pointStatValue}>
            {valueChangeCell(unavailable ? null : change.amountUsd, change.startPoint?.totalUsd, {
              adorned: true,
            })}
          </div>
          <span className={styles.pointStatMeta}>
            <span className={shared.nowrap}>{formatInstant(change.from, timezone)}</span> –{" "}
            <span className={shared.nowrap}>{formatInstant(change.to, timezone)}</span>
          </span>
        </div>

        <div className={styles.pointStatCard}>
          <span className={styles.pointStatLabel}>Start point</span>
          <div className={styles.pointStatValue}>
            {change.startPoint?.totalUsd ? (
              <NumericText>{formatUsd(change.startPoint.totalUsd)}</NumericText>
            ) : (
              "—"
            )}
          </div>
          <span className={styles.pointStatMeta}>
            {change.startPoint
              ? `at ${formatInstant(change.startPoint.evaluatedAt, timezone)}`
              : "No start point"}
          </span>
        </div>

        <div className={styles.pointStatCard}>
          <span className={styles.pointStatLabel}>End point</span>
          <div className={styles.pointStatValue}>
            {change.endPoint?.totalUsd ? (
              <NumericText>{formatUsd(change.endPoint.totalUsd)}</NumericText>
            ) : (
              "—"
            )}
          </div>
          <span className={styles.pointStatMeta}>
            {change.endPoint
              ? `at ${formatInstant(change.endPoint.evaluatedAt, timezone)}`
              : "No end point"}
          </span>
        </div>

        <div className={styles.pointStatCard}>
          <span className={styles.pointStatLabel}>Flows</span>
          <div className={styles.pointStatValue}>
            {unavailable ? "—" : combinedFlowsCell(change)}
          </div>
          <span className={styles.pointStatMeta}>
            {unavailable
              ? "Flows unavailable"
              : unknownFlows
                ? "Known flows shown; unvalued amounts: —"
                : boundaryEventCountText(change.boundaryEvents.length)}
          </span>
        </div>
      </div>

      {unavailable ? <p className={shared.notice}>{unavailableReasons.join(" ")}</p> : null}
      {change.issues.length > 0 ? (
        <details className={shared.reportDetails}>
          <summary>Limitations ({change.issues.length})</summary>
          <IssueList issues={change.issues} timezone={timezone} />
        </details>
      ) : null}

      {change.boundaryEvents.length > 0 ? (
        <details className={shared.reportDetails}>
          <summary>Boundary events ({change.boundaryEvents.length})</summary>
          <DataTable
            caption="Boundary events in the range"
            columns={BOUNDARY_COLUMNS}
            rows={newestBoundaryEvents(change.boundaryEvents).map((item) =>
              boundaryEventRow(item, timezone, platforms.data?.items),
            )}
            empty="No account entered or left the boundary in this range."
          />
        </details>
      ) : null}
    </>
  );
}
