import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router";
import { listDualPositions } from "@/api/generated/sdk.gen";
import { HoldingsFilterFooter, HoldingsFilterRecovery } from "./HoldingsFilterControls";
import type { HoldingsFilters } from "./useHoldingsFilters";
import { DualPositionsTable } from "@/features/holdings/DualPositionsTable";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { ContextSummary } from "@/shared/ui/ContextSummary";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { Pager } from "@/shared/ui/Pager";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import styles from "@/features/pages.module.css";

export function DualPositionsPage({ filters }: { filters: HoldingsFilters }) {
  const { portfolioId = "" } = useParams();
  const { userId, redirect } = useSessionScope();
  const { platform, identityId, symbol } = filters.values;
  const offset = filters.offset;
  const query = useQuery({
    queryKey: ["dual-positions", userId, portfolioId, platform, identityId, symbol, offset],
    enabled: Boolean(userId && portfolioId && filters.ready),
    queryFn: async () =>
      unwrapOrRedirect(
        await listDualPositions({
          path: { portfolioId },
          query: {
            platform: platform || undefined,
            identityId: identityId || undefined,
            symbol: symbol || undefined,
            offset,
            limit: 50,
          },
        }),
        redirect,
      ),
  });
  const data = query.data;
  return (
    <>
      <p className={styles.lead}>
        Outstanding Dual Investment subscriptions at the latest Valuation Point. Their principal is
        already included in Balances; expected yield is not included.
      </p>
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
      {data ? <ContextSummary context={data.context} /> : null}
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
          <IssuesPanel issues={data.issues} empty="No issues for these dual positions." />
        </>
      ) : (
        <HoldingsFilterFooter filters={filters} />
      )}
    </>
  );
}
