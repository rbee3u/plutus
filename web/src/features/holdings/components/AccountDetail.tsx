import type { AccountValuation } from "@/api";
import { formatAmount, formatDecimal, formatRatio } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { shortId } from "@/shared/lib/id";
import { AccountHealthBadges } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import { Codes, IssueList } from "@/shared/ui/IssueList";
import { NumericText } from "@/shared/ui/NumericText";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import type { ManualTarget } from "../dialogs/manual";
import { useAccountReport } from "../hooks/useAccountReport";
import { BalancesTable } from "./BalancesTable";
import { DualPositionsTable } from "./DualPositionsTable";
import { EarnPositionsTable, EarnBalanceSummaries } from "./EarnPositionsTable";
import { POSITION_COLUMNS, positionRow } from "./PositionsTable";
import styles from "@/features/holdings/styles.module.css";
import shared from "@/shared/styles/page.module.css";

/** One Account's current Account Report: valuation metrics, balances, and positions. */
export function AccountDetail({
  portfolioId,
  account,
  pointId,
  timezone,
  onManual,
}: {
  portfolioId: string;
  account: AccountValuation;
  pointId: string;
  timezone?: string;
  onManual?: (target: ManualTarget) => void;
}) {
  const { balances, positions, dualPositions, earnPositions, isManual, isDual, isEarn } =
    useAccountReport(portfolioId, account, pointId);
  const issues = [
    ...(balances.data?.issues ?? []),
    ...(isDual
      ? (dualPositions.data?.issues ?? [])
      : isEarn
        ? (earnPositions.data?.issues ?? [])
        : (positions.data?.issues ?? [])),
  ];

  const showEarn =
    isEarn &&
    (!earnPositions.isSuccess ||
      earnPositions.data.items.length > 0 ||
      earnPositions.data.balanceSummaries.length > 0 ||
      !earnPositions.data.detailsCollected ||
      earnPositions.isError);
  const showDual =
    isDual &&
    (!dualPositions.isSuccess || dualPositions.data.items.length > 0 || dualPositions.isError);
  const showFutures =
    !isDual &&
    !isEarn &&
    !isManual &&
    (!positions.isSuccess || positions.data.items.length > 0 || positions.isError);

  return (
    <div className={styles.accountDetail}>
      <AccountMetrics account={account} />
      {!isManual || account.reportId ? (
        <AccountFeedback account={account} timezone={timezone} />
      ) : null}
      <QueryRecovery query={balances} fallback="Account balances could not be loaded." />
      <BalancesTable
        account={account}
        data={balances.data}
        isPending={balances.isPending}
        isError={balances.isError}
        timezone={timezone}
        onManual={onManual}
      />
      {showEarn ? (
        <>
          <QueryRecovery query={earnPositions} fallback="Earn positions could not be loaded." />
          <h3 className={styles.accountSectionTitle}>Earn</h3>
          <EarnPositionsTable
            items={earnPositions.data?.items ?? []}
            timezone={earnPositions.data?.context.timezone}
            loading={earnPositions.isPending}
            empty={
              earnPositions.isError
                ? "Earn positions could not be loaded. Retry above."
                : earnPositions.data?.balanceSummaries.length
                  ? "Product details were not collected. Known balances are shown below."
                  : !earnPositions.data?.detailsCollected
                    ? "No successful Earn report with product details is available."
                    : undefined
            }
          />
          <EarnBalanceSummaries
            items={earnPositions.data?.balanceSummaries ?? []}
            timezone={earnPositions.data?.context.timezone}
          />
        </>
      ) : null}
      {showDual ? (
        <>
          <QueryRecovery query={dualPositions} fallback="Dual positions could not be loaded." />
          <h3 className={styles.accountSectionTitle}>Dual Investment</h3>
          {dualPositions.isError && !dualPositions.data ? (
            <p className={shared.muted}>Dual Investment could not be loaded. Retry above.</p>
          ) : (
            <DualPositionsTable
              items={dualPositions.data?.items ?? []}
              timezone={dualPositions.data?.context.timezone}
              loading={dualPositions.isPending}
            />
          )}
        </>
      ) : null}
      {showFutures ? (
        <>
          <QueryRecovery query={positions} fallback="Account positions could not be loaded." />
          <h3 className={styles.accountSectionTitle}>Futures</h3>
          <DataTable
            caption={`Positions of ${account.accountName}`}
            columns={POSITION_COLUMNS}
            rows={(positions.data?.items ?? []).map((item) =>
              positionRow(item, positions.data?.context.timezone),
            )}
            empty={
              positions.isPending
                ? "Loading positions."
                : positions.isError && !positions.data
                  ? "Futures could not be loaded. Retry above."
                  : "No open positions in the current Account Report for this account."
            }
          />
        </>
      ) : null}
      {issues.length > 0 ? (
        <>
          <h3 className={styles.accountSectionTitle}>Issues</h3>
          <IssueList issues={issues} />
        </>
      ) : null}
      <details className={shared.reportDetails}>
        <summary>Report details</summary>
        <dl className={shared.metaList}>
          <div>
            <dt>Account Report</dt>
            <dd title={account.reportId ?? undefined}>{shortId(account.reportId)}</dd>
          </div>
          <div>
            <dt>Valuation method</dt>
            <dd>{account.method}</dd>
          </div>
          <div>
            <dt>Report observed</dt>
            <dd>{formatInstant(account.reportObservedAt, timezone)}</dd>
          </div>
        </dl>
        <p className={shared.muted}>
          {account.method === "reported"
            ? "Account value uses equity reported by the Platform."
            : account.method === "calculated"
              ? "Account value is calculated from Balance equity and prices."
              : "Account value is unknown because no compatible successful Account Report is available."}
        </p>
      </details>
    </div>
  );
}

