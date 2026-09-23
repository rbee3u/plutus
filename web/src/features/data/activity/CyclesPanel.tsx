import { useQuery } from "@tanstack/react-query";
import { ChevronRight } from "lucide-react";
import { getCollectionCycle, listCollectionCycles } from "@/api/generated/sdk.gen";
import type { CollectionCycle } from "@/api/generated/types.gen";
import { useCursorPages } from "@/features/data/activity/useCursorPages";
import { usePlatforms } from "@/features/data/usePlatforms";
import {
  ACCOUNT_VALUATION_COLUMNS,
  accountValuationRow,
} from "@/features/history/ValuationPointsPage";
import { formatBoundaryFlows, formatUsd } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { shortId } from "@/shared/lib/id";
import { accountSummary, capitalize } from "@/shared/lib/text";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { CompletenessBadge, StatusBadge } from "@/shared/ui/Badge";
import { iconSize } from "@/shared/icons/iconSizes";
import { DataTable } from "@/shared/ui/DataTable";
import { IconButton } from "@/shared/ui/IconButton";
import { Codes, IssueList } from "@/shared/ui/IssueList";
import { CursorPager } from "@/shared/ui/Pager";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import styles from "@/features/pages.module.css";

const PAGE_SIZE = 25;
const CYCLE_DETAIL_ID = "collection-cycle-detail";

export function CyclesPanel({
  portfolioId,
  selectedId,
  timezone,
  onSelect,
}: {
  portfolioId: string;
  selectedId: string;
  timezone?: string;
  onSelect: (cycleId: string) => void;
}) {
  const { userId, redirect } = useSessionScope();
  const pages = useCursorPages();
  const query = useQuery({
    queryKey: ["collection-cycles", userId, portfolioId, pages.cursor],
    enabled: Boolean(userId && portfolioId),
    refetchInterval: 5000,
    queryFn: async () =>
      unwrapOrRedirect(
        await listCollectionCycles({
          path: { portfolioId },
          query: { limit: PAGE_SIZE, cursor: pages.cursor || undefined },
        }),
        redirect,
      ),
  });

  return (
    <>
      <p className={styles.lead}>
        Each Collection Cycle collects one Account Report per enabled connection and closes with one
        Valuation Point. Open a cycle to see how each account was valued.
      </p>
      <QueryRecovery
        error={query.error}
        hasData={Boolean(query.data)}
        busy={query.isFetching}
        fallback="Collection cycles could not be loaded."
        onRetry={() => {
          void query.refetch();
        }}
      />
      <DataTable
        caption="Collection cycles, newest first"
        columns={[
          { key: "createdAt", header: "Created", nowrap: true },
          { key: "status", header: "Status" },
          { key: "trigger", header: "Trigger" },
          { key: "finishedAt", header: "Finished", nowrap: true },
          { key: "point", header: "Valuation point" },
          { key: "error", header: "Last error" },
          { key: "actions", header: "" },
        ]}
        rowClassName={(row) => (row.id === selectedId ? styles.openRow : undefined)}
        openKey={selectedId}
        onRowClick={(key) => onSelect(key === selectedId ? "" : key)}
        detailId={CYCLE_DETAIL_ID}
        renderDetail={() => (
          <CycleDetail portfolioId={portfolioId} cycleId={selectedId} timezone={timezone} />
        )}
        rows={(query.data?.items ?? []).map((item) =>
          cycleRow(item, timezone, selectedId, onSelect),
        )}
        empty={
          query.isPending
            ? "Loading collection cycles."
            : "No Collection Cycle yet. Enable a connection and use “Collect now” on the Overview, or wait for the schedule."
        }
      />
      <CursorPager
        canPrev={pages.canPrev}
        hasMore={query.data?.page.hasMore ?? false}
        onPrev={pages.prev}
        onNext={() => pages.next(query.data?.page.nextCursor)}
      />
    </>
  );
}

