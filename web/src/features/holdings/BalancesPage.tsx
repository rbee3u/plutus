import { useState } from "react";
import { Plus } from "lucide-react";
import { ManualActions, ManualDialog, ManualPending, type ManualTarget } from "./ManualBalances";
import { ActionButton } from "@/shared/ui/ActionButton";
import { iconSize } from "@/shared/icons/iconSizes";
import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router";
import { listBalances } from "@/api/generated/sdk.gen";
import type { BalanceList } from "@/api/generated/types.gen";
import { platformLabel } from "@/features/data/usePlatforms";
import { BALANCE_COLUMNS, balanceRow } from "./AccountDetail";
import { HoldingsFilterHeader } from "./HoldingsFilterHeader";
import { HoldingsFilterFooter, HoldingsFilterRecovery } from "./HoldingsFilterControls";
import { useHoldingsFilters } from "./useHoldingsFilters";
import { formatInstant } from "@/shared/lib/datetime";
import { formatAmount } from "@/shared/lib/amount";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { NO_POINT } from "@/shared/lib/text";
import { CompletenessBadge } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { NumericText } from "@/shared/ui/NumericText";
import { Pager } from "@/shared/ui/Pager";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import styles from "@/features/pages.module.css";

const PAGE_SIZE = 50;

export function BalancesPage() {
  const { portfolioId = "" } = useParams();
  const { userId, redirect } = useSessionScope();
  const filters = useHoldingsFilters(portfolioId);
  const { platform, identityId, symbol } = filters.values;
  const offset = filters.offset;
  const query = useQuery({
    queryKey: ["balances", userId, portfolioId, platform, identityId, symbol, offset],
    enabled: Boolean(userId && portfolioId && filters.ready),
    queryFn: async (): Promise<BalanceList> =>
      unwrapOrRedirect(
        await listBalances({
          path: { portfolioId },
          query: {
            platform: platform || undefined,
            identityId: identityId || undefined,
            symbol: symbol || undefined,
            offset,
            limit: PAGE_SIZE,
          },
        }),
        redirect,
      ),
  });
  const data = query.data;
  const [manualTarget, setManualTarget] = useState<ManualTarget | null>(null);
  return (
    <section className={styles.wide}>
      <div className={styles.headingRow}>
        <h2>Balances</h2>
        <ActionButton tone="secondary" onClick={() => setManualTarget({ identityId })}>
          <Plus size={iconSize.sm} aria-hidden="true" /> Add manual balance
        </ActionButton>
      </div>
      <QueryRecovery
        error={query.error}
        hasData={Boolean(data)}
        busy={query.isFetching}
        fallback="Balances could not be loaded."
        onRetry={() => {
          void query.refetch();
        }}
      />
      <HoldingsFilterRecovery filters={filters} />
      {data?.context.pointId ? (
        <div className={styles.holdingsSummaryBar}>
          <div className={styles.holdingsSummaryAmount}>
            <span className={styles.holdingsSummaryLabel}>Total Balance</span>
            <span className={styles.holdingsSummaryValue}>
              {data.totalUsd ? <span className={styles.holdingsSummaryCurrency}>$</span> : null}
              <NumericText>{formatAmount(data.totalUsd)}</NumericText>
            </span>
            <CompletenessBadge value={data.completeness} />
          </div>
          <div className={styles.holdingsSummaryMeta}>
            <span>
              {data.context.evaluatedAt
                ? `Evaluated ${formatInstant(data.context.evaluatedAt, data.context.timezone)} · ${data.context.timezone}`
                : `Times in ${data.context.timezone}`}
            </span>
          </div>
        </div>
      ) : null}
      <DataTable
        caption="Balances at the latest Valuation Point"
        columns={[
          {
            key: "platform",
            header: "Platform",
            nowrap: true,
            headerContent: (
              <HoldingsFilterHeader label="Platform" filterKey="platform" filters={filters} />
            ),
          },
          {
            key: "identityName",
            header: "Identity",
            nowrap: true,
            headerContent: (
              <HoldingsFilterHeader label="Identity" filterKey="identityId" filters={filters} />
            ),
          },
          { key: "accountName", header: "Account", nowrap: true },
          ...BALANCE_COLUMNS.map((column) =>
            column.key === "symbol"
              ? {
                  ...column,
                  headerContent: (
                    <HoldingsFilterHeader label="Asset" filterKey="symbol" filters={filters} />
                  ),
                }
              : column,
          ),
          ...((data?.items ?? []).some((item) => item.accountType === "manual")
            ? [{ key: "actions", header: "Actions" }]
            : []),
        ]}
        rows={(data?.items ?? []).map((item) => ({
          ...balanceRow(item, data?.context.timezone, data?.issues),
          platform: platformLabel(filters.platforms.data?.items, item.platform),
          identityName: item.identityName,
          accountName: item.accountName,
          actions:
            item.accountType === "manual" ? (
              <ManualActions
                identityId={item.identityId}
                symbol={item.symbol}
                onOpen={setManualTarget}
              />
            ) : undefined,
        }))}
        empty={
          query.isPending
            ? "Loading balances."
            : query.isError && !data
              ? "Balances could not be loaded. Retry above."
              : !data?.context.pointId
                ? NO_POINT
                : "No balances match these filters at the latest Valuation Point."
        }
      />
      <HoldingsFilterFooter filters={filters}>
        {data ? (
          <Pager
            offset={data.page.offset}
            limit={data.page.limit}
            total={data.page.total}
            onChange={filters.page}
          />
        ) : null}
      </HoldingsFilterFooter>
      {filters.identities.data?.items
        .filter(
          (item) =>
            item.status !== "archived" &&
            (!identityId || item.id === identityId) &&
            (!platform || item.platform === platform),
        )
        .map((item) => (
          <ManualPending
            key={item.id}
            portfolioId={portfolioId}
            identityId={item.id}
            name={item.displayName}
            onOpen={setManualTarget}
          />
        ))}
      <ManualDialog
        portfolioId={portfolioId}
        target={manualTarget}
        onClose={() => setManualTarget(null)}
      />
      {data?.context.pointId ? (
        <IssuesPanel issues={data.issues} empty="No issues for these balances." />
      ) : null}
    </section>
  );
}
