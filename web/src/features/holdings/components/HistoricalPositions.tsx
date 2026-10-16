import { keys, getAccountReport } from "@/api";
import type { AccountReport } from "@/api";
import { DataTable } from "@/shared/ui/DataTable";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { POSITION_COLUMNS, positionRow } from "./PositionsTable";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";

export function HistoricalPositions({
  accountId,
  cycleId,
}: {
  accountId: string;
  cycleId: string;
}) {
  const query = usePortfolioQuery<AccountReport>({
    key: (userId, portfolioId) =>
      keys.accountReport(userId, portfolioId, accountId, cycleId, false),
    ready: Boolean(accountId && cycleId),
    load: (portfolioId) =>
      getAccountReport({ path: { portfolioId, accountId }, query: { cycleId } }),
  });
  const matches = query.data?.context.cycleId === cycleId;
  const data = matches ? query.data : undefined;
  return (
    <>
      <h4>Futures</h4>
      <QueryRecovery query={query} fallback="Historical positions could not be loaded." />
      <DataTable
        caption="Futures positions at this valuation point"
        columns={POSITION_COLUMNS}
        rows={(data?.positions ?? []).map((item) => positionRow(item, data?.context.timezone))}
        empty={
          query.isError && !data
            ? "Historical positions could not be loaded."
            : !data
              ? "Loading historical positions."
              : data.account.method === "missing"
                ? "No compatible successful account report is available; positions are unknown."
                : "No positions in this account report."
        }
      />
      {data ? (
        <IssuesPanel
          issues={data.issues.filter((issue) => !issue.omittedBalance)}
          timezone={data.context.timezone}
        />
      ) : null}
    </>
  );
}
