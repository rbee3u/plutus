import { useParams } from "react-router";
import { keys, listDualPositions } from "@/api";
import type { DualPositionList } from "@/api";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { TableEmptyState } from "@/shared/ui/TableEmptyState";
import { DualPositionsTable } from "../components/DualPositionsTable";
import {
  HoldingsFilterFooter,
  HoldingsListRecovery,
  HoldingsNoPoint,
} from "../components/HoldingsFilterControls";
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
      keys.dualPositions(userId, portfolioId, platform, identityId, symbol, offset, filters.readId),
    ready: filters.ready,
    load: (portfolioId) =>
      filters.read((cycleId) =>
        listDualPositions({
          path: { portfolioId },
          query: { ...holdingsListRequest(filters), cycleId },
        }),
      ),
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
          label="Subscriptions"
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
              title="Dual investment subscriptions could not be loaded"
              help="Check the notice above to retry the request."
            />
          ) : !data?.context.cycleId ? (
            <HoldingsNoPoint filters={filters} />
          ) : !data.coverage.complete ? (
            <TableEmptyState
              title="Dual investment data is incomplete"
              help="Some accounts have no compatible successful report."
            />
          ) : filters.hasFilters ? (
            <TableEmptyState
              title="No dual investment subscriptions match these filters"
              help="Try adjusting your platform, identity, or pair filter."
            />
          ) : undefined
        }
      />
      <HoldingsFilterFooter filters={filters} page={data?.page} />
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
