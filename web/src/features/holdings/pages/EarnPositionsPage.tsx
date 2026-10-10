import { useParams } from "react-router";
import { keys, listEarnPositions } from "@/api";
import type { EarnPositionList } from "@/api";
import { formatCount } from "@/shared/lib/amount";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { EarnPositionsTable } from "../components/EarnPositionsTable";
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
  let empty: string | undefined;
  if (query.isError && !data) {
    empty = "Earn positions could not be loaded. Retry above.";
  } else if (!data?.context.cycleId) {
    empty = "No Collection Cycle has produced a Valuation Point yet.";
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
      {data?.context.cycleId ? (
        <HoldingsSummary
          context={data.context}
          label="Positions"
          value={!data.detailsComplete ? "Details incomplete" : formatCount(data.page.total)}
        ></HoldingsSummary>
      ) : null}
      <EarnPositionsTable
        items={data?.items ?? []}
        timezone={data?.context.timezone}
        showAccount
        loading={query.isPending}
        filters={filters}
        empty={empty}
      />
      <HoldingsFilterFooter
        filters={filters}
        page={data && (data.detailsComplete || data.page.total > 0) ? data.page : undefined}
      />
      {data?.context.cycleId ? (
        <IssuesPanel issues={data.issues} empty="No issues for these Earn positions." />
      ) : null}
    </section>
  );
}
