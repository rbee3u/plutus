import { NumericText } from "@/shared/ui/NumericText";
import { useState } from "react";
import type { ValuationPointDetail } from "@/api";
import { formatBoundaryFlows } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { shortId } from "@/shared/lib/id";
import { usePlatforms } from "@/shared/lib/usePlatforms";
import { HistoricalPositions } from "@/features/holdings/display";
import { DataTable } from "@/shared/ui/DataTable";
import { IssueList } from "@/shared/ui/IssueList";
import { AccountValuationsGrouped } from "@/shared/ui/AccountValuationsGrouped";
import { BOUNDARY_COLUMNS, boundaryEventRow } from "@/shared/lib/valuationRows";
import styles from "@/features/history/styles.module.css";
import shared from "@/shared/styles/page.module.css";

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
  return (
    <>
      <dl className={shared.metaList}>
        <div>
          <dt>Boundary flows</dt>
          <dd>
            <NumericText>{formatBoundaryFlows(point)}</NumericText>
          </dd>
        </div>
        <div>
          <dt>Collection Cycle</dt>
          <dd title={point.jobId}>{shortId(point.jobId)}</dd>
        </div>
        {point.recalculatedAt ? (
          <div>
            <dt>Recalculated</dt>
            <dd>{formatInstant(point.recalculatedAt, timezone)}</dd>
          </div>
        ) : null}
      </dl>
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
      <h3>Boundary Events</h3>
      <DataTable
        caption="Boundary Events at this point"
        columns={BOUNDARY_COLUMNS}
        rows={detail.boundaryEvents.map((item) =>
          boundaryEventRow(item, timezone, platforms.data?.items),
        )}
        empty="No account entered or left the boundary at this point."
      />
      {detail.issues.length > 0 ? (
        <>
          <h3>Issues</h3>
          <IssueList issues={detail.issues} />
        </>
      ) : null}
    </>
  );
}
