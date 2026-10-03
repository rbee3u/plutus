import { useQuery } from "@tanstack/react-query";
import { listAccountPositions } from "@/api/generated/sdk.gen";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { DataTable } from "@/shared/ui/DataTable";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { POSITION_COLUMNS, positionRow } from "./PositionsTable";

export function HistoricalPositions({
  portfolioId,
  accountId,
  pointId,
}: {
  portfolioId: string;
  accountId: string;
  pointId: string;
}) {
  const { userId, redirect } = useSessionScope();
  const query = useQuery({
    queryKey: ["account-positions", userId, portfolioId, accountId, pointId],
    enabled: Boolean(userId && portfolioId && accountId && pointId),
    queryFn: async () =>
      unwrapOrRedirect(
        await listAccountPositions({ path: { portfolioId, accountId }, query: { pointId } }),
        redirect,
      ),
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
