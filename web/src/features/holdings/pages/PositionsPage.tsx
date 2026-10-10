import { useParams } from "react-router";
import { keys, listPositions } from "@/api";
import type { PositionList } from "@/api";
import { formatAmount } from "@/shared/lib/amount";
import { NO_POINT } from "@/shared/lib/text";
import { platformLabel } from "@/shared/lib/usePlatforms";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { CompletenessBadge } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import { IssuesPanel } from "@/shared/ui/IssueList";
import { NumericText } from "@/shared/ui/NumericText";
import { HoldingsFilterFooter, HoldingsListRecovery } from "../components/HoldingsFilterControls";
import { holdingsAccountColumns, HoldingsFilterHeader } from "../components/HoldingsFilterHeader";
import { HoldingsSummary } from "../components/HoldingsSummary";
import { POSITION_COLUMNS, positionRow } from "../components/PositionsTable";
import { useHoldingsFilters } from "../hooks/useHoldingsFilters";
import { holdingsListRequest } from "../lib/listRequest";
import styles from "@/features/holdings/styles.module.css";
import shared from "@/shared/styles/page.module.css";

export function PositionsPage() {
  const { portfolioId = "" } = useParams();
  const filters = useHoldingsFilters(portfolioId);
  const { platform, identityId, symbol } = filters.values;
  const offset = filters.offset;
  const query = usePortfolioQuery<PositionList>({
    key: (userId, portfolioId) =>
      keys.positions(userId, portfolioId, platform, identityId, symbol, offset),
    ready: filters.ready,
    load: (portfolioId) =>
      listPositions({
        path: { portfolioId },
        query: holdingsListRequest(filters),
      }),
  });
  const data = query.data;
  let empty = "No positions match these filters at the latest Valuation Point.";
  if (query.isPending) {
    empty = "Loading positions.";
  } else if (query.isError && !data) {
    empty = "Positions could not be loaded. Retry above.";
  } else if (!data?.context.cycleId) {
    empty = NO_POINT;
  }

  return (
    <section className={shared.wide}>
      <HoldingsListRecovery
        query={query}
        filters={filters}
        fallback="Positions could not be loaded."
      />
      {data?.context.cycleId ? (
        <HoldingsSummary
          context={data.context}
          label="Total Notional"
          value={
            <>
              {data.totalNotionalUsd ? (
                <span className={styles.holdingsSummaryCurrency}>$</span>
              ) : null}
              <NumericText>{formatAmount(data.totalNotionalUsd)}</NumericText>
            </>
          }
          badge={<CompletenessBadge value={data.completeness} />}
        />
      ) : null}
      <DataTable
        caption="Positions at the latest Valuation Point"
        columns={[
          ...holdingsAccountColumns(filters),
          { key: "accountName", header: "Account", nowrap: true },
          ...POSITION_COLUMNS.map((column) =>
            column.key === "symbol"
              ? {
                  ...column,
                  headerContent: (
                    <HoldingsFilterHeader label="Symbol" filterKey="symbol" filters={filters} />
                  ),
                }
              : column,
          ),
        ]}
        rows={(data?.items ?? []).map((item) => ({
          ...positionRow(item, data?.context.timezone),
          platform: platformLabel(filters.platforms.data?.items, item.platform),
          identityName: item.identityName,
          accountName: item.accountName,
        }))}
        empty={empty}
      />
      <HoldingsFilterFooter filters={filters} page={data?.page} />
      {data?.context.cycleId ? (
        <IssuesPanel issues={data.issues} empty="No issues for these positions." />
      ) : null}
    </section>
  );
}
