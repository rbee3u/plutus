import { keys, listBoundaryEvents } from "@/api";
import type { BoundaryEvent, BoundaryEventList } from "@/api";
import { formatCount } from "@/shared/lib/amount";
import { usePlatforms } from "@/shared/lib/usePlatforms";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { useTimeWindow } from "@/shared/lib/useTimeWindow";
import { BOUNDARY_COLUMNS, boundaryEventRow } from "@/shared/lib/valuationRows";
import { ActionButton } from "@/shared/ui/ActionButton";
import { DataTable } from "@/shared/ui/DataTable";
import { TableEmptyState } from "@/shared/ui/TableEmptyState";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { TimeWindowFilters } from "@/shared/ui/TimeWindowFilters";
import styles from "../styles.module.css";
import shared from "@/shared/styles/page.module.css";

export function BoundaryEventsPanel({ timezone }: { timezone?: string }) {
  const timeWindow = useTimeWindow({ defaultPreset: "1W" });
  const window = timeWindow.window;
  const from = window?.from;
  const to = window?.to;

  const platforms = usePlatforms();

  const events = usePortfolioQuery<BoundaryEventList>({
    key: (userId, portfolioId) => keys.boundaryEvents(userId, portfolioId, from, to),
    ready: Boolean(from && to),
    load: (portfolioId) =>
      listBoundaryEvents({
        path: { portfolioId },
        query: { from: from!, to: to! },
      }),
  });

  // API cycles are chronological; keep each cycle's account order intact.
  const batches = new Map<string, BoundaryEvent[]>();
  for (const item of events.data?.items ?? []) {
    const batch = batches.get(item.cycleId) ?? [];
    batch.push(item);
    batches.set(item.cycleId, batch);
  }
  const items = [...batches.values()].reverse().flat();

  return (
    <>
      <QueryRecovery query={events} fallback="Boundary events could not be loaded." />
      <TimeWindowFilters state={timeWindow} ariaLabel="Boundary event time range" />
      <div className={`${shared.tableToolbar} ${styles.tableToolbar}`}>
        <div className={`${shared.summaryBar} ${styles.summaryBar}`}>
          <span className={shared.summaryCount}>
            {formatCount(items.length)} {items.length === 1 ? "event" : "events"}
          </span>
        </div>
      </div>
      <DataTable
        caption="Boundary events in the window"
        columns={BOUNDARY_COLUMNS}
        rows={items.map((item) => boundaryEventRow(item, timezone, platforms.data?.items))}
        empty={
          events.isPending || timeWindow.busy ? (
            <TableEmptyState title="Loading boundary events..." />
          ) : (
            <TableEmptyState
              title={`No boundary events in ${timeWindow.preset ? `the last ${timeWindow.preset}` : "this window"}`}
              help="Try expanding your time window."
              action={
                timeWindow.preset !== "6M" ? (
                  <ActionButton
                    type="button"
                    tone="secondary"
                    onClick={() => timeWindow.selectPreset("6M")}
                  >
                    View last 6 months (6M)
                  </ActionButton>
                ) : null
              }
            />
          )
        }
      />
    </>
  );
}
