import { useQuery } from "@tanstack/react-query";
import {
  listAccountBalances,
  listAccountPositions,
  listAccountDualPositions,
} from "@/api/generated/sdk.gen";
import type { AccountValuation, BalanceRow, PositionRow } from "@/api/generated/types.gen";
import { formatAmount, formatDecimal } from "@/shared/lib/amount";
import { shortId } from "@/shared/lib/id";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { FreshnessBadge } from "@/shared/ui/Badge";
import { DataTable, type DataRow } from "@/shared/ui/DataTable";
import { Codes, IssueList } from "@/shared/ui/IssueList";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { DualPositionsTable } from "@/features/holdings/DualPositionsTable";
import styles from "@/features/pages.module.css";

/** One Account's current Account Report: valuation metrics, balances, and positions. */
export function AccountDetail({
  portfolioId,
  account,
}: {
  portfolioId: string;
  account: AccountValuation;
}) {
  const { userId, redirect } = useSessionScope();
  const accountId = account.accountId;
  const balances = useQuery({
    queryKey: ["account-balances", userId, portfolioId, accountId],
    enabled: Boolean(userId && portfolioId && accountId),
    queryFn: async () =>
      unwrapOrRedirect(await listAccountBalances({ path: { portfolioId, accountId } }), redirect),
  });
  const isDual = account.accountType === "dual_investment";
  const positions = useQuery({
    queryKey: ["account-positions", userId, portfolioId, accountId],
    enabled: Boolean(userId && portfolioId && accountId && !isDual),
    queryFn: async () =>
      unwrapOrRedirect(await listAccountPositions({ path: { portfolioId, accountId } }), redirect),
  });
  const dualPositions = useQuery({
    queryKey: ["account-dual-positions", userId, portfolioId, accountId],
    enabled: Boolean(userId && portfolioId && isDual),
    queryFn: async () =>
      unwrapOrRedirect(
        await listAccountDualPositions({ path: { portfolioId, accountId } }),
        redirect,
      ),
  });
  const issues = [
    ...(balances.data?.issues ?? []),
    ...(isDual ? (dualPositions.data?.issues ?? []) : (positions.data?.issues ?? [])),
  ];

  return (
    <div className={styles.detail}>
      <AccountMetrics account={account} />
      <QueryRecovery
        error={balances.error}
        hasData={Boolean(balances.data)}
        busy={balances.isFetching}
        fallback="Account balances could not be loaded."
        onRetry={() => {
          void balances.refetch();
        }}
      />
      <h3>Balances</h3>
      <DataTable
        caption={`Balances of ${account.accountName}`}
        columns={BALANCE_COLUMNS}
        rows={(balances.data?.items ?? []).map(balanceRow)}
        empty={
          balances.isPending
            ? "Loading balances."
            : "No balances in the current Account Report for this account."
        }
      />
      {isDual ? (
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
          <h3>Dual Investment positions</h3>
          <DualPositionsTable
            items={dualPositions.data?.items ?? []}
            timezone={dualPositions.data?.context.timezone}
            loading={dualPositions.isPending}
          />
        </>
      ) : (
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
          <h3>Positions</h3>
          <DataTable
            caption={`Positions of ${account.accountName}`}
            columns={POSITION_COLUMNS}
            rows={(positions.data?.items ?? []).map(positionRow)}
            empty={
              positions.isPending
                ? "Loading positions."
                : "No open positions in the current Account Report for this account."
            }
          />
        </>
      )}
      {issues.length > 0 ? (
        <>
          <h3>Issues</h3>
          <IssueList issues={issues} />
        </>
      ) : null}
    </div>
  );
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
      format: formatDecimal,
    },
    {
      label: "MMR",
      title: "Maintenance margin ratio",
      value: account.maintenanceMarginRatio,
      format: formatDecimal,
    },
    {
      label: "UniMMR",
      title: "Unified margin ratio",
      value: account.unifiedMarginRatio,
      format: formatDecimal,
    },
  ];
  // A calculated account carries none of these; six dashes would say less than one sentence.
  const reported = figures.some((figure) => figure.value != null);
  return (
    <>
      {reported ? (
        <dl className={`${styles.metaList} ${styles.metaListCased}`}>
          {figures.map((figure) => (
            <div key={figure.label}>
              <dt>
                {figure.label === figure.title ? (
                  figure.label
                ) : (
                  <abbr title={figure.title}>{figure.label}</abbr>
                )}
              </dt>
              <dd>{figure.format(figure.value)}</dd>
            </div>
          ))}
        </dl>
      ) : null}
      <p className={styles.muted}>
        Account Report{" "}
        <span title={account.reportId ?? undefined}>{shortId(account.reportId)}</span>
        {reported
          ? ", figures as reported by the Platform."
          : " carries no equity or margin figures; the value is calculated from the balances below."}
      </p>
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
              {formatAmount(item.equity)} <Codes values={[item.reason]} />
            </span>
          ))}
        </p>
      ) : null}
    </>
  );
}

