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
import { formatUsd } from "@/shared/lib/amount";
import { useSearchState } from "@/shared/lib/searchParams";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { NO_POINT } from "@/shared/lib/text";
import { SegmentedControl } from "@/shared/ui/SegmentedControl";
import { CompletenessBadge } from "@/shared/ui/Badge";
import { ContextSummary } from "@/shared/ui/ContextSummary";
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
      <p className={styles.lead}>
        Open positions inside the boundary at the latest Valuation Point. Unknown Size or USD
        notional stays unknown. Total notional covers all positions in the boundary and identifies
        omissions.
      </p>
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
      {data && !data.context.pointId ? <p className={styles.notice}>{NO_POINT}</p> : null}
      {data?.context.pointId ? (
        <>
          <ContextSummary context={data.context} />
          <p className={styles.totalRow}>
            <span>Total notional</span>
            <span className={styles.totalValue}>
              <NumericText>{formatUsd(data.totalNotionalUsd)}</NumericText>
            </span>
            <CompletenessBadge value={data.completeness} />
          </p>
        </>
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
      {data ? <IssuesPanel issues={data.issues} empty="No issues for these positions." /> : null}
    </>
  );
}

const POSITION_TYPES = [
  { value: "futures", label: "Futures" },
  { value: "dual_investment", label: "Dual Investment" },
] as const;

export function PositionsPage() {
  const { params } = useSearchState();
  const { portfolioId = "" } = useParams();
  const filters = useHoldingsFilters(portfolioId);
  const dual = params.get("kind") === "dual_investment";
  const activeTab = dual ? "dual_investment" : "futures";

  return (
    <section className={styles.wide}>
      <div className={styles.viewHeader}>
        <SegmentedControl
          label="Position type"
          value={activeTab}
          onChange={filters.switchKind}
          options={POSITION_TYPES}
        />
      </div>
      {dual ? <DualPositionsPage filters={filters} /> : <FuturesPositionsPage filters={filters} />}
    </section>
  );
}
