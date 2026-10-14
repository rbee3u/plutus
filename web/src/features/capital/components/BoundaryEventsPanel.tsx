import { useEffect } from "react";
import { keys, listBoundaryEvents } from "@/api";
import type { BoundaryEventList } from "@/api";
import { formatCount } from "@/shared/lib/amount";
import { isoFromDateInput, normalizeDateInput } from "@/shared/lib/datetime";
import { usePlatforms } from "@/shared/lib/usePlatforms";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { useSearchState } from "@/shared/lib/searchParams";
import { BOUNDARY_COLUMNS, boundaryEventRow } from "@/shared/lib/valuationRows";
import { ActionButton } from "@/shared/ui/ActionButton";
import { DataTable } from "@/shared/ui/DataTable";
import { TableEmptyState } from "@/shared/ui/TableEmptyState";
import { TextInput } from "@/shared/ui/FormControls";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import styles from "../styles.module.css";
import shared from "@/shared/styles/page.module.css";

export function BoundaryEventsPanel({ timezone }: { timezone?: string }) {
  const { params, update } = useSearchState();
  const rawFrom = params.get("from") || params.get("eventsFrom");
  const rawTo = params.get("to") || params.get("eventsTo");
  const from = normalizeDateInput(rawFrom);
  const to = normalizeDateInput(rawTo);

  useEffect(() => {
    const hasLegacyParams = params.has("eventsFrom") || params.has("eventsTo");
    if (
      hasLegacyParams ||
      (rawFrom !== null && rawFrom !== from) ||
      (rawTo !== null && rawTo !== to)
    ) {
      update({ from, to, eventsFrom: "", eventsTo: "" });
    }
  }, [params, rawFrom, rawTo, from, to, update]);

  const platforms = usePlatforms();

  const events = usePortfolioQuery<BoundaryEventList>({
    key: (userId, portfolioId) => keys.boundaryEvents(userId, portfolioId, from, to),
    load: (portfolioId) =>
      listBoundaryEvents({
        path: { portfolioId },
        query: { from: isoFromDateInput(from), to: isoFromDateInput(to, "inclusiveEnd") },
      }),
  });

  const items = events.data?.items ?? [];
  const hasFilter = Boolean(from || to);

  return (
    <>
      <QueryRecovery query={events} fallback="Boundary Events could not be loaded." />
      <div className={`${shared.tableToolbar} ${styles.tableToolbar}`}>
        <div className={`${shared.summaryBar} ${styles.summaryBar}`}>
          <span className={shared.summaryCount}>
            {formatCount(items.length)} {items.length === 1 ? "event" : "events"}
          </span>
          {hasFilter ? (
            <ActionButton
              type="button"
              tone="ghost"
              onClick={() => update({ from: "", to: "", eventsFrom: "", eventsTo: "" })}
            >
              Clear filters
            </ActionButton>
          ) : null}
        </div>
        <div className={`${shared.toolbarActions} ${styles.toolbarActions}`}>
          <form
            className={`${shared.compactFilterBar} ${styles.filterForm}`}
            onSubmit={(event) => event.preventDefault()}
          >
            <label className={`${shared.compactFilterField} ${styles.filterField}`}>
              <span className={shared.compactFilterLabel}>From</span>
              <TextInput
                type="date"
                value={from}
                className={styles.compactDateInput}
                onChange={(event) => update({ from: event.target.value, eventsFrom: "" })}
              />
            </label>
            <label className={`${shared.compactFilterField} ${styles.filterField}`}>
              <span className={shared.compactFilterLabel}>To</span>
              <TextInput
                type="date"
                value={to}
                className={styles.compactDateInput}
                onChange={(event) => update({ to: event.target.value, eventsTo: "" })}
              />
            </label>
          </form>
        </div>
      </div>
      <DataTable
        caption="Boundary Events in the window"
        columns={BOUNDARY_COLUMNS}
        rows={items.map((item) => boundaryEventRow(item, timezone, platforms.data?.items))}
        empty={
          events.isPending ? (
            <TableEmptyState title="Loading Boundary Events..." />
          ) : hasFilter ? (
            <TableEmptyState
              title="No Boundary Events match this date range"
              help="Try adjusting your from or to dates."
              action={
                <ActionButton
                  type="button"
                  tone="secondary"
                  onClick={() => update({ from: "", to: "", eventsFrom: "", eventsTo: "" })}
                >
                  Clear date filter
                </ActionButton>
              }
            />
          ) : (
            <TableEmptyState
              title="No Boundary Events recorded yet"
              help="Boundary events are generated automatically when accounts enter or leave the portfolio."
            />
          )
        }
      />
    </>
  );
}
