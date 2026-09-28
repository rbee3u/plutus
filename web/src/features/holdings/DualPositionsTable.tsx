import type { DualPositionRow } from "@/api/generated/types.gen";
import { formatDecimal, parseDecimal } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { FreshnessBadge } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import styles from "@/features/pages.module.css";

export function DualPositionsTable({
  items,
  timezone,
  showAccount = false,
  loading = false,
}: {
  items: DualPositionRow[];
  timezone?: string;
  showAccount?: boolean;
  loading?: boolean;
}) {
  return (
    <DataTable
      caption="Dual Investment positions"
      columns={[
        ...(showAccount ? [{ key: "account", header: "Account" }] : []),
        { key: "pair", header: "Pair" },
        { key: "direction", header: "Direction", nowrap: true },
        { key: "principal", header: "Principal", numeric: true, nowrap: true },
        { key: "target", header: "Target price", numeric: true, nowrap: true },
        { key: "expiry", header: "Expiry", nowrap: true },
        { key: "rate", header: "Annualized rate", numeric: true, nowrap: true },
        { key: "status", header: "Status" },
        { key: "freshness", header: "Freshness" },
      ]}
      rows={items.map((item) => ({
        id: `${item.accountId}-${item.platformPositionRef}`,
        account: (
          <span className={styles.cellStack}>
            <span>{item.accountName}</span>
            <span className={styles.cellMeta}>
              {item.platform} · {item.identityName}
            </span>
          </span>
        ),
        pair: (
          <details>
            <summary>
              {item.baseSymbol ?? "—"} / {item.quoteSymbol ?? "—"}
            </summary>
            <dl>
              <dt>Position</dt>
              <dd>{item.platformPositionRef}</dd>
              <dt>Product</dt>
              <dd>{item.productRef ?? "—"}</dd>
              <dt>Order</dt>
              <dd>{item.orderRef ?? "—"}</dd>
            </dl>
          </details>
        ),
        direction:
          item.direction === "buy_low"
            ? "Buy Low"
            : item.direction === "sell_high"
              ? "Sell High"
              : "—",
        principal: `${formatDecimal(item.principal)} ${item.investSymbol}`,
        target:
          item.targetPrice == null
            ? "—"
            : `${formatDecimal(item.targetPrice)} ${item.quoteSymbol ?? ""}`,
        expiry: formatInstant(item.expiryAt, timezone),
        rate:
          item.annualizedRate == null
            ? "—"
            : `${formatDecimal(parseDecimal(item.annualizedRate)?.times(100).toString())}% ${item.rateKind === "annualized_yield" ? "Annualized Yield" : (item.rateKind?.toUpperCase() ?? "")}`,
        status: item.status.charAt(0).toUpperCase() + item.status.slice(1),
        freshness: <FreshnessBadge value={item.freshness} />,
      }))}
      empty={
        loading
          ? "Loading dual positions."
          : "No outstanding Dual Investment positions match this selection."
      }
    />
  );
}
