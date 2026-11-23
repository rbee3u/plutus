import { Info, Plus } from "lucide-react";
import { ManualActions, type ManualTarget } from "../dialogs/manual";
import { IconButton } from "@/shared/ui/IconButton";
import { iconSize } from "@/shared/icons/iconSizes";
import { NumericText } from "@/shared/ui/NumericText";
import type { AccountReport, AccountValuation, BalanceRow, Issue } from "@/api";
import { formatAmount, formatDecimal, formatPrice, parseDecimal } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { NO_ACCOUNT_REPORT } from "@/shared/lib/text";
import { Badge } from "@/shared/ui/Badge";
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
                  : "No balances in this account report."
        }
      />
    </>
  );
}

const RISK_EXPOSURE_HEADER = (
  <HoverDetails
    name="Risk exposure definition"
    className={styles.riskExposureTrigger}
    label={
      <span className={styles.riskExposureHeader}>
        Risk exposure
        <Info size={iconSize.xs} aria-hidden="true" />
      </span>
    }
  >
    <p>
      Spot equity plus the signed coin quantity of open linear and inverse contracts in this
      account. Positive is net long; negative is net short. Inverse contracts use their mark price.
    </p>
    <p>
      Funding, earn, dual investment, other accounts, and options are not included. Available for
      supported unified accounts.
    </p>
  </HoverDetails>
);

const RISK_EXPOSURE_UNAVAILABLE: Record<string, string> = {
  not_unified: "Risk exposure is available for unified accounts only.",
  platform_pending: "Risk exposure is not yet available for this platform.",
  unknown_position_inputs:
    "Risk exposure cannot be determined because a position's size or mark price is unavailable.",
};

export const BALANCE_COLUMNS = [
  { key: "symbol", header: "Asset" },
  { key: "equity", header: "Equity", numeric: true, width: "130px" },
  { key: "priceUsd", header: "Price (USD)", numeric: true, width: "120px" },
  { key: "equityUsd", header: "Equity (USD)", numeric: true, width: "130px" },
  { key: "liability", header: "Liability", numeric: true, width: "120px" },
  { key: "unrealizedPnl", header: "Unrealized PnL", numeric: true, width: "130px" },
  {
    key: "riskExposure",
    header: "Risk exposure",
    headerContent: RISK_EXPOSURE_HEADER,
    numeric: true,
    width: "150px",
  },
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

/** Liability already contains the unpaid interest, so the interest is disclosed rather than added. */
function balanceLiability(item: BalanceRow) {
  const interest = parseDecimal(item.interest);
  if (item.liability == null || !interest || interest.isZero()) {
    return formatDecimal(item.liability);
  }
  return (
    <HoverDetails
      name={`${item.assetSymbol} liability ${formatDecimal(item.liability)}`}
      label={<NumericText>{formatDecimal(item.liability)}</NumericText>}
    >
      <p>
        Of which unpaid interest: <NumericText>{formatDecimal(item.interest)}</NumericText>{" "}
        {item.assetSymbol}
      </p>
    </HoverDetails>
  );
}

function symbolCell(item: BalanceRow) {
  const mapped = item.canonicalAssetSymbol && item.canonicalAssetSymbol !== item.assetSymbol;
  if (!mapped && !item.contractOnly) return item.assetSymbol;
  return (
    <span className={shared.cellStack}>
      <span className={styles.assetBadgeRow}>
        <span>{item.assetSymbol}</span>
        {item.contractOnly ? <Badge tone="neutral">Contract only</Badge> : null}
      </span>
      {mapped ? (
        <span className={shared.cellMeta}>Canonical asset: {item.canonicalAssetSymbol}</span>
      ) : null}
    </span>
  );
}

function priceCell(item: BalanceRow) {
  if (item.priceSource === "contract" && item.priceUsd != null) {
    const source = item.priceSourceSymbol ? ` from ${item.priceSourceSymbol}` : "";
    return (
      <HoverDetails
        name={`${item.assetSymbol} contract mark price source`}
        label={<NumericText>{formatPrice(item.priceUsd)}</NumericText>}
      >
        <p>
          No spot price is available. This is the mark price{source}, shown for reference; it is not
          used for valuation.
        </p>
      </HoverDetails>
    );
  }
  if (item.noPrice || item.priceUsd == null) {
    return <span className={shared.cellMeta}>Missing price</span>;
  }
  return formatPrice(item.priceUsd);
}

function riskExposureCell(item: BalanceRow) {
  if (item.riskExposure != null) return formatDecimal(item.riskExposure);
  const reason = item.riskExposureUnavailableReason;
  if (!reason) return "—";
  return (
    <HoverDetails
      name={`${item.assetSymbol} risk exposure unavailable`}
      label={<span className={shared.muted}>—</span>}
    >
      <p>{RISK_EXPOSURE_UNAVAILABLE[reason] ?? "Risk exposure is unavailable."}</p>
    </HoverDetails>
  );
}

export function balanceRow(item: BalanceRow, timezone?: string, issues: Issue[] = []): DataRow {
  const quantity = item.manualChange === "added" ? item.manualQuantity : item.equity;
  const stale = issues.find(
    (issue) => issue.accountId === item.accountId && issue.code === "stale",
  );
  return {
    id: `${item.accountId}-${item.assetId}`,
    symbol: symbolCell(item),
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
    liability: balanceLiability(item),
    unrealizedPnl: balancePnl(item.unrealizedPnl),
    riskExposure: riskExposureCell(item),
    priceUsd: priceCell(item),
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
