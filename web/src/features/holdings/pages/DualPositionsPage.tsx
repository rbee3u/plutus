import { useParams } from "react-router";
import { keys, listDualPositions } from "@/api";
import type { DualPositionList } from "@/api";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { TableEmptyState } from "@/shared/ui/TableEmptyState";
import { DualPositionsTable } from "../components/DualPositionsTable";
import { HoldingsFilterFooter, HoldingsListRecovery } from "../components/HoldingsFilterControls";
import { HoldingsSummary } from "../components/HoldingsSummary";
import { useHoldingsFilters } from "../hooks/useHoldingsFilters";
import { holdingsListRequest } from "../lib/listRequest";
import shared from "@/shared/styles/page.module.css";

export function DualPositionsPage() {
  const { portfolioId = "" } = useParams();
  const filters = useHoldingsFilters(portfolioId);
  const { platform, identityId, symbol } = filters.values;
  const offset = filters.offset;
  const query = usePortfolioQuery<DualPositionList>({
    key: (userId, portfolioId) =>
      keys.dualPositions(userId, portfolioId, platform, identityId, symbol, offset),
    ready: filters.ready,
    load: (portfolioId) =>
      listDualPositions({
        path: { portfolioId },
        query: holdingsListRequest(filters),
      }),
  });
  const data = query.data;
  return (
    <section className={shared.wide}>
      <HoldingsListRecovery
        query={query}
        filters={filters}
        fallback="Dual positions could not be loaded."
      />
      {data?.context.cycleId ? (
        <HoldingsSummary
          context={data.context}
          label="Active Subscriptions"
          value={data.page.total ?? data.items.length}
        />
      ) : null}
      <DualPositionsTable
        items={data?.items ?? []}
        timezone={data?.context.timezone}
        showAccount
        loading={query.isPending}
        filters={filters}
        empty={
          query.isError && !data ? (
            <TableEmptyState
              title="Dual Investment subscriptions could not be loaded"
              help="Check the notice above to retry the request."
            />
          ) : !data?.context.cycleId ? (
            <TableEmptyState
              title="No Valuation Point yet"
              help="Run a Collection Cycle to create the first point."
            />
          ) : filters.hasFilters ? (
            <TableEmptyState
              title="No Dual Investment subscriptions match these filters"
              help="Try adjusting your platform, identity, or pair filter."
            />
          ) : undefined
        }
      />
      <HoldingsFilterFooter filters={filters} page={data?.page} />
      {data?.context.cycleId ? <IssuesPanel issues={data.issues} /> : null}
    </section>
  );
}
