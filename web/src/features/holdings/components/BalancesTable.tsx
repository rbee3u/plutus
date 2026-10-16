import { Plus } from "lucide-react";
import { ManualActions, type ManualTarget } from "../dialogs/manual";
import { IconButton } from "@/shared/ui/IconButton";
import { iconSize } from "@/shared/icons/iconSizes";
import { NumericText } from "@/shared/ui/NumericText";
import type { AccountReport, AccountValuation, BalanceRow, Issue } from "@/api";
import { formatAmount, formatDecimal, formatPrice, parseDecimal } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { NO_ACCOUNT_REPORT } from "@/shared/lib/text";
import { DataTable, type DataRow } from "@/shared/ui/DataTable";
import { HoverDetails } from "@/shared/ui/HoverDetails";
import { ObservedCell } from "./ObservedCell";
import styles from "@/features/holdings/styles.module.css";
import shared from "@/shared/styles/page.module.css";

/** The balance table of one account, with row actions for manual accounts. */
export function BalancesTable({
  account,
  data,
  isPending,
  isError,
  timezone,
  onManual,
  missingReport,
}: {
  account: AccountValuation;
  data?: AccountReport;
  isPending: boolean;
  isError: boolean;
  timezone?: string;
  onManual?: (target: ManualTarget) => void;
  missingReport: boolean;
}) {
  const isManual = account.accountType === "manual";
  return (
    <>
      <div className={shared.headingRow}>
        <h3 className={styles.accountSectionTitle}>Balances</h3>
        {isManual && onManual ? (
          <IconButton
            label="Add manual balance"
            onClick={() => onManual({ identityId: account.identityId })}
          >
            <Plus size={iconSize.sm} aria-hidden="true" />
          </IconButton>
        ) : null}
      </div>
      {isManual && account.manualModifiedAt ? (
        <p className={shared.muted}>
          Collected quantities last edited {formatInstant(account.manualModifiedAt, timezone)}.
          Collection time is shown below.
        </p>
      ) : null}
      <DataTable
        caption={`Balances of ${account.accountName}`}
        columns={BALANCE_COLUMNS}
        rows={(data?.balances ?? []).map((item) => {
          const row = balanceRow(item, data?.context.timezone, data?.issues);
          if (isManual && onManual) {
            row.symbol = (
              <span className={styles.inlineActionCell}>
                <span>{row.symbol}</span>
                {item.manualChange !== "deleted" ? (
                  <ManualActions
                    identityId={item.identityId}
                    symbol={item.assetSymbol}
                    onOpen={onManual}
                  />
                ) : null}
              </span>
            );
          }
          return row;
        })}
        empty={
          isPending
            ? "Loading balances."
            : isError && !data
              ? "Balances could not be loaded. Retry above."
              : isManual && !account.reportId
                ? "No manual balances entered."
                : missingReport
                  ? NO_ACCOUNT_REPORT
                  : "No balances in the current account report for this account."
        }
      />
    </>
  );
}

export const BALANCE_COLUMNS = [
  { key: "symbol", header: "Asset" },
  { key: "equity", header: "Equity", numeric: true, width: "130px" },
  { key: "priceUsd", header: "Price (USD)", numeric: true, width: "120px" },
  { key: "equityUsd", header: "Equity (USD)", numeric: true, width: "130px" },
  { key: "liability", header: "Liability", numeric: true, width: "120px" },
  { key: "unrealizedPnl", header: "Unrealized PnL", numeric: true, width: "130px" },
  { key: "issues", header: "Issues", nowrap: true, width: "100px" },
];

function balancePnl(value: string | null | undefined) {
  if (value == null) return "—";
  const parsed = parseDecimal(value);
  if (!parsed || parsed.isZero()) return formatDecimal(value);
  const sign = parsed.gt(0) ? "+" : "";
  const className = parsed.gt(0) ? shared.pnlPositive : shared.pnlNegative;
  return (
    <span className={className}>
      <NumericText>{`${sign}${formatDecimal(value)}`}</NumericText>
    </span>
  );
}

export function balanceRow(item: BalanceRow, timezone?: string, issues: Issue[] = []): DataRow {
  const quantity = item.manualChange === "added" ? item.manualQuantity : item.equity;
  const stale = issues.find(
    (issue) => issue.accountId === item.accountId && issue.code === "stale",
  );
  return {
    id: `${item.accountId}-${item.assetId}`,
    symbol:
      item.canonicalAssetSymbol && item.canonicalAssetSymbol !== item.assetSymbol ? (
        <span className={shared.cellStack}>
          <span>{item.assetSymbol}</span>
          <span className={shared.cellMeta}>Canonical asset: {item.canonicalAssetSymbol}</span>
        </span>
      ) : (
        item.assetSymbol
      ),
    equity:
      item.manualChange || item.excluded ? (
        <HoverDetails
          name={`Manual ${item.assetSymbol} quantity ${formatDecimal(quantity)}`}
          label={<NumericText>{formatDecimal(quantity)}</NumericText>}
        >
          {item.manualChange ? (
            <p>
              {item.manualChange === "added" ? (
                "Pending collection"
              ) : item.manualChange === "deleted" ? (
                "Pending removal"
              ) : (
                <>
                  Pending: <NumericText>{formatDecimal(item.manualQuantity)}</NumericText>{" "}
                  {item.assetSymbol}
                </>
              )}
            </p>
          ) : null}
          {item.excluded ? <p>Excluded</p> : null}
        </HoverDetails>
      ) : (
        formatDecimal(quantity)
      ),
    liability: formatDecimal(item.liability),
    unrealizedPnl: balancePnl(item.unrealizedPnl),
    priceUsd: item.noPrice ? (
      <span className={shared.cellMeta}>Missing price</span>
    ) : (
      formatPrice(item.priceUsd)
    ),
    equityUsd: formatAmount(item.equityUsd),
    issues: (
      <ObservedCell
        observedAt={item.reportObservedAt}
        freshness={item.freshness}
        issues={stale ? [stale] : []}
        name={item.assetSymbol}
        timezone={timezone}
      />
    ),
  };
}
