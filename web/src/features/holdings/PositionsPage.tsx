import { NumericText } from "@/shared/ui/NumericText";
import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router";
import { listPositions } from "@/api/generated/sdk.gen";
import type { PositionList } from "@/api/generated/types.gen";
import { platformLabel } from "@/features/data/usePlatforms";
import { POSITION_COLUMNS, positionRow } from "@/features/holdings/PositionsTable";
import { HoldingsFilterHeader } from "./HoldingsFilterHeader";
import { HoldingsFilterFooter, HoldingsFilterRecovery } from "./HoldingsFilterControls";
import { useHoldingsFilters, type HoldingsFilters } from "./useHoldingsFilters";
import { formatInstant } from "@/shared/lib/datetime";
import { formatAmount } from "@/shared/lib/amount";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { NO_POINT } from "@/shared/lib/text";
import { CompletenessBadge } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { Pager } from "@/shared/ui/Pager";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { DualPositionsPage } from "@/features/holdings/DualPositionsPage";
import styles from "@/features/pages.module.css";

const PAGE_SIZE = 50;

function FuturesPositionsPage({ filters }: { filters: HoldingsFilters }) {
  const { portfolioId = "" } = useParams();
  const { userId, redirect } = useSessionScope();
  const platforms = filters.platforms;
  const { platform, identityId, symbol } = filters.values;
  const offset = filters.offset;
  const query = useQuery({
    queryKey: ["positions", userId, portfolioId, platform, identityId, symbol, offset],
    enabled: Boolean(userId && portfolioId && filters.ready),
    queryFn: async (): Promise<PositionList> =>
      unwrapOrRedirect(
        await listPositions({
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
    <>
      <QueryRecovery
        error={query.error}
        hasData={Boolean(data)}
        busy={query.isFetching}
        fallback="Positions could not be loaded."
        onRetry={() => {
          void query.refetch();
        }}
      />
      <HoldingsFilterRecovery filters={filters} />
      {data?.context.pointId ? (
        <div className={styles.holdingsSummaryBar}>
          <div className={styles.holdingsSummaryAmount}>
            <span className={styles.holdingsSummaryLabel}>Total Notional</span>
            <span className={styles.holdingsSummaryValue}>
              {data.totalNotionalUsd ? (
                <span className={styles.holdingsSummaryCurrency}>$</span>
              ) : null}
              <NumericText>{formatAmount(data.totalNotionalUsd)}</NumericText>
              {data.totalNotionalUsd ? (
                <span className={styles.holdingsSummaryUnit}>USD</span>
              ) : null}
            </span>
            <CompletenessBadge value={data.completeness} />
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
      <DataTable
        caption="Positions at the latest Valuation Point"
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
          ...POSITION_COLUMNS.map((column) =>
            column.key === "symbol"
              ? {
                  ...column,
                  headerContent: (
                    <HoldingsFilterHeader label="Symbol" filterKey="symbol" filters={filters} />
                  ),
                }
              : column,
          ),
        ]}
        rows={(data?.items ?? []).map((item) => ({
          ...positionRow(item, data?.context.timezone),
          platform: platformLabel(platforms.data?.items, item.platform),
          identityName: item.identityName,
          accountName: item.accountName,
        }))}
        empty={
          query.isPending
            ? "Loading positions."
            : query.isError && !data
              ? "Positions could not be loaded. Retry above."
              : !data?.context.pointId
                ? NO_POINT
                : "No positions match these filters at the latest Valuation Point."
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
      {data?.context.pointId ? (
        <IssuesPanel issues={data.issues} empty="No issues for these positions." />
      ) : null}
    </>
  );
}

export function PositionsPage({ dual = false }: { dual?: boolean }) {
  const { portfolioId = "" } = useParams();
  const filters = useHoldingsFilters(portfolioId);
  return (
    <section className={styles.wide}>
      {dual ? <DualPositionsPage filters={filters} /> : <FuturesPositionsPage filters={filters} />}
    </section>
  );
}
