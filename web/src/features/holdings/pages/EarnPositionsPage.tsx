import type { ReactNode } from "react";
import { useParams } from "react-router";
import { keys, listEarnPositions } from "@/api";
import type { EarnPositionList } from "@/api";
import { formatCount } from "@/shared/lib/amount";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { TableEmptyState } from "@/shared/ui/TableEmptyState";
import { EarnPositionsTable } from "../components/EarnPositionsTable";
import {
  HoldingsFilterFooter,
  HoldingsListRecovery,
  HoldingsNoPoint,
} from "../components/HoldingsFilterControls";
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
  let empty: ReactNode | undefined;
  if (query.isError && !data) {
    empty = (
      <TableEmptyState
        title="Earn positions could not be loaded"
        help="Check the notice above to retry the request."
      />
    );
  } else if (!data?.context.cycleId) {
    empty = <HoldingsNoPoint filters={filters} />;
  } else if (!data.detailsComplete) {
    empty = (
      <TableEmptyState
        title="Some earn accounts have no compatible report"
        help="Detailed earn positions could not be resolved. See Issues below."
      />
    );
  } else if (filters.hasFilters) {
    empty = (
      <TableEmptyState
        title="No earn positions match these filters"
        help="Try adjusting your platform, identity, or asset filter."
      />
    );
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
        <IssuesPanel
          issues={data.issues}
          timezone={data.context.timezone}
          resolveScope={filters.resolveIssueScope}
        />
      ) : null}
    </section>
  );
}
