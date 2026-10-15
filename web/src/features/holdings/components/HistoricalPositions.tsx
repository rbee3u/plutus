import { useValuationRevision } from "@/shared/lib/useValuationRevision";
import { keys, listAccountPositions } from "@/api";
import type { AccountPositionList } from "@/api";
import { DataTable } from "@/shared/ui/DataTable";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { POSITION_COLUMNS, positionRow } from "./PositionsTable";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";

export function HistoricalPositions({
  accountId,
  cycleId,
  valuationRevision,
}: {
  accountId: string;
  cycleId: string;
  valuationRevision: string;
}) {
  const query = usePortfolioQuery<AccountPositionList>({
    key: (userId, portfolioId) =>
      keys.accountPositions(userId, portfolioId, accountId, cycleId, undefined, valuationRevision),
    ready: Boolean(accountId && cycleId),
    load: (portfolioId) =>
      listAccountPositions({ path: { portfolioId, accountId }, query: { cycleId } }),
  });
  const matches = useValuationRevision(valuationRevision, query.data?.context.valuationRevision);
  const data = matches ? query.data : undefined;
  return (
    <>
      <h4>Contract positions</h4>
      <QueryRecovery query={query} fallback="Historical positions could not be loaded." />
      <DataTable
        caption="Contract positions at this historical point"
        columns={POSITION_COLUMNS}
        rows={(data?.items ?? []).map((item) => positionRow(item, data?.context.timezone))}
        empty={
          query.isPending ? "Loading historical positions." : "No positions in this account report."
        }
      />
      {data ? <IssuesPanel issues={data.issues} timezone={data.context.timezone} /> : null}
    </>
  );
}
