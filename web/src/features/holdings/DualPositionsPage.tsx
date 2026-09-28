import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router";
import { listDualPositions } from "@/api/generated/sdk.gen";
import { HoldingsFilters } from "@/features/holdings/HoldingsFilters";
import { DualPositionsTable } from "@/features/holdings/DualPositionsTable";
import { readOffset, useSearchState } from "@/shared/lib/searchParams";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { ContextSummary } from "@/shared/ui/ContextSummary";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { Pager } from "@/shared/ui/Pager";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import styles from "@/features/pages.module.css";

export function DualPositionsPage() {
  const { portfolioId = "" } = useParams();
  const { userId, redirect } = useSessionScope();
  const { params, update } = useSearchState();
  const platform = params.get("platform") ?? "";
  const identityId = params.get("identityId") ?? "";
  const accountId = params.get("accountId") ?? "";
  const symbol = params.get("symbol") ?? "";
  const offset = readOffset(params);
  const query = useQuery({
    queryKey: [
      "dual-positions",
      userId,
      portfolioId,
      platform,
      identityId,
      accountId,
      symbol,
      offset,
    ],
    enabled: Boolean(userId && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(
        await listDualPositions({
          path: { portfolioId },
          query: {
            platform: platform || undefined,
            identityId: identityId || undefined,
            accountId: accountId || undefined,
            symbol: symbol || undefined,
            offset,
            limit: 50,
          },
        }),
        redirect,
      ),
  });
  const data = query.data;
  return (
    <section className={styles.wide}>
      <p className={styles.lead}>
        Outstanding Dual Investment subscriptions at the latest Valuation Point. Their principal is
        already included in Balances; expected yield is not included.
      </p>
      <QueryRecovery
        error={query.error}
        hasData={Boolean(data)}
        busy={query.isFetching}
        fallback="Dual positions could not be loaded."
        onRetry={() => {
          void query.refetch();
        }}
      />
      <HoldingsFilters
        portfolioId={portfolioId}
        values={{ platform, identityId, accountId, symbol }}
        onChange={(next) => update(next)}
      />
      {data ? <ContextSummary context={data.context} /> : null}
      {data || query.isPending ? (
        <DualPositionsTable
          items={data?.items ?? []}
          timezone={data?.context.timezone}
          showAccount
          loading={query.isPending}
        />
      ) : null}
      {data ? (
        <>
          <Pager
            offset={data.page.offset}
            limit={data.page.limit}
            total={data.page.total}
            onChange={(next) => update({ offset: next > 0 ? String(next) : "" })}
          />
          <IssuesPanel issues={data.issues} empty="No issues for these dual positions." />
        </>
      ) : null}
    </section>
  );
}
