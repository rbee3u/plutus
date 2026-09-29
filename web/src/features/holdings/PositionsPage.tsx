import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router";
import { listPositions } from "@/api/generated/sdk.gen";
import type { PositionList } from "@/api/generated/types.gen";
import { platformLabel, usePlatforms } from "@/features/data/usePlatforms";
import { POSITION_COLUMNS, positionRow } from "@/features/holdings/AccountDetail";
import { HoldingsFilters } from "@/features/holdings/HoldingsFilters";
import { formatUsd } from "@/shared/lib/amount";
import { readOffset, useSearchState } from "@/shared/lib/searchParams";
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

function FuturesPositionsPage() {
  const { portfolioId = "" } = useParams();
  const { userId, redirect } = useSessionScope();
  const { params, update } = useSearchState();
  const platforms = usePlatforms();
  const platform = params.get("platform") ?? "";
  const identityId = params.get("identityId") ?? "";
  const accountId = params.get("accountId") ?? "";
  const symbol = params.get("symbol") ?? "";
  const offset = readOffset(params);
  const query = useQuery({
    queryKey: ["positions", userId, portfolioId, platform, identityId, accountId, symbol, offset],
    enabled: Boolean(userId && portfolioId),
    queryFn: async (): Promise<PositionList> =>
      unwrapOrRedirect(
        await listPositions({
          path: { portfolioId },
          query: {
            platform: platform || undefined,
            identityId: identityId || undefined,
            accountId: accountId || undefined,
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
        Open positions inside the boundary at the latest Valuation Point. A position without a mark
        price stays unvalued, never zero.
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
      <HoldingsFilters
        portfolioId={portfolioId}
        values={{ platform, identityId, accountId, symbol }}
        onChange={(next) => update(next)}
      />
      {query.isPending && !data ? <p className={styles.muted}>Loading positions.</p> : null}
      {data && !data.context.pointId ? <p className={styles.notice}>{NO_POINT}</p> : null}
      {data?.context.pointId ? (
        <>
          <ContextSummary context={data.context} />
          <p className={styles.totalRow}>
            <span>Total notional</span>
            <span className={styles.totalValue}>{formatUsd(data.totalNotionalUsd)}</span>
            <CompletenessBadge value={data.completeness} />
          </p>
          <DataTable
            caption="Positions at the latest Valuation Point"
            columns={[
              { key: "platform", header: "Platform", nowrap: true },
              { key: "accountName", header: "Account", nowrap: true },
              ...POSITION_COLUMNS,
            ]}
            rows={data.items.map((item) => ({
              ...positionRow(item),
              platform: platformLabel(platforms.data?.items, item.platform),
              accountName: (
                <span className={styles.cellStack}>
                  <span>{item.accountName}</span>
                  <span className={styles.cellMeta}>{item.identityName}</span>
                </span>
              ),
            }))}
            empty="No positions match these filters at the latest Valuation Point."
          />
          <Pager
            offset={data.page.offset}
            limit={data.page.limit}
            total={data.page.total}
            onChange={(next) => update({ offset: next > 0 ? String(next) : "" })}
          />
          <IssuesPanel issues={data.issues} empty="No issues for these positions." />
        </>
      ) : null}
    </>
  );
}

const POSITION_TYPES = [
  { value: "futures", label: "Futures" },
  { value: "dual_investment", label: "Dual Investment" },
] as const;

export function PositionsPage() {
  const { params, update } = useSearchState();
  const dual = params.get("kind") === "dual_investment";
  const activeTab = dual ? "dual_investment" : "futures";

  return (
    <section className={styles.wide}>
      <div className={styles.viewHeader}>
        <SegmentedControl
          label="Position type"
          value={activeTab}
          onChange={(next) =>
            update({
              kind: next === "dual_investment" ? "dual_investment" : "",
              offset: "",
            })
          }
          options={POSITION_TYPES}
        />
      </div>
      {dual ? <DualPositionsPage /> : <FuturesPositionsPage />}
    </section>
  );
}
