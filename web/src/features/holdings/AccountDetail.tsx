import { NumericText } from "@/shared/ui/NumericText";
import { useQuery } from "@tanstack/react-query";
import {
  listAccountBalances,
  listAccountPositions,
  listAccountDualPositions,
  listAccountEarnPositions,
} from "@/api/generated/sdk.gen";
import type { AccountValuation, BalanceRow, Issue, ReadContext } from "@/api/generated/types.gen";
import {
  formatAmount,
  formatDecimal,
  formatRatio,
  formatPrice,
  parseDecimal,
} from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { shortId } from "@/shared/lib/id";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { AccountHealthBadges, Badge } from "@/shared/ui/Badge";
import { DataTable, type DataRow } from "@/shared/ui/DataTable";
import { HoverDetails } from "@/shared/ui/HoverDetails";
import { Codes, IssueList } from "@/shared/ui/IssueList";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { POSITION_COLUMNS, positionRow } from "./PositionsTable";
import { EarnPositionsTable, EarnBalanceSummaries } from "./EarnPositionsTable";
import { DualPositionsTable } from "@/features/holdings/DualPositionsTable";
import styles from "@/features/pages.module.css";

/** One Account's current Account Report: valuation metrics, balances, and positions. */
export function AccountDetail({
  portfolioId,
  account,
  pointId,
  timezone,
}: {
  portfolioId: string;
  account: AccountValuation;
  pointId: string;
  timezone?: string;
}) {
  const { userId, redirect } = useSessionScope();
  const accountId = account.accountId;
  const balances = useQuery({
    queryKey: ["account-balances", userId, portfolioId, accountId, pointId, account.reportId],
    enabled: Boolean(userId && portfolioId && accountId),
    queryFn: async () =>
      checkAccountReport(
        unwrapOrRedirect(await listAccountBalances({ path: { portfolioId, accountId } }), redirect),
        account,
        pointId,
      ),
  });
  const isDual = account.accountType === "dual_investment";
  const isEarn = account.accountType === "earn";
  const positions = useQuery({
    queryKey: ["account-positions", userId, portfolioId, accountId, pointId, account.reportId],
    enabled: Boolean(userId && portfolioId && accountId && !isDual && !isEarn),
    queryFn: async () =>
      checkAccountReport(
        unwrapOrRedirect(
          await listAccountPositions({ path: { portfolioId, accountId }, query: { pointId } }),
          redirect,
        ),
        account,
        pointId,
      ),
  });
  const dualPositions = useQuery({
    queryKey: ["account-dual-positions", userId, portfolioId, accountId, pointId, account.reportId],
    enabled: Boolean(userId && portfolioId && isDual),
    queryFn: async () =>
      checkAccountReport(
        unwrapOrRedirect(
          await listAccountDualPositions({ path: { portfolioId, accountId } }),
          redirect,
        ),
        account,
        pointId,
      ),
  });
  const earnPositions = useQuery({
    queryKey: ["account-earn-positions", userId, portfolioId, accountId, pointId, account.reportId],
    enabled: Boolean(userId && portfolioId && isEarn),
    queryFn: async () =>
      checkAccountReport(
        unwrapOrRedirect(
          await listAccountEarnPositions({ path: { portfolioId, accountId } }),
          redirect,
        ),
        account,
        pointId,
      ),
  });
  const issues = [
    ...(balances.data?.issues ?? []),
    ...(isDual
      ? (dualPositions.data?.issues ?? [])
      : isEarn
        ? (earnPositions.data?.issues ?? [])
        : (positions.data?.issues ?? [])),
  ];

  return (
    <div className={styles.accountDetail}>
      <AccountMetrics account={account} />
      <AccountFeedback account={account} timezone={timezone} />
      <QueryRecovery
        error={balances.error}
        hasData={Boolean(balances.data)}
        busy={balances.isFetching}
        fallback="Account balances could not be loaded."
        onRetry={() => {
          void balances.refetch();
        }}
      />
      <h3 className={styles.accountSectionTitle}>Balances</h3>
      <DataTable
        caption={`Balances of ${account.accountName}`}
        columns={BALANCE_COLUMNS}
        rows={(balances.data?.items ?? []).map((item) =>
          balanceRow(item, balances.data?.context.timezone, balances.data?.issues),
        )}
        empty={
          balances.isPending
            ? "Loading balances."
            : balances.isError && !balances.data
              ? "Balances could not be loaded. Retry above."
              : "No balances in the current Account Report for this account."
        }
      />
      {isEarn &&
      (!earnPositions.isSuccess ||
        earnPositions.data.items.length > 0 ||
        earnPositions.data.balanceSummaries.length > 0 ||
        !earnPositions.data.detailsCollected ||
        earnPositions.isError) ? (
        <>
          <QueryRecovery
            error={earnPositions.error}
            hasData={Boolean(earnPositions.data)}
            busy={earnPositions.isFetching}
            fallback="Earn positions could not be loaded."
            onRetry={() => {
              void earnPositions.refetch();
            }}
          />
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
      {isDual &&
      (!dualPositions.isSuccess || dualPositions.data.items.length > 0 || dualPositions.isError) ? (
        <>
          <QueryRecovery
            error={dualPositions.error}
            hasData={Boolean(dualPositions.data)}
            busy={dualPositions.isFetching}
            fallback="Dual positions could not be loaded."
            onRetry={() => {
              void dualPositions.refetch();
            }}
          />
          <h3 className={styles.accountSectionTitle}>Dual Investment</h3>
          {dualPositions.isError && !dualPositions.data ? (
            <p className={styles.muted}>Dual Investment could not be loaded. Retry above.</p>
          ) : (
            <DualPositionsTable
              items={dualPositions.data?.items ?? []}
              timezone={dualPositions.data?.context.timezone}
              loading={dualPositions.isPending}
            />
          )}
        </>
      ) : !isDual &&
        !isEarn &&
        (!positions.isSuccess || positions.data.items.length > 0 || positions.isError) ? (
        <>
          <QueryRecovery
            error={positions.error}
            hasData={Boolean(positions.data)}
            busy={positions.isFetching}
            fallback="Account positions could not be loaded."
            onRetry={() => {
              void positions.refetch();
            }}
          />
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
      <details className={styles.reportDetails}>
        <summary>Report details</summary>
        <dl className={styles.metaList}>
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
        <p className={styles.muted}>
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

/** A newer latest-only response must not describe an older visible Account row. */
function checkAccountReport<T extends { context: ReadContext; account: AccountValuation }>(
  data: T,
  account: AccountValuation,
  pointId: string,
): T {
  if (
    data.context.pointId !== pointId ||
    (data.account.reportId ?? null) !== (account.reportId ?? null)
  ) {
    throw new Error(
      "This Account changed since the displayed Valuation Point. Reload the page to read its latest Account Report.",
    );
  }
  return data;
}

function AccountMetrics({ account }: { account: AccountValuation }) {
  // Short labels keep the six figures on one line; <abbr> carries the full name for hover and
  // assistive technology.
  const figures = [
    { label: "Equity", title: "Equity", value: account.equity, format: formatAmount },
    {
      label: "AdjEq",
      title: "Adjusted equity",
      value: account.adjustedEquity,
      format: formatAmount,
    },
    {
      label: "AvailEq",
      title: "Available equity",
      value: account.availableEquity,
      format: formatAmount,
    },
    {
      label: "IMR",
      title: "Initial margin ratio",
      value: account.initialMarginRatio,
      format: formatRatio,
    },
    {
      label: "MMR",
      title: "Maintenance margin ratio",
      value: account.maintenanceMarginRatio,
      format: formatRatio,
    },
    {
      label: "UniMMR",
      title: "Unified margin ratio",
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
          <span className={styles.accountMetricLabel}>
            {figure.label === figure.title ? (
              figure.label
            ) : (
              <abbr title={figure.title}>{figure.label}</abbr>
            )}
          </span>
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
        <div className={styles.statusGroup}>
          <AccountHealthBadges
            status={account.status}
            freshness={account.freshness}
            completeness={account.completeness}
            errorClass={account.errorClass}
            errorDetail={account.errorDetail}
          />
          {account.freshness === "stale" ? (
            <span className={styles.muted}>
              Earlier successful Account Report retained, observed{" "}
              {formatInstant(account.reportObservedAt, timezone)}
              {timezone ? ` · ${timezone}` : ""}.
            </span>
          ) : account.method === "missing" ? (
            <span className={styles.muted}>
              No compatible successful Account Report; value is unknown.
            </span>
          ) : null}
        </div>
      ) : null}
      {!account.selected ||
      account.identityStatus === "archived" ||
      (account.connectionStatus && account.connectionStatus !== "enabled") ? (
        <p className={styles.notice}>
          {!account.selected ? "Account is not selected. " : ""}
          {account.identityStatus === "archived" ? "Identity is archived. " : ""}
          {account.connectionStatus && account.connectionStatus !== "enabled"
            ? `Connection is ${account.connectionStatus}.`
            : ""}
        </p>
      ) : null}
      {account.errorClass ? (
        <p className={styles.alert} role="status">
          <Codes values={[account.errorClass]} />
          {account.errorDetail ? ` ${account.errorDetail}` : ""}
        </p>
      ) : null}
      {account.omitted.length > 0 ? (
        <p className={styles.notice}>
          Omitted from the total:{" "}
          {account.omitted.map((item, index) => (
            <span key={item.symbol}>
              {index > 0 ? "; " : ""}
              {item.symbol}
              {item.assetSymbol ? ` (mapped to ${item.assetSymbol})` : ""}{" "}
              <NumericText>{formatDecimal(item.equity)}</NumericText>{" "}
              <Codes values={[item.reason]} />
            </span>
          ))}
        </p>
      ) : null}
    </>
  );
}

export const BALANCE_COLUMNS = [
  { key: "symbol", header: "Asset" },
  { key: "equity", header: "Equity", numeric: true },
  { key: "priceUsd", header: "Price (USD)", numeric: true },
  { key: "equityUsd", header: "Equity (USD)", numeric: true },
  { key: "liability", header: "Liability", numeric: true },
  { key: "unrealizedPnl", header: "Unrealized PnL", numeric: true },
  { key: "observed", header: "Observed", nowrap: true },
];

function balancePnl(value: string | null | undefined) {
  if (value == null) return "—";
  const parsed = parseDecimal(value);
  if (!parsed || parsed.isZero()) return formatDecimal(value);
  const sign = parsed.gt(0) ? "+" : "";
  const className = parsed.gt(0) ? styles.pnlPositive : styles.pnlNegative;
  return (
    <span className={className}>
      <NumericText>{`${sign}${formatDecimal(value)}`}</NumericText>
    </span>
  );
}

export function balanceRow(item: BalanceRow, timezone?: string, issues: Issue[] = []): DataRow {
  const stale = issues.find(
    (issue) => issue.accountId === item.accountId && issue.code === "stale",
  );
  return {
    id: `${item.accountId}-${item.listingId}`,
    // The Asset is the Symbol unless a listing remaps it; only the exception earns a second line.
    symbol:
      item.assetSymbol && item.assetSymbol !== item.symbol ? (
        <span className={styles.cellStack}>
          <span>{item.symbol}</span>
          <span className={styles.cellMeta}>as {item.assetSymbol}</span>
        </span>
      ) : (
        item.symbol
      ),
    equity: formatDecimal(item.equity),
    liability: formatDecimal(item.liability),
    unrealizedPnl: balancePnl(item.unrealizedPnl),
    priceUsd: item.noPrice ? (
      <span className={styles.cellMeta}>no price</span>
    ) : (
      formatPrice(item.priceUsd)
    ),
    equityUsd: formatAmount(item.equityUsd),
    observed: (
      <span className={styles.statusGroup}>
        <span
          className={styles.muted}
          title={`Account Report observed at ${item.reportObservedAt ?? "an unknown time"}; times in ${timezone ?? "the Portfolio timezone"}.`}
        >
          {formatInstant(item.reportObservedAt, timezone)}
        </span>
        {item.freshness === "stale" ? (
          <HoverDetails
            label={<Badge tone="warn">Stale</Badge>}
            name={`Stale report for ${item.symbol}`}
          >
            <p>{stale?.detail ?? "An earlier successful Account Report is carried forward."}</p>
          </HoverDetails>
        ) : null}
      </span>
    ),
  };
}