export const BALANCE_COLUMNS = [
  { key: "symbol", header: "Symbol" },
  { key: "equity", header: "Equity", numeric: true },
  { key: "availableToWithdraw", header: "Withdrawable", numeric: true },
  { key: "availableToTrade", header: "Tradable", numeric: true },
  { key: "liability", header: "Liability", numeric: true },
  { key: "unrealizedPnl", header: "Unrealized PnL", numeric: true },
  { key: "priceUsd", header: "Price (USD)", numeric: true },
  { key: "equityUsd", header: "Equity (USD)", numeric: true },
  { key: "freshness", header: "Freshness" },
];

export function balanceRow(item: BalanceRow): DataRow {
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
    availableToWithdraw: formatDecimal(item.availableToWithdraw),
    availableToTrade: formatDecimal(item.availableToTrade),
    liability: formatDecimal(item.liability),
    unrealizedPnl: formatDecimal(item.unrealizedPnl),
    priceUsd: item.noPrice ? (
      <span className={styles.cellMeta}>no price</span>
    ) : (
      formatDecimal(item.priceUsd)
    ),
    equityUsd: formatAmount(item.equityUsd),
    freshness: <FreshnessBadge value={item.freshness} />,
  };
}

export const POSITION_COLUMNS = [
  { key: "symbol", header: "Symbol", nowrap: true },
  { key: "quantity", header: "Quantity", numeric: true },
  { key: "entryPrice", header: "Entry", numeric: true },
  { key: "markPrice", header: "Mark", numeric: true },
  { key: "liquidationPrice", header: "Liquidation", numeric: true },
  { key: "unrealizedPnl", header: "Unrealized PnL", numeric: true },
  { key: "leverage", header: "Leverage", numeric: true },
  { key: "margin", header: "Margin", numeric: true },
  { key: "notionalUsd", header: "Notional (USD)", numeric: true },
  { key: "freshness", header: "Freshness" },
];

/** The unit a position's quantity counts: the base or quote Asset, or contracts. */
function quantityUnit(item: PositionRow): string {
  if (item.quantityUnit === "base") return item.baseSymbol ?? "base";
  if (item.quantityUnit === "quote") return item.quoteSymbol ?? "quote";
  return "contracts";
}

export function positionRow(item: PositionRow): DataRow {
  return {
    id: `${item.accountId}-${item.instrumentId}-${item.platformPositionRef}`,
    symbol: (
      <span className={styles.cellStack}>
        <span>{item.symbol}</span>
        <span className={styles.cellMeta}>
          {item.kind} · {item.side}
        </span>
      </span>
    ),
    quantity: `${formatDecimal(item.quantity)} ${quantityUnit(item)}`,
    entryPrice: formatDecimal(item.entryPrice),
    markPrice: formatDecimal(item.markPrice),
    liquidationPrice: formatDecimal(item.liquidationPrice),
    unrealizedPnl: formatDecimal(item.unrealizedPnl),
    leverage: formatDecimal(item.leverage),
    margin: formatDecimal(item.margin),
    notionalUsd: formatAmount(item.notionalUsd),
    freshness: <FreshnessBadge value={item.freshness} />,
  };
}
