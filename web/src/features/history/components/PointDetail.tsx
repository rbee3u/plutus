import { NumericText } from "@/shared/ui/NumericText";
import { useState } from "react";
import type { ValuationPointDetail } from "@/api";
import { formatBoundaryFlows, formatUsd } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { shortId } from "@/shared/lib/id";
import { usePlatforms } from "@/shared/lib/usePlatforms";
import { HistoricalPositions } from "@/features/holdings/display";
import { DataTable } from "@/shared/ui/DataTable";
import { IssueList } from "@/shared/ui/IssueList";
import { AccountValuationsGrouped } from "@/shared/ui/AccountValuationsGrouped";
import { BOUNDARY_COLUMNS, boundaryEventRow } from "@/shared/lib/valuationRows";
import styles from "@/features/history/styles.module.css";

export const POINT_DETAIL_ID = "valuation-point-detail";

/** The expanded row of a Raw Valuation Point: flows, accounts, boundary events, and issues. */
export function PointDetail({
  detail,
  timezone,
}: {
  detail: ValuationPointDetail;
  timezone?: string;
}) {
  const [openAccount, setOpenAccount] = useState<string | undefined>();
  const platforms = usePlatforms();
  const point = detail.point;

  const accountsHealthy = point.staleAccountCount === 0 && point.incompleteAccountCount === 0;

  return (
    <>
      <div className={styles.pointStatsGrid}>
        <div className={styles.pointStatCard}>
          <span className={styles.pointStatLabel}>Accounts</span>
          <div className={styles.pointStatValue}>{point.accountCount}</div>
          <span className={styles.pointStatMeta}>
            {accountsHealthy
              ? "All healthy"
              : `${point.staleAccountCount > 0 ? `${point.staleAccountCount} stale` : ""}${
                  point.staleAccountCount > 0 && point.incompleteAccountCount > 0 ? " · " : ""
                }${point.incompleteAccountCount > 0 ? `${point.incompleteAccountCount} incomplete` : ""}`}
          </span>
        </div>

        <div className={styles.pointStatCard}>
          <span className={styles.pointStatLabel}>Valuation Point</span>
          <div className={styles.pointStatValue}>
            <NumericText>{formatUsd(point.totalUsd)}</NumericText>
          </div>
          <span className={styles.pointStatMeta}>
            {point.recalculatedAt
              ? `Recalculated ${formatInstant(point.recalculatedAt, timezone)}`
              : `Evaluated ${formatInstant(point.evaluatedAt, timezone)}`}
          </span>
        </div>

        <div className={styles.pointStatCard}>
          <span className={styles.pointStatLabel}>Boundary Flows</span>
          <div className={styles.pointStatValue}>
            <NumericText>{formatBoundaryFlows(point)}</NumericText>
          </div>
          <span className={styles.pointStatMeta}>
            {detail.boundaryEvents.length === 0
              ? "No boundary events"
              : `${detail.boundaryEvents.length} event(s)`}
          </span>
        </div>

        <div className={styles.pointStatCard}>
          <span className={styles.pointStatLabel}>Collection Cycle</span>
          <div className={styles.pointStatValue} title={point.jobId}>
            {shortId(point.jobId)}
          </div>
          <span className={styles.pointStatMeta}>Job reference</span>
        </div>
      </div>

      <h3>Accounts</h3>
      <AccountValuationsGrouped
        accounts={detail.accounts}
        timezone={timezone}
        platforms={platforms.data?.items}
        openKey={openAccount}
        onRowClick={(key) => setOpenAccount((current) => (current === key ? undefined : key))}
        detailId={`historical-positions-${point.id}`}
        renderDetail={(row) => (
          <HistoricalPositions
            accountId={typeof row.id === "string" ? row.id : ""}
            pointId={point.id}
          />
        )}
        renderAccountAction={(item) => (
          <button
            type="button"
            className={styles.inlineButton}
            aria-expanded={openAccount === item.accountId}
            aria-controls={`historical-positions-${point.id}`}
            onClick={() =>
              setOpenAccount((current) => (current === item.accountId ? undefined : item.accountId))
            }
          >
            {item.accountName}
          </button>
        )}
        empty="No accounts were inside the boundary at this point."
      />

      {detail.boundaryEvents.length > 0 ? (
        <>
          <h3>Boundary Events</h3>
          <DataTable
            caption="Boundary Events at this point"
            columns={BOUNDARY_COLUMNS}
            rows={detail.boundaryEvents.map((item) =>
              boundaryEventRow(item, timezone, platforms.data?.items),
            )}
            empty="No account entered or left the boundary at this point."
          />
        </>
      ) : null}

      {detail.issues.length > 0 ? (
        <>
          <h3>Issues</h3>
          <IssueList issues={detail.issues} />
        </>
      ) : null}
    </>
  );
}
