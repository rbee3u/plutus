import { useEffect } from "react";
import { Pager } from "@/shared/ui/Pager";
import { readOffset } from "@/shared/lib/searchParams";
import { keys, listBoundaryEvents } from "@/api";
import type { BoundaryEventList } from "@/api";
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
  const timeWindow = useTimeWindow({
    defaultPreset: "1W",
    onSelectPreset: () => ({ offset: "" }),
    onApply: () => ({ offset: "" }),
  });
  const window = timeWindow.window;
  const offset = readOffset(timeWindow.params);
  const from = window?.from;
  const to = window?.to;

  const platforms = usePlatforms();

  const events = usePortfolioQuery<BoundaryEventList>({
    key: (userId, portfolioId) => keys.boundaryEvents(userId, portfolioId, from, to, offset),
    ready: Boolean(from && to),
    load: (portfolioId) =>
      listBoundaryEvents({
        path: { portfolioId },
        query: { from: from!, to: to!, offset, limit: 50 },
      }),
  });

  useEffect(() => {
    const total = events.data?.page.total;
    if (total !== undefined && offset > 0 && offset >= total) {
      timeWindow.update({ offset: String(Math.max(0, Math.ceil(total / 50) - 1) * 50) });
    }
  }, [events.data?.page.total, offset, timeWindow]);

  const items = events.data?.items ?? [];

  return (
    <>
      <QueryRecovery query={events} fallback="Boundary events could not be loaded." />
      <TimeWindowFilters state={timeWindow} ariaLabel="Boundary event time range" />
      <div className={`${shared.tableToolbar} ${styles.tableToolbar}`}>
        <div className={`${shared.summaryBar} ${styles.summaryBar}`}>
          <span className={shared.summaryCount}>
            {formatCount(events.data?.page.total ?? 0)}{" "}
            {events.data?.page.total === 1 ? "event" : "events"}
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
      {events.data ? (
        <Pager
          offset={offset}
          limit={events.data.page.limit}
          total={events.data.page.total}
          busy={events.isFetching}
          onChange={(next) => timeWindow.update({ offset: next ? String(next) : "" })}
        />
      ) : null}
    </>
  );
}
