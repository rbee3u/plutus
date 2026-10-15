import type { AccountValuation, Issue } from "@/api";
import { formatAmount, formatDecimal, formatRatio } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { shortId } from "@/shared/lib/id";
import { capitalize, NO_ACCOUNT_REPORT } from "@/shared/lib/text";
import { isAccountReportMissing } from "@/shared/lib/valuationRows";
import { AccountHealthBadges } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import { Codes, IssueList } from "@/shared/ui/IssueList";
import { NumericText } from "@/shared/ui/NumericText";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import type { ManualTarget } from "../dialogs/manual";
import { useAccountReport } from "../hooks/useAccountReport";
import { BalancesTable } from "./BalancesTable";
import { DualPositionsTable } from "./DualPositionsTable";
import { EarnPositionsTable } from "./EarnPositionsTable";
import { POSITION_COLUMNS, positionRow } from "./PositionsTable";
import styles from "@/features/holdings/styles.module.css";
import shared from "@/shared/styles/page.module.css";

/** One account's current account report: valuation metrics, balances, and positions. */
export function AccountDetail({
  portfolioId,
  account,
  cycleId,
  valuationRevision,
  timezone,
  onManual,
}: {
  portfolioId: string;
  account: AccountValuation;
  cycleId: string;
  valuationRevision: string | null;
  timezone?: string;
  onManual?: (target: ManualTarget) => void;
}) {
  const { balances, positions, dualPositions, earnPositions, isManual, isDual, isEarn } =
    useAccountReport(portfolioId, account, cycleId, valuationRevision);
  const missingReport = isAccountReportMissing(account);
  const showEarn =
    isEarn &&
    (!earnPositions.isSuccess ||
      (earnPositions.data?.items.length ?? 0) > 0 ||
      (earnPositions.data?.issues.length ?? 0) > 0 ||
      earnPositions.isError);
  const showDual =
    isDual &&
    (!dualPositions.isSuccess ||
      (dualPositions.data?.items.length ?? 0) > 0 ||
      (dualPositions.data?.issues.length ?? 0) > 0 ||
      dualPositions.isError);
  const showFutures =
    !isDual &&
    !isEarn &&
    !isManual &&
    (!positions.isSuccess ||
      (positions.data?.items.length ?? 0) > 0 ||
      (positions.data?.issues.length ?? 0) > 0 ||
      positions.isError);

  return (
    <div className={styles.accountDetail}>
      <AccountMetrics account={account} />
      {!isManual || account.reportId ? (
        <AccountFeedback account={account} timezone={timezone} missingReport={missingReport} />
      ) : null}
      <section className={styles.accountDetail} aria-label="Balances">
        <QueryRecovery query={balances} fallback="Account balances could not be loaded." />
        <BalancesTable
          account={account}
          data={balances.data}
          isPending={balances.isPending}
          isError={balances.isError}
          timezone={timezone}
          onManual={onManual}
          missingReport={missingReport}
        />
        <HoldingIssues issues={balances.data?.issues} timezone={timezone} />
      </section>
      {showEarn ? (
        <section className={styles.accountDetail} aria-label="Earn">
          <QueryRecovery query={earnPositions} fallback="Earn positions could not be loaded." />
          <h3 className={styles.accountSectionTitle}>Earn</h3>
          <EarnPositionsTable
            items={earnPositions.data?.items ?? []}
            timezone={earnPositions.data?.context.timezone}
            loading={earnPositions.isPending}
            empty={
              earnPositions.isError
                ? "Earn positions could not be loaded. Retry above."
                : missingReport
                  ? NO_ACCOUNT_REPORT
                  : undefined
            }
          />
          <HoldingIssues issues={earnPositions.data?.issues} timezone={timezone} />
        </section>
      ) : null}
      {showDual ? (
        <section className={styles.accountDetail} aria-label="Dual investment">
          <QueryRecovery query={dualPositions} fallback="Dual positions could not be loaded." />
          <h3 className={styles.accountSectionTitle}>Dual investment</h3>
          {dualPositions.isError && !dualPositions.data ? (
            <p className={shared.muted}>Dual investment could not be loaded. Retry above.</p>
          ) : (
            <DualPositionsTable
              items={dualPositions.data?.items ?? []}
              timezone={dualPositions.data?.context.timezone}
              loading={dualPositions.isPending}
              empty={missingReport ? NO_ACCOUNT_REPORT : undefined}
            />
          )}
          <HoldingIssues issues={dualPositions.data?.issues} timezone={timezone} />
        </section>
      ) : null}
      {showFutures ? (
        <section className={styles.accountDetail} aria-label="Futures">
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
                  : missingReport
                    ? NO_ACCOUNT_REPORT
                    : "No open positions in the current account report for this account."
            }
          />
          <HoldingIssues issues={positions.data?.issues} timezone={timezone} />
        </section>
      ) : null}
      <details className={shared.reportDetails}>
        <summary>Report details</summary>
        <dl className={shared.metaList}>
          <div>
            <dt>Account report</dt>
            <dd title={account.reportId ?? undefined}>{shortId(account.reportId)}</dd>
          </div>
          <div>
            <dt>Valuation method</dt>
            <dd>{capitalize(account.method)}</dd>
          </div>
          <div>
            <dt>Report observed</dt>
            <dd>{formatInstant(account.reportObservedAt, timezone)}</dd>
          </div>
        </dl>
        <p className={shared.muted}>
          {account.method === "reported"
            ? "Account value uses equity reported by the platform."
            : account.method === "calculated"
              ? "Account value is calculated from balance equity and prices."
              : "Account value is unknown because no compatible successful account report is available."}
        </p>
      </details>
    </div>
  );
}

function HoldingIssues({ issues, timezone }: { issues?: Issue[]; timezone?: string }) {
  if (!issues?.length) return null;
  return (
    <>
      <h4 className={styles.accountSectionTitle}>Issues</h4>
      <IssueList issues={issues} timezone={timezone} />
    </>
  );
}

function AccountMetrics({ account }: { account: AccountValuation }) {
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
      label: marginRatioLabel(account),
      value: account.marginRatio,
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

function marginRatioLabel(account: AccountValuation) {
  if (account.platform === "binance") {
    if (account.accountType === "cross_margin") {
      return "Margin level";
    }
    if (account.accountType === "portfolio_margin") {
      return "UniMMR";
    }
  }
  return "Margin ratio";
}

function AccountFeedback({
  account,
  timezone,
  missingReport,
}: {
  account: AccountValuation;
  timezone?: string;
  missingReport: boolean;
}) {
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
            freshness={missingReport ? undefined : account.freshness}
            completeness={account.completeness}
            errorClass={account.errorClass}
            errorDetail={account.errorDetail}
          />
          {missingReport ? (
            <span className={shared.muted}>
              No compatible successful account report; holdings and value are unknown.
            </span>
          ) : account.freshness === "stale" ? (
            <span className={shared.muted}>
              Earlier successful account report retained, observed{" "}
              {formatInstant(account.reportObservedAt, timezone)}
              {timezone ? ` · ${timezone}` : ""}.
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
