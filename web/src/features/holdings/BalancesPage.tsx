import { NumericText } from "@/shared/ui/NumericText";
import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router";
import { listBalances } from "@/api/generated/sdk.gen";
import type { BalanceList, GroupTotal } from "@/api/generated/types.gen";
import { platformLabel, usePlatforms } from "@/features/data/usePlatforms";
import { BALANCE_COLUMNS, balanceRow } from "@/features/holdings/AccountDetail";
import { HoldingsFilters } from "@/features/holdings/HoldingsFilters";
import { formatAmount, formatUsd, formatCount } from "@/shared/lib/amount";
import { readChoice, readOffset, useSearchState } from "@/shared/lib/searchParams";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { capitalize, NO_POINT } from "@/shared/lib/text";
import { CompletenessBadge } from "@/shared/ui/Badge";
import { ContextSummary } from "@/shared/ui/ContextSummary";
import { DataTable, type DataRow } from "@/shared/ui/DataTable";
import { Select } from "@/shared/ui/FormControls";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { Pager } from "@/shared/ui/Pager";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import styles from "@/features/pages.module.css";

const PAGE_SIZE = 50;
const GROUP_OPTIONS = ["", "asset", "platform", "identity", "account"] as const;

export function BalancesPage() {
  const { portfolioId = "" } = useParams();
  const { userId, redirect } = useSessionScope();
  const { params, update } = useSearchState();
  const platforms = usePlatforms();
  const platform = params.get("platform") ?? "";
  const identityId = params.get("identityId") ?? "";
  const accountId = params.get("accountId") ?? "";
  const symbol = params.get("symbol") ?? "";
  const groupBy = readChoice(params.get("groupBy"), GROUP_OPTIONS, "");
  const offset = readOffset(params);
  const query = useQuery({
    queryKey: [
      "balances",
      userId,
      portfolioId,
      platform,
      identityId,
      accountId,
      symbol,
      groupBy,
      offset,
    ],
    enabled: Boolean(userId && portfolioId),
    queryFn: async (): Promise<BalanceList> =>
      unwrapOrRedirect(
        await listBalances({
          path: { portfolioId },
          query: {
            platform: platform || undefined,
            identityId: identityId || undefined,
            accountId: accountId || undefined,
            symbol: symbol || undefined,
            groupBy: groupBy || undefined,
            offset,
            limit: PAGE_SIZE,
          },
        }),
        redirect,
      ),
  });
  const data = query.data;

  return (
    <section className={styles.wide}>
      <p className={styles.lead}>
        Balances inside the boundary at the latest Valuation Point. A balance without a USD price
        stays unpriced, never zero.
      </p>
      <QueryRecovery
        error={query.error}
        hasData={Boolean(data)}
        busy={query.isFetching}
        fallback="Balances could not be loaded."
        onRetry={() => {
          void query.refetch();
        }}
      />
      <HoldingsFilters
        portfolioId={portfolioId}
        values={{ platform, identityId, accountId, symbol }}
        onChange={(next) => update(next)}
      >
        <label className={styles.field}>
          <span>Group by</span>
          <Select value={groupBy} onChange={(event) => update({ groupBy: event.target.value })}>
            {GROUP_OPTIONS.map((option) => (
              <option key={option || "none"} value={option}>
                {option ? capitalize(option) : "No grouping"}
              </option>
            ))}
          </Select>
        </label>
      </HoldingsFilters>
      {query.isPending && !data ? <p className={styles.muted}>Loading balances.</p> : null}
      {data && !data.context.pointId ? <p className={styles.notice}>{NO_POINT}</p> : null}
      {data?.context.pointId ? (
        <>
          <ContextSummary context={data.context} />
          <p className={styles.totalRow}>
            <span>Total</span>
            <span className={styles.totalValue}>
              <NumericText>{formatUsd(data.totalUsd)}</NumericText>
            </span>
            <CompletenessBadge value={data.completeness} />
          </p>
          {groupBy ? (
            <section className={styles.panel}>
              <h2>Groups</h2>
              <DataTable
                caption={`Totals grouped by ${groupBy}`}
                columns={[
                  { key: "label", header: "Group" },
                  { key: "valueUsd", header: "Value (USD)", numeric: true },
                  { key: "accountCount", header: "Accounts", numeric: true },
                  { key: "rowCount", header: "Rows", numeric: true },
                  { key: "completeness", header: "Completeness" },
                ]}
                rows={data.groups.map((group) => groupRow(group))}
                empty="No groups. Groups appear once the latest Valuation Point has balances."
              />
            </section>
          ) : null}
          <DataTable
            caption="Balances at the latest Valuation Point"
            columns={[
              { key: "platform", header: "Platform" },
              { key: "accountName", header: "Account", nowrap: true },
              ...BALANCE_COLUMNS,
            ]}
            rows={data.items.map((item) => ({
              ...balanceRow(item),
              platform: platformLabel(platforms.data?.items, item.platform),
              accountName: (
                <span className={styles.cellStack}>
                  <span>{item.accountName}</span>
                  <span className={styles.cellMeta}>{item.identityName}</span>
                </span>
              ),
            }))}
            empty="No balances match these filters at the latest Valuation Point."
          />
          <Pager
            offset={data.page.offset}
            limit={data.page.limit}
            total={data.page.total}
            onChange={(next) => update({ offset: next > 0 ? String(next) : "" })}
          />
          <IssuesPanel issues={data.issues} empty="No issues for these balances." />
        </>
      ) : null}
    </section>
  );
}

function groupRow(group: GroupTotal): DataRow {
  return {
    id: group.key,
    label: group.label,
    valueUsd: formatAmount(group.valueUsd),
    accountCount: formatCount(group.accountCount),
    rowCount: formatCount(group.rowCount),
    completeness: <CompletenessBadge value={group.completeness} />,
  };
}
