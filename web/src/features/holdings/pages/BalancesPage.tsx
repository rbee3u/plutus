import { Plus } from "lucide-react";
import { useState } from "react";
import { useParams } from "react-router";
import { keys, listBalances } from "@/api";
import type { BalanceList } from "@/api";
import { iconSize } from "@/shared/icons/iconSizes";
import { formatAmount } from "@/shared/lib/amount";
import { NO_POINT } from "@/shared/lib/text";
import { platformLabel } from "@/shared/lib/usePlatforms";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { ActionButton } from "@/shared/ui/ActionButton";
import { CompletenessBadge } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { NumericText } from "@/shared/ui/NumericText";
import { BALANCE_COLUMNS, balanceRow } from "../components/BalancesTable";
import { HoldingsFilterFooter, HoldingsListRecovery } from "../components/HoldingsFilterControls";
import { holdingsAccountColumns, HoldingsFilterHeader } from "../components/HoldingsFilterHeader";
import { HoldingsSummary } from "../components/HoldingsSummary";
import { ManualDialog } from "../dialogs/ManualBalances";
import { ManualActions, type ManualTarget } from "../dialogs/manual";
import { useHoldingsFilters } from "../hooks/useHoldingsFilters";
import { holdingsListRequest } from "../lib/listRequest";
import styles from "@/features/holdings/styles.module.css";
import shared from "@/shared/styles/page.module.css";

export function BalancesPage() {
  const { portfolioId = "" } = useParams();
  const filters = useHoldingsFilters(portfolioId);
  const { platform, identityId, symbol } = filters.values;
  const offset = filters.offset;
  const query = usePortfolioQuery<BalanceList>({
    key: (userId, portfolioId) =>
      keys.balances(userId, portfolioId, platform, identityId, symbol, offset),
    ready: filters.ready,
    load: (portfolioId) =>
      listBalances({
        path: { portfolioId },
        query: holdingsListRequest(filters),
      }),
  });
  const data = query.data;
  const [manualTarget, setManualTarget] = useState<ManualTarget | null>(null);
  const addManualButton = (
    <ActionButton tone="secondary" onClick={() => setManualTarget({ identityId })}>
      <Plus size={iconSize.sm} aria-hidden="true" /> Add manual balance
    </ActionButton>
  );

  return (
    <section className={shared.wide}>
      <HoldingsListRecovery
        query={query}
        filters={filters}
        fallback="Balances could not be loaded."
      />
      {data?.context.cycleId ? (
        <HoldingsSummary
          context={data.context}
          label="Collected balance total"
          value={
            <>
              {data.totalUsd ? <span className={styles.holdingsSummaryCurrency}>$</span> : null}
              <NumericText>{formatAmount(data.totalUsd)}</NumericText>
            </>
          }
          badge={<CompletenessBadge value={data.completeness} />}
          action={addManualButton}
        />
      ) : (
        <div className={styles.holdingsActionRow}>{addManualButton}</div>
      )}
      <DataTable
        caption="Balances"
        columns={[
          ...holdingsAccountColumns(filters),
          { key: "accountName", header: "Account", nowrap: true, width: "140px" },
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
        ]}
        rows={(data?.items ?? []).map((item) => ({
          ...balanceRow(item, data?.context.timezone, data?.issues),
          platform: platformLabel(filters.platforms.data?.items, item.platform),
          identityName: item.identityName,
          accountName:
            item.accountType === "manual" ? (
              <span className={styles.inlineActionCell}>
                <span>{item.accountName}</span>
                {item.manualChange !== "deleted" ? (
                  <ManualActions
                    identityId={item.identityId}
                    symbol={item.assetSymbol}
                    onOpen={setManualTarget}
                  />
                ) : null}
              </span>
            ) : (
              item.accountName
            ),
        }))}
        empty={
          query.isPending
            ? "Loading balances."
            : query.isError && !data
              ? "Balances could not be loaded. Retry above."
              : !data?.context.cycleId
                ? NO_POINT
                : "No balances match these filters at the latest Valuation Point."
        }
      />
      <HoldingsFilterFooter filters={filters} page={data?.page} />
      <ManualDialog
        portfolioId={portfolioId}
        target={manualTarget}
        onClose={() => setManualTarget(null)}
      />
      {data?.context.cycleId ? (
        <IssuesPanel issues={data.issues} empty="No issues for these balances." />
      ) : null}
    </section>
  );
}