function cycleRow(
  item: CollectionCycle,
  timezone: string | undefined,
  selectedId: string,
  onSelect: (id: string) => void,
) {
  const open = selectedId === item.id;
  return {
    id: item.id,
    createdAt: (
      <span className={styles.cellStack}>
        <span>{formatInstant(item.createdAt, timezone)}</span>
        <span className={styles.cellMeta}>{shortId(item.id)}</span>
      </span>
    ),
    status: <StatusBadge value={item.status} />,
    trigger: capitalize(item.trigger),
    finishedAt: formatInstant(item.finishedAt, timezone),
    point: item.point ? (
      <span className={styles.cellStack}>
        <span>{formatUsd(item.point.totalUsd)}</span>
        <span>
          <CompletenessBadge value={item.point.completeness} />
        </span>
      </span>
    ) : (
      "—"
    ),
    error: item.lastErrorClass ? (
      <span className={styles.cellStack}>
        <Codes values={[item.lastErrorClass]} />
        {item.lastDetail ? <span className={styles.cellMeta}>{item.lastDetail}</span> : null}
      </span>
    ) : (
      "—"
    ),
    actions: (
      <span className={styles.rowActions}>
        <IconButton
          label={`${open ? "Collapse" : "Expand"} the cycle ${shortId(item.id)}`}
          aria-expanded={open}
          aria-controls={open ? CYCLE_DETAIL_ID : undefined}
          onClick={() => onSelect(open ? "" : item.id)}
        >
          <ChevronRight
            size={iconSize.sm}
            aria-hidden="true"
            className={open ? styles.chevronOpen : styles.chevron}
          />
        </IconButton>
      </span>
    ),
  };
}

function CycleDetail({
  portfolioId,
  cycleId,
  timezone,
}: {
  portfolioId: string;
  cycleId: string;
  timezone?: string;
}) {
  const { userId, redirect } = useSessionScope();
  const platforms = usePlatforms();
  const query = useQuery({
    queryKey: ["collection-cycle", userId, portfolioId, cycleId],
    enabled: Boolean(userId && portfolioId && cycleId),
    refetchInterval: (state) => {
      const status = state.state.data?.cycle.status;
      return status === "queued" || status === "running" ? 2000 : false;
    },
    queryFn: async () =>
      unwrapOrRedirect(await getCollectionCycle({ path: { portfolioId, cycleId } }), redirect),
  });
  const detail = query.data;
  const cycle = detail?.cycle;

  return (
    <>
      <QueryRecovery
        error={query.error}
        hasData={Boolean(detail)}
        busy={query.isFetching}
        fallback="The collection cycle could not be loaded."
        onRetry={() => {
          void query.refetch();
        }}
      />
      {cycle ? (
        <>
          <dl className={styles.metaList}>
            <div>
              <dt>Attempts</dt>
              <dd>
                {String(cycle.attemptCount)}/{String(cycle.maxAttempts)}
              </dd>
            </div>
            <div>
              <dt>Started</dt>
              <dd>{formatInstant(cycle.startedAt, timezone)}</dd>
            </div>
            <div>
              <dt>Accounts</dt>
              <dd>
                {cycle.point
                  ? accountSummary(
                      cycle.point.accountCount,
                      cycle.point.staleAccountCount,
                      cycle.point.incompleteAccountCount,
                    )
                  : "—"}
              </dd>
            </div>
            <div>
              <dt>Boundary flows</dt>
              <dd>{cycle.point ? formatBoundaryFlows(cycle.point) : "—"}</dd>
            </div>
          </dl>
          {cycle.lastErrorClass ? (
            <p className={styles.alert}>
              <Codes values={[cycle.lastErrorClass]} />
              {cycle.lastDetail ? ` ${cycle.lastDetail}` : ""}
            </p>
          ) : null}
          <h3>Accounts</h3>
          <DataTable
            caption="Account valuations of this cycle"
            columns={ACCOUNT_VALUATION_COLUMNS}
            rows={(detail?.accounts ?? []).map((item) =>
              accountValuationRow(item, timezone, platforms.data?.items),
            )}
            empty={
              cycle.status === "queued" || cycle.status === "running"
                ? "The cycle is still running; account valuations appear when it finishes."
                : "No account was valued in this cycle. No connection was enabled, or the cycle failed before collecting."
            }
          />
          {detail && detail.issues.length > 0 ? (
            <>
              <h3>Issues</h3>
              <IssueList issues={detail.issues} />
            </>
          ) : null}
        </>
      ) : null}
    </>
  );
}
