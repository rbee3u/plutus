import type { AccountValuation, Issue } from "@/api";
import { formatAmount, formatDecimal, formatRatio } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { shortId } from "@/shared/lib/id";
import { capitalize, issueLabel, NO_ACCOUNT_REPORT } from "@/shared/lib/text";
import { isAccountReportMissing } from "@/shared/lib/valuationRows";
import { AccountHealthBadges, Badge } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import { IssueList } from "@/shared/ui/IssueList";
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
  timezone,
  onManual,
}: {
  portfolioId: string;
  account: AccountValuation;
  cycleId: string;
  timezone?: string;
  onManual?: (target: ManualTarget) => void;
}) {
  const { report, isManual, isDual, isEarn } = useAccountReport(portfolioId, account, cycleId);
  const missingReport = isAccountReportMissing(account);
  const balanceIssues = report.data?.issues.filter(
    (issue) =>
      issue.omittedBalance ||
      issue.code === "missing" ||
      issue.code === "stale" ||
      issue.code === "outside_boundary",
  );
  const productIssues =
    isEarn || isDual
      ? report.data?.issues
      : report.data?.issues.filter((issue) => !issue.omittedBalance);
  const showEarn =
    isEarn &&
    (!report.isSuccess ||
      (report.data?.earnPositions.length ?? 0) > 0 ||
      (productIssues?.length ?? 0) > 0 ||
      report.isError);
  const showDual =
    isDual &&
    (!report.isSuccess ||
      (report.data?.dualPositions.length ?? 0) > 0 ||
      (productIssues?.length ?? 0) > 0 ||
      report.isError);
  const showFutures =
    !isDual &&
    !isEarn &&
    !isManual &&
    (!report.isSuccess ||
      (report.data?.positions.length ?? 0) > 0 ||
      (productIssues?.length ?? 0) > 0 ||
      report.isError);

  return (
    <div className={styles.accountDetail}>
      <QueryRecovery query={report} fallback="The account report could not be loaded." />
      <AccountMetrics account={account} />
      {!isManual || account.reportId ? (
        <AccountFeedback account={account} timezone={timezone} missingReport={missingReport} />
      ) : null}
      <section className={styles.accountDetail} aria-label="Balances">
        <BalancesTable
          account={account}
          data={report.data}
          isPending={report.isPending}
          isError={report.isError}
          timezone={timezone}
          onManual={onManual}
          missingReport={missingReport}
        />
        <HoldingIssues issues={balanceIssues} timezone={timezone} />
      </section>
      {showEarn ? (
        <section className={styles.accountDetail} aria-label="Earn">
          <h3 className={styles.accountSectionTitle}>Earn</h3>
          <EarnPositionsTable
            items={report.data?.earnPositions ?? []}
            timezone={report.data?.context.timezone}
            loading={report.isPending}
            empty={
              report.isError
                ? "Earn positions could not be loaded. Retry above."
                : missingReport
                  ? NO_ACCOUNT_REPORT
                  : undefined
            }
          />
          <HoldingIssues issues={productIssues} timezone={timezone} />
        </section>
      ) : null}
      {showDual ? (
        <section className={styles.accountDetail} aria-label="Dual investment">
          <h3 className={styles.accountSectionTitle}>Dual investment</h3>
          {report.isError && !report.data ? (
            <p className={shared.muted}>Dual investment could not be loaded. Retry above.</p>
          ) : (
            <DualPositionsTable
              items={report.data?.dualPositions ?? []}
              timezone={report.data?.context.timezone}
              loading={report.isPending}
              empty={missingReport ? NO_ACCOUNT_REPORT : undefined}
            />
          )}
          <HoldingIssues issues={productIssues} timezone={timezone} />
        </section>
      ) : null}
      {showFutures ? (
        <section className={styles.accountDetail} aria-label="Futures">
          <h3 className={styles.accountSectionTitle}>Futures</h3>
          <DataTable
            caption={`Positions of ${account.accountName}`}
            columns={POSITION_COLUMNS}
            rows={(report.data?.positions ?? []).map((item) =>
              positionRow(item, report.data?.context.timezone),
            )}
            empty={
              report.isPending
                ? "Loading positions."
                : report.isError && !report.data
                  ? "Futures could not be loaded. Retry above."
                  : missingReport
                    ? NO_ACCOUNT_REPORT
                    : "No open positions in this account report."
            }
          />
          <HoldingIssues issues={productIssues} timezone={timezone} />
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
    { label: "Equity", value: account.reportedEquityUsd, format: formatAmount },
    {
      label: "Adjusted equity",
      value: account.adjustedEquityUsd,
      format: formatAmount,
    },
    {
      label: "Available equity",
      value: account.availableEquityUsd,
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
  const showConnectionNotice =
    account.accountType !== "manual" &&
    account.connectionStatus &&
    account.connectionStatus !== "enabled";
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
      {!account.selected || account.identityStatus === "archived" || showConnectionNotice ? (
        <p className={shared.notice}>
          {!account.selected ? "Account is not selected. " : ""}
          {account.identityStatus === "archived" ? "Identity is archived. " : ""}
          {showConnectionNotice ? `Connection is ${account.connectionStatus}.` : ""}
        </p>
      ) : null}
      {account.errorClass ? (
        <p className={shared.alert} role="status">
          <Badge tone="bad">{issueLabel(account.errorClass)}</Badge>
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
              <Badge tone="warn">{issueLabel(item.reason)}</Badge>
            </span>
          ))}
        </p>
      ) : null}
    </>
  );
}
