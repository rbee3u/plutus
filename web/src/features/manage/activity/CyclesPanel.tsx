import { ChevronRight } from "lucide-react";
import { getCollectionCycle, keys, listCollectionCycles } from "@/api";
import type { CollectionCycle, CollectionCycleDetail, CollectionCycleList } from "@/api";
import { iconSize } from "@/shared/icons/iconSizes";
import { formatBoundaryFlows, formatUsd, formatCount } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { shortId } from "@/shared/lib/id";
import { accountSummary, capitalize } from "@/shared/lib/text";
import { usePlatforms } from "@/shared/lib/usePlatforms";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { ACCOUNT_VALUATION_COLUMNS, accountValuationRow } from "@/shared/lib/valuationRows";
import { CompletenessBadge, StatusBadge } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import { IconButton } from "@/shared/ui/IconButton";
import { Codes, IssueList } from "@/shared/ui/IssueList";
import { NumericText } from "@/shared/ui/NumericText";
import { CursorPager } from "@/shared/ui/Pager";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { useCursorPages } from "../hooks/useCursorPages";
import shared from "@/shared/styles/page.module.css";

const PAGE_SIZE = 25;
const CYCLE_DETAIL_ID = "collection-cycle-detail";

export function CyclesPanel({
  selectedId,
  timezone,
  onSelect,
}: {
  selectedId: string;
  timezone?: string;
  onSelect: (cycleId: string) => void;
}) {
  const pages = useCursorPages();
  const query = usePortfolioQuery<CollectionCycleList>({
    key: (userId, portfolioId) => keys.collectionCycles(userId, portfolioId, pages.cursor),
    load: (portfolioId) =>
      listCollectionCycles({
        path: { portfolioId },
        query: { limit: PAGE_SIZE, cursor: pages.cursor || undefined },
      }),
  });

  return (
    <>
      <QueryRecovery query={query} fallback="Collection cycles could not be loaded." />
      <DataTable
        caption="Collection cycles, newest first"
        columns={[
          { key: "createdAt", header: "Created", nowrap: true },
          { key: "status", header: "Status" },
          { key: "trigger", header: "Trigger" },
          { key: "finishedAt", header: "Finished", nowrap: true },
          { key: "point", header: "Valuation point" },
          { key: "actions", header: "" },
        ]}
        rowClassName={(row) => (row.id === selectedId ? shared.openRow : undefined)}
        openKey={selectedId}
        onRowClick={(key) => onSelect(key === selectedId ? "" : key)}
        detailId={CYCLE_DETAIL_ID}
        renderDetail={() => <CycleDetail cycleId={selectedId} timezone={timezone} />}
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
      <span className={shared.cellStack}>
        <span>{formatInstant(item.createdAt, timezone)}</span>
        <span className={shared.cellMeta}>#{shortId(item.id)}</span>
      </span>
    ),
    status: (
      <span className={shared.cellStack}>
        <span>
          <StatusBadge value={item.status} />
        </span>
        {item.lastErrorClass ? (
          <span className={shared.cellStack} style={{ marginTop: "2px" }}>
            <Codes values={[item.lastErrorClass]} />
            {item.lastDetail ? (
              <span className={shared.cellMeta} style={{ color: "var(--danger)" }}>
                {item.lastDetail}
              </span>
            ) : null}
          </span>
        ) : null}
      </span>
    ),
    trigger: capitalize(item.trigger),
    finishedAt: formatInstant(item.finishedAt, timezone),
    point: item.point ? (
      <span className={shared.cellStack}>
        <span>
          <NumericText>{formatUsd(item.point.totalUsd)}</NumericText>
        </span>
        <span>
          <CompletenessBadge value={item.point.completeness} />
        </span>
      </span>
    ) : (
      "—"
    ),
    actions: (
      <span className={shared.rowActions}>
        <IconButton
          label={`${open ? "Collapse" : "Expand"} the cycle ${shortId(item.id)}`}
          aria-expanded={open}
          aria-controls={open ? CYCLE_DETAIL_ID : undefined}
          onClick={() => onSelect(open ? "" : item.id)}
        >
          <ChevronRight
            size={iconSize.sm}
            aria-hidden="true"
            className={open ? shared.chevronOpen : shared.chevron}
          />
        </IconButton>
      </span>
    ),
  };
}

function CycleDetail({ cycleId, timezone }: { cycleId: string; timezone?: string }) {
  const platforms = usePlatforms();
  const query = usePortfolioQuery<CollectionCycleDetail>({
    key: (userId, portfolioId) => keys.collectionCycle(userId, portfolioId, cycleId),
    ready: Boolean(cycleId),
    load: (portfolioId) => getCollectionCycle({ path: { portfolioId, cycleId } }),
    refetchInterval: (state) => {
      const status = state.state.data?.cycle.status;
      return status === "queued" || status === "running" ? 2000 : false;
    },
  });
  const detail = query.data;
  const cycle = detail?.cycle;

  return (
    <>
      <QueryRecovery query={query} fallback="The collection cycle could not be loaded." />
      {cycle ? (
        <>
          <dl className={shared.metaList}>
            <div>
              <dt>Attempts</dt>
              <dd>
                {formatCount(cycle.attemptCount)}/{formatCount(cycle.maxAttempts)}
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
              <dd>
                <NumericText>{cycle.point ? formatBoundaryFlows(cycle.point) : "—"}</NumericText>
              </dd>
            </div>
          </dl>
          {cycle.lastErrorClass ? (
            <p className={shared.alert}>
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
