import { useParams } from "react-router";
import { keys, listEarnPositions } from "@/api";
import type { EarnPositionList } from "@/api";
import { formatCount } from "@/shared/lib/amount";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { EarnPositionsTable, EarnBalanceSummaries } from "../components/EarnPositionsTable";
import { HoldingsFilterFooter, HoldingsListRecovery } from "../components/HoldingsFilterControls";
import { HoldingsSummary } from "../components/HoldingsSummary";
import { useHoldingsFilters } from "../hooks/useHoldingsFilters";
import { holdingsListRequest } from "../lib/listRequest";
import shared from "@/shared/styles/page.module.css";

export function EarnPositionsPage() {
  const { portfolioId = "" } = useParams();
  const filters = useHoldingsFilters(portfolioId);
  const { platform, identityId, symbol } = filters.values;
  const offset = filters.offset;
  const query = usePortfolioQuery<EarnPositionList>({
    key: (userId, portfolioId) =>
      keys.earnPositions(userId, portfolioId, platform, identityId, symbol, offset),
    ready: filters.ready,
    load: (portfolioId) =>
      listEarnPositions({
        path: { portfolioId },
        query: holdingsListRequest(filters),
      }),
  });
  const data = query.data;
  const hasBalanceSummaries = Boolean(data?.balanceSummaries.length);
  let empty: string | undefined;
  if (query.isError && !data) {
    empty = "Earn positions could not be loaded. Retry above.";
  } else if (!data?.context.pointId) {
    empty = "No Collection Cycle has produced a Valuation Point yet.";
  } else if (hasBalanceSummaries) {
    empty = "Product details were not collected. Known balances are shown below.";
  } else if (!data.detailsComplete) {
    empty = "Some Earn Accounts have no compatible successful report. See Issues below.";
  }
  return (
    <section className={shared.wide}>
      <HoldingsListRecovery
        query={query}
        filters={filters}
        fallback="Earn positions could not be loaded."
      />
      {data?.context.pointId ? (
        <HoldingsSummary
          context={data.context}
          label="Positions"
          value={!data.detailsComplete ? "Details incomplete" : formatCount(data.page.total)}
        >
          {hasBalanceSummaries && data.page.total > 0 ? (
            <span>
              {formatCount(data.page.total)} known product positions; additional Balance summaries
              below
            </span>
          ) : null}
        </HoldingsSummary>
      ) : null}
      <EarnPositionsTable
        items={data?.items ?? []}
        timezone={data?.context.timezone}
        showAccount
        loading={query.isPending}
        filters={filters}
        empty={empty}
      />
      <EarnBalanceSummaries
        items={data?.balanceSummaries ?? []}
        timezone={data?.context.timezone}
        showAccount
      />
      <HoldingsFilterFooter
        filters={filters}
        page={data && (data.detailsComplete || data.page.total > 0) ? data.page : undefined}
      />
      {data?.context.pointId ? (
        <IssuesPanel issues={data.issues} empty="No issues for these Earn positions." />
      ) : null}
    </section>
  );
}
