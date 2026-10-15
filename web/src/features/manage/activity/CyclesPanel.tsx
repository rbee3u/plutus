import { useMemo } from "react";
import { ChevronRight } from "lucide-react";
import { getCollectionCycle, keys, listCollectionCycles } from "@/api";
import type { CollectionCycle, CollectionCycleDetail, CollectionCycleList } from "@/api";
import { iconSize } from "@/shared/icons/iconSizes";
import { formatAmount, formatBoundaryFlows, formatCount, formatUsd } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { shortId } from "@/shared/lib/id";
import { capitalize } from "@/shared/lib/text";
import { usePlatforms } from "@/shared/lib/usePlatforms";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { AccountValuationsGrouped } from "@/shared/ui/AccountValuationsGrouped";
import { Badge, CompletenessBadge, StatusBadge } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import { IconButton } from "@/shared/ui/IconButton";
import { Codes, IssueList } from "@/shared/ui/IssueList";
import { NumericText } from "@/shared/ui/NumericText";
import { CursorPager } from "@/shared/ui/Pager";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { TableEmptyState } from "@/shared/ui/TableEmptyState";
import { useCursorPages } from "../hooks/useCursorPages";
import manageStyles from "../styles.module.css";
import shared from "@/shared/styles/page.module.css";

const PAGE_SIZE = 25;
const CYCLE_DETAIL_ID = "collection-cycle-detail";

