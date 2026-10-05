import { keys, listDualPositions } from "@/api";
import type { DualPositionList } from "@/api";
import { HoldingsFilterFooter, HoldingsFilterRecovery } from "../components/HoldingsFilterControls";
import type { HoldingsFilters } from "../hooks/useHoldingsFilters";
import { DualPositionsTable } from "@/features/holdings/components/DualPositionsTable";
import { formatInstant } from "@/shared/lib/datetime";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { Pager } from "@/shared/ui/Pager";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import styles from "@/features/holdings/styles.module.css";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";

export function DualPositionsPage({ filters }: { filters: HoldingsFilters }) {
  const { platform, identityId, symbol } = filters.values;
  const offset = filters.offset;
  const query = usePortfolioQuery<DualPositionList>({
    key: (userId, portfolioId) =>
      keys.dualPositions(userId, portfolioId, platform, identityId, symbol, offset),
    ready: Boolean(filters.ready),
    load: (portfolioId) =>
      listDualPositions({
        path: { portfolioId },
        query: {
          platform: platform || undefined,
          identityId: identityId || undefined,
          symbol: symbol || undefined,
          offset,
          limit: 50,
        },
      }),
  });
  const data = query.data;
  return (
    <>
      <QueryRecovery
        error={query.error}
        hasData={Boolean(data)}
        busy={query.isFetching}
        fallback="Dual positions could not be loaded."
        onRetry={() => {
          void query.refetch();
        }}
      />
      <HoldingsFilterRecovery filters={filters} />
      {data?.context.pointId ? (
        <div className={styles.holdingsSummaryBar}>
          <div className={styles.holdingsSummaryAmount}>
            <span className={styles.holdingsSummaryLabel}>Active Subscriptions</span>
            <span className={styles.holdingsSummaryValue}>
              {data.page.total ?? data.items.length}
            </span>
          </div>
          <div className={styles.holdingsSummaryMeta}>
            <span>
              {data.context.evaluatedAt
                ? `Evaluated ${formatInstant(data.context.evaluatedAt, data.context.timezone)} · ${data.context.timezone}`
                : `Times in ${data.context.timezone}`}
            </span>
          </div>
        </div>
      ) : null}
      <DualPositionsTable
        items={data?.items ?? []}
        timezone={data?.context.timezone}
        showAccount
        loading={query.isPending}
        filters={filters}
      />
      {data ? (
        <>
          <HoldingsFilterFooter filters={filters}>
            <Pager
              offset={data.page.offset}
              limit={data.page.limit}
              total={data.page.total}
              onChange={filters.page}
            />
          </HoldingsFilterFooter>
          {data.context.pointId ? (
            <IssuesPanel issues={data.issues} empty="No issues for these dual positions." />
          ) : null}
        </>
      ) : (
        <HoldingsFilterFooter filters={filters} />
      )}
    </>
  );
}
