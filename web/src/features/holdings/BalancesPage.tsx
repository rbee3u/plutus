import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router";
import { listBalances } from "@/api/generated/sdk.gen";
import type { BalanceList } from "@/api/generated/types.gen";
import { platformLabel } from "@/features/data/usePlatforms";
import { BALANCE_COLUMNS, balanceRow } from "./AccountDetail";
import { HoldingsFilterHeader } from "./HoldingsFilterHeader";
import { HoldingsFilterFooter, HoldingsFilterRecovery } from "./HoldingsFilterControls";
import { useHoldingsFilters } from "./useHoldingsFilters";
import { formatUsd } from "@/shared/lib/amount";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { NO_POINT } from "@/shared/lib/text";
import { CompletenessBadge } from "@/shared/ui/Badge";
import { ContextSummary } from "@/shared/ui/ContextSummary";
import { DataTable } from "@/shared/ui/DataTable";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { NumericText } from "@/shared/ui/NumericText";
import { Pager } from "@/shared/ui/Pager";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import styles from "@/features/pages.module.css";

const PAGE_SIZE = 50;

export function BalancesPage() {
  const { portfolioId = "" } = useParams();
  const { userId, redirect } = useSessionScope();
  const filters = useHoldingsFilters(portfolioId);
  const { platform, identityId, symbol } = filters.values;
  const offset = filters.offset;
  const query = useQuery({
    queryKey: ["balances", userId, portfolioId, platform, identityId, symbol, offset],
    enabled: Boolean(userId && portfolioId && filters.ready),
    queryFn: async (): Promise<BalanceList> =>
      unwrapOrRedirect(
        await listBalances({
          path: { portfolioId },
          query: {
            platform: platform || undefined,
            identityId: identityId || undefined,
            symbol: symbol || undefined,
            offset,
            limit: PAGE_SIZE,
          },
        }),
        redirect,
      ),
  });
  const data = query.data;
  return (
    <section className={styles.wide}>
      <p className={styles.lead}>
        Balances inside the boundary at the latest Valuation Point. A balance without a USD price
        stays unpriced, never zero.
      </p>
      <QueryRecovery
        error={query.error}
        hasData={Boolean(data)}
        busy={query.isFetching}
        fallback="Balances could not be loaded."
        onRetry={() => {
          void query.refetch();
        }}
      />
      <HoldingsFilterRecovery filters={filters} />
      {data && !data.context.pointId ? <p className={styles.notice}>{NO_POINT}</p> : null}
      {data?.context.pointId ? (
        <>
          <ContextSummary context={data.context} />
          <p className={styles.totalRow}>
            <span>Total</span>
            <span className={styles.totalValue}>
              <NumericText>{formatUsd(data.totalUsd)}</NumericText>
            </span>
            <CompletenessBadge value={data.completeness} />
          </p>
        </>
      ) : null}
      <DataTable
        caption="Balances at the latest Valuation Point"
        columns={[
          {
            key: "platform",
            header: "Platform",
            nowrap: true,
            headerContent: (
              <HoldingsFilterHeader label="Platform" filterKey="platform" filters={filters} />
            ),
          },
          {
            key: "identityName",
            header: "Identity",
            nowrap: true,
            headerContent: (
              <HoldingsFilterHeader label="Identity" filterKey="identityId" filters={filters} />
            ),
          },
          { key: "accountName", header: "Account", nowrap: true },
          ...BALANCE_COLUMNS.map((column) =>
            column.key === "symbol"
              ? {
                  ...column,
                  headerContent: (
                    <HoldingsFilterHeader label="Asset" filterKey="symbol" filters={filters} />
                  ),
                }
              : column,
          ),
        ]}
        rows={(data?.items ?? []).map((item) => ({
          ...balanceRow(item, data?.context.timezone, data?.issues),
          platform: platformLabel(filters.platforms.data?.items, item.platform),
          identityName: item.identityName,
          accountName: item.accountName,
        }))}
        empty={
          query.isPending
            ? "Loading balances."
            : query.isError && !data
              ? "Balances could not be loaded. Retry above."
              : !data?.context.pointId
                ? NO_POINT
                : "No balances match these filters at the latest Valuation Point."
        }
      />
      <HoldingsFilterFooter filters={filters}>
        {data ? (
          <Pager
            offset={data.page.offset}
            limit={data.page.limit}
            total={data.page.total}
            onChange={filters.page}
          />
        ) : null}
      </HoldingsFilterFooter>
      {data ? <IssuesPanel issues={data.issues} empty="No issues for these balances." /> : null}
    </section>
  );
}