function AccountMetrics({ account }: { account: AccountValuation }) {
  // Short labels keep the six figures on one line.
  const figures = [
    { label: "Equity", value: account.equity, format: formatAmount },
    {
      label: "AdjEq",
      value: account.adjustedEquity,
      format: formatAmount,
    },
    {
      label: "AvailEq",
      value: account.availableEquity,
      format: formatAmount,
    },
    {
      label: "IMR",
      value: account.initialMarginRatio,
      format: formatRatio,
    },
    {
      label: "MMR",
      value: account.maintenanceMarginRatio,
      format: formatRatio,
    },
    {
      label: "UniMMR",
      value: account.unifiedMarginRatio,
      format: formatRatio,
    },
  ];
  const reported = figures.filter((figure) => figure.value != null);
  if (reported.length === 0) return null;
  return (
    <div className={styles.accountMetricsBar}>
      {reported.map((figure) => (
        <div key={figure.label} className={styles.accountMetricChip}>
          <span className={styles.accountMetricLabel}>{figure.label}</span>
          <span className={styles.accountMetricValue}>
            <NumericText>{figure.format(figure.value)}</NumericText>
          </span>
        </div>
      ))}
    </div>
  );
}

function AccountFeedback({ account, timezone }: { account: AccountValuation; timezone?: string }) {
  const exceptional =
    account.status === "failed" ||
    account.freshness === "stale" ||
    account.completeness === "incomplete";
  return (
    <>
      {exceptional ? (
        <div className={shared.statusGroup}>
          <AccountHealthBadges
            status={account.status}
            freshness={account.freshness}
            completeness={account.completeness}
            errorClass={account.errorClass}
            errorDetail={account.errorDetail}
          />
          {account.freshness === "stale" ? (
            <span className={shared.muted}>
              Earlier successful Account Report retained, observed{" "}
              {formatInstant(account.reportObservedAt, timezone)}
              {timezone ? ` · ${timezone}` : ""}.
            </span>
          ) : account.method === "missing" ? (
            <span className={shared.muted}>
              No compatible successful Account Report; value is unknown.
            </span>
          ) : null}
        </div>
      ) : null}
      {!account.selected ||
      account.identityStatus === "archived" ||
      (account.connectionStatus && account.connectionStatus !== "enabled") ? (
        <p className={shared.notice}>
          {!account.selected ? "Account is not selected. " : ""}
          {account.identityStatus === "archived" ? "Identity is archived. " : ""}
          {account.connectionStatus && account.connectionStatus !== "enabled"
            ? `Connection is ${account.connectionStatus}.`
            : ""}
        </p>
      ) : null}
      {account.errorClass ? (
        <p className={shared.alert} role="status">
          <Codes values={[account.errorClass]} />
          {account.errorDetail ? ` ${account.errorDetail}` : ""}
        </p>
      ) : null}
      {account.omitted.length > 0 ? (
        <p className={shared.notice}>
          Omitted from the total:{" "}
          {account.omitted.map((item, index) => (
            <span key={item.assetSymbol}>
              {index > 0 ? "; " : ""}
              {item.assetSymbol}
              {item.canonicalAssetSymbol ? ` (mapped to ${item.canonicalAssetSymbol})` : ""}{" "}
              <NumericText>{formatDecimal(item.equity)}</NumericText>{" "}
              <Codes values={[item.reason]} />
            </span>
          ))}
        </p>
      ) : null}
    </>
  );
}
