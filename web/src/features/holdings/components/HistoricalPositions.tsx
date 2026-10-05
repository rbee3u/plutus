import { keys, listAccountPositions } from "@/api";
import type { AccountPositionList } from "@/api";
import { DataTable } from "@/shared/ui/DataTable";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { POSITION_COLUMNS, positionRow } from "./PositionsTable";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";

export function HistoricalPositions({
  accountId,
  pointId,
}: {
  accountId: string;
  pointId: string;
}) {
  const query = usePortfolioQuery<AccountPositionList>({
    key: (userId, portfolioId) => keys.accountPositions(userId, portfolioId, accountId, pointId),
    ready: Boolean(accountId && pointId),
    load: (portfolioId) =>
      listAccountPositions({ path: { portfolioId, accountId }, query: { pointId } }),
  });
  return (
    <>
      <h4>Contract positions</h4>
      <QueryRecovery
        error={query.error}
        hasData={Boolean(query.data)}
        busy={query.isFetching}
        fallback="Historical positions could not be loaded."
        onRetry={() => {
          void query.refetch();
        }}
      />
      <DataTable
        caption="Contract positions at this historical point"
        columns={POSITION_COLUMNS}
        rows={(query.data?.items ?? []).map((item) =>
          positionRow(item, query.data?.context.timezone),
        )}
        empty={
          query.isPending ? "Loading historical positions." : "No positions in this Account Report."
        }
      />
      {query.data ? <IssuesPanel issues={query.data.issues} empty="No position issues." /> : null}
    </>
  );
}
