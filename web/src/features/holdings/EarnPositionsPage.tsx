import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router";
import { listEarnPositions } from "@/api/generated/sdk.gen";
import { formatCount } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { Pager } from "@/shared/ui/Pager";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { HoldingsFilterFooter, HoldingsFilterRecovery } from "./HoldingsFilterControls";
import { useHoldingsFilters } from "./useHoldingsFilters";
import { EarnPositionsTable, EarnBalanceSummaries } from "./EarnPositionsTable";
import styles from "@/features/pages.module.css";

export function EarnPositionsPage() {
  const { portfolioId = "" } = useParams();
  const { userId, redirect } = useSessionScope();
  const filters = useHoldingsFilters(portfolioId);
  const { platform, identityId, symbol } = filters.values;
  const query = useQuery({
    queryKey: ["earn-positions", userId, portfolioId, platform, identityId, symbol, filters.offset],
    enabled: Boolean(userId && portfolioId && filters.ready),
    queryFn: async () =>
      unwrapOrRedirect(
        await listEarnPositions({
          path: { portfolioId },
          query: {
            platform: platform || undefined,
            identityId: identityId || undefined,
            symbol: symbol || undefined,
            offset: filters.offset,
            limit: 50,
          },
        }),
        redirect,
      ),
  });
  const data = query.data;
  const legacy = Boolean(data?.balanceSummaries.length);
  return (
    <section className={styles.wide}>
      <QueryRecovery
        error={query.error}
        hasData={Boolean(data)}
        busy={query.isFetching}
        fallback="Earn positions could not be loaded."
        onRetry={() => {
          void query.refetch();
        }}
      />
      <HoldingsFilterRecovery filters={filters} />
      {data?.context.pointId ? (
        <div className={styles.holdingsSummaryBar}>
          <div className={styles.holdingsSummaryAmount}>
            <span className={styles.holdingsSummaryLabel}>Positions</span>
            <span className={styles.holdingsSummaryValue}>
              {!data.detailsComplete ? "Details incomplete" : formatCount(data.page.total)}
            </span>
          </div>
          <div className={styles.holdingsSummaryMeta}>
            {legacy && data.page.total > 0 ? (
              <span>
                {formatCount(data.page.total)} known product positions; additional Balance summaries
                below
              </span>
            ) : null}
            <span>
              {data.context.evaluatedAt
                ? `Evaluated ${formatInstant(data.context.evaluatedAt, data.context.timezone)} · ${data.context.timezone}`
                : `Times in ${data.context.timezone}`}
            </span>
          </div>
        </div>
      ) : null}
      <EarnPositionsTable
        items={data?.items ?? []}
        timezone={data?.context.timezone}
        showAccount
        loading={query.isPending}
        filters={filters}
        empty={
          query.isError && !data
            ? "Earn positions could not be loaded. Retry above."
            : !data?.context.pointId
              ? "No Collection Cycle has produced a Valuation Point yet."
              : legacy
                ? "Product details were not collected. Known balances are shown below."
                : !data.detailsComplete
                  ? "Some Earn Accounts have no compatible successful report. See Issues below."
                  : undefined
        }
      />
      <EarnBalanceSummaries
        items={data?.balanceSummaries ?? []}
        timezone={data?.context.timezone}
        showAccount
      />
      <HoldingsFilterFooter filters={filters}>
        {data && (data.detailsComplete || data.page.total > 0) ? (
          <Pager
            offset={data.page.offset}
            limit={data.page.limit}
            total={data.page.total}
            onChange={filters.page}
          />
        ) : null}
      </HoldingsFilterFooter>
      {data?.context.pointId ? (
        <IssuesPanel issues={data.issues} empty="No issues for these Earn positions." />
      ) : null}
    </section>
  );
}