function cycleDuration(item: CollectionCycle): string | null {
  if (!item.finishedAt) return null;
  const start = new Date(item.startedAt || item.createdAt).getTime();
  const end = new Date(item.finishedAt).getTime();
  if (Number.isNaN(start) || Number.isNaN(end) || end < start) return null;
  const diffSec = Math.max(0, Math.round((end - start) / 1000));
  return `${diffSec}s`;
}

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
    refetchInterval: (state) => {
      const items = state.state.data?.items ?? [];
      return items.some((i) => i.status === "queued" || i.status === "running") ? 2000 : false;
    },
  });
  const items = query.data?.items ?? [];

  return (
    <>
      <QueryRecovery query={query} fallback="Collection cycles could not be loaded." />
      <DataTable
        caption="Collection cycles, newest first"
        columns={[
          { key: "time", header: "Time", nowrap: true, width: "170px" },
          { key: "status", header: "Status", width: "130px" },
          { key: "trigger", header: "Trigger", width: "100px" },
          { key: "valuation", header: "Valuation (USD)", numeric: true, width: "160px" },
          { key: "issues", header: "Issues", width: "140px" },
          { key: "actions", header: "", width: "60px" },
        ]}
        rowClassName={(row) => (row.id === selectedId ? shared.openRow : undefined)}
        openKey={selectedId}
        onRowClick={(key) => onSelect(key === selectedId ? "" : key)}
        detailId={CYCLE_DETAIL_ID}
        renderDetail={() => <CycleDetail cycleId={selectedId} timezone={timezone} />}
        rows={items.map((item) => cycleRow(item, timezone, selectedId, onSelect))}
        empty={
          query.isPending ? (
            <TableEmptyState title="Loading collection cycles..." />
          ) : (
            <TableEmptyState title="No collection cycles recorded yet" />
          )
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
  const duration = cycleDuration(item);
  const point = item.point;
  const isHealthy =
    item.status === "succeeded" &&
    !item.errorClass &&
    (!point ||
      (point.completeness === "complete" &&
        point.staleAccountCount === 0 &&
        point.incompleteAccountCount === 0));

  return {
    id: item.id,
    time: (
      <span
        className={shared.cellStack}
        title={
          item.finishedAt ? `Finished: ${formatInstant(item.finishedAt, timezone)}` : "In progress"
        }
      >
        <span>{formatInstant(item.createdAt, timezone)}</span>
        <span className={shared.cellMeta}>#{shortId(item.id)}</span>
      </span>
    ),
    status: (
      <span style={{ display: "inline-flex", alignItems: "center", gap: "var(--space-xs)" }}>
        <StatusBadge value={item.status} />
        {duration ? <span className={shared.cellMeta}>{duration}</span> : null}
      </span>
    ),
    trigger: capitalize(item.trigger),
    valuation: point?.totalUsd ? <NumericText>{formatAmount(point.totalUsd)}</NumericText> : "—",
    issues: isHealthy ? (
      <span className={shared.muted}>—</span>
    ) : (
      <span className={shared.statusGroup}>
        {item.errorClass ? <Codes values={[item.errorClass]} /> : null}
        {item.status === "failed" && !item.errorClass ? <Badge tone="bad">Failed</Badge> : null}
        {point?.completeness &&
        point.completeness !== "complete" &&
        point.incompleteAccountCount === 0 ? (
          <CompletenessBadge value={point.completeness} />
        ) : null}
        {point?.staleAccountCount && point.staleAccountCount > 0 ? (
          <Badge tone="warn">{formatCount(point.staleAccountCount)} stale</Badge>
        ) : null}
        {point?.incompleteAccountCount && point.incompleteAccountCount > 0 ? (
          <Badge tone="warn">{formatCount(point.incompleteAccountCount)} incomplete</Badge>
        ) : null}
      </span>
    ),
    actions: (
      <span className={shared.rowActions}>
        <IconButton
          label={`${open ? "Collapse" : "Expand"} cycle ${shortId(item.id)}`}
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
  const duration = cycle ? cycleDuration(cycle) : null;
  const accountMap = useMemo(() => {
    const map = new Map<string, string>();
    for (const acc of detail?.accounts ?? []) {
      if (acc.accountId && acc.identityName) {
        map.set(acc.accountId, acc.identityName);
      }
    }
    return map;
  }, [detail?.accounts]);

  return (
    <>
      <QueryRecovery query={query} fallback="The collection cycle could not be loaded." />
      {cycle ? (
        <>
          <div className={manageStyles.cycleStatsGrid}>
            <div className={manageStyles.cycleStatCard}>
              <span className={manageStyles.cycleStatLabel}>Valuation</span>
              <span className={manageStyles.cycleStatValue}>
                {cycle.point?.totalUsd ? (
                  <NumericText>{formatUsd(cycle.point.totalUsd)}</NumericText>
                ) : (
                  "—"
                )}
              </span>
              <span className={manageStyles.cycleStatMeta}>
                <span>Flows: </span>
                <NumericText>{cycle.point ? formatBoundaryFlows(cycle.point) : "—"}</NumericText>
                {cycle.point?.completeness && cycle.point.completeness !== "complete" ? (
                  <CompletenessBadge value={cycle.point.completeness} />
                ) : null}
              </span>
            </div>

            <div className={manageStyles.cycleStatCard}>
              <span className={manageStyles.cycleStatLabel}>Accounts</span>
              <span className={manageStyles.cycleStatValue}>
                {cycle.point
                  ? formatCount(cycle.point.accountCount)
                  : formatCount(detail?.accounts.length ?? 0)}
              </span>
              <span className={manageStyles.cycleStatMeta}>
                {cycle.point &&
                (cycle.point.staleAccountCount > 0 || cycle.point.incompleteAccountCount > 0) ? (
                  <span className={shared.statusGroup}>
                    {cycle.point.staleAccountCount > 0 ? (
                      <Badge tone="warn">{formatCount(cycle.point.staleAccountCount)} stale</Badge>
                    ) : null}
                    {cycle.point.incompleteAccountCount > 0 ? (
                      <Badge tone="warn">
                        {formatCount(cycle.point.incompleteAccountCount)} incomplete
                      </Badge>
                    ) : null}
                  </span>
                ) : cycle.point && cycle.status === "succeeded" ? (
                  <span className={shared.pnlPositive}>All healthy</span>
                ) : (
                  <span className={shared.cellMeta}>—</span>
                )}
              </span>
            </div>

            <div className={manageStyles.cycleStatCard}>
              <span className={manageStyles.cycleStatLabel}>Execution</span>
              <span className={manageStyles.cycleStatValue}>
                <StatusBadge value={cycle.status} />
              </span>
              <span className={manageStyles.cycleStatMeta}>
                <span>{capitalize(cycle.trigger)}</span>
                {cycle.attemptCount > 1 ? (
                  <span> · Attempt {formatCount(cycle.attemptCount)}</span>
                ) : null}
              </span>
            </div>

            <div className={manageStyles.cycleStatCard}>
              <span className={manageStyles.cycleStatLabel}>Timing</span>
              <span className={manageStyles.cycleStatValue}>
                {duration ?? (cycle.finishedAt ? "Finished" : "In progress")}
              </span>
              <span className={manageStyles.cycleStatMeta}>
                Started {formatInstant(cycle.startedAt || cycle.createdAt, timezone)}
              </span>
            </div>
          </div>

          {cycle.errorClass ? (
            <p className={shared.alert}>
              <Codes values={[cycle.errorClass]} />
              {cycle.errorDetail ? ` ${cycle.errorDetail}` : ""}
            </p>
          ) : null}
          {detail && detail.issues.length > 0 ? (
            <>
              <div className={manageStyles.cycleSectionHeader}>
                <h3 className={manageStyles.cycleSectionTitle}>
                  Issues ({formatCount(detail.issues.length)})
                </h3>
              </div>
              <IssueList
                issues={detail.issues}
                timezone={timezone}
                resolveScope={(issue) => {
                  const identityName = issue.accountId
                    ? accountMap.get(issue.accountId)
                    : undefined;
                  return [identityName, issue.accountName, issue.symbol]
                    .filter(Boolean)
                    .join(" · ");
                }}
              />
            </>
          ) : null}
          <div className={manageStyles.cycleSectionHeader}>
            <h3 className={manageStyles.cycleSectionTitle}>
              Accounts ({formatCount(detail?.accounts.length ?? 0)})
            </h3>
          </div>
          <AccountValuationsGrouped
            accounts={detail?.accounts ?? []}
            timezone={timezone}
            platforms={platforms.data?.items}
            empty={
              cycle.status === "queued" || cycle.status === "running"
                ? "The cycle is still running; account valuations appear when it finishes."
                : cycle.point?.detailState === "expired"
                  ? "Detailed account valuations for this cycle have expired. Its total and issues remain available."
                  : "No account was valued in this cycle. No connection was enabled, or the cycle failed before collecting."
            }
          />
        </>
      ) : null}
    </>
  );
}
