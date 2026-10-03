import type { DualPositionRow } from "@/api/generated/types.gen";
import { platformLabel, usePlatforms } from "@/features/data/usePlatforms";
import { formatDecimal, formatPercent, formatPrice } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { Badge } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import styles from "@/features/pages.module.css";
import { HoldingsFilterHeader } from "./HoldingsFilterHeader";
import type { HoldingsFilters } from "./useHoldingsFilters";

export function DualPositionsTable({
  items,
  timezone,
  showAccount = false,
  loading = false,
  filters,
}: {
  items: DualPositionRow[];
  timezone?: string;
  showAccount?: boolean;
  loading?: boolean;
  filters?: HoldingsFilters;
}) {
  const platforms = usePlatforms();
  return (
    <DataTable
      caption="Dual Investment positions"
      columns={[
        ...(showAccount
          ? [
              {
                key: "platform",
                header: "Platform",
                nowrap: true,
                headerContent: filters ? (
                  <HoldingsFilterHeader label="Platform" filterKey="platform" filters={filters} />
                ) : undefined,
              },
              {
                key: "identity",
                header: "Identity",
                nowrap: true,
                headerContent: filters ? (
                  <HoldingsFilterHeader label="Identity" filterKey="identityId" filters={filters} />
                ) : undefined,
              },
            ]
          : []),
        {
          key: "pair",
          header: "Pair",
          nowrap: true,
          headerContent: filters ? (
            <HoldingsFilterHeader label="Pair" filterKey="symbol" filters={filters} />
          ) : undefined,
        },
        { key: "expiry", header: "Expiry", nowrap: true },
        { key: "strike", header: "Strike", numeric: true, nowrap: true },
        { key: "direction", header: "Direction", nowrap: true },
        { key: "apy", header: "APY", numeric: true, nowrap: true },
        { key: "principal", header: "Principal", numeric: true, nowrap: true },
        { key: "subscribed", header: "Subscribed", nowrap: true },
        { key: "status", header: "Status" },
      ]}
      rows={items.map((item) => ({
        id: `${item.accountId}-${item.platformPositionRef}`,
        platform: platformLabel(platforms.data?.items, item.platform),
        identity: item.identityName,
        pair: `${item.baseSymbol ?? "—"} / ${item.quoteSymbol ?? "—"}`,
        expiry: formatInstant(item.expiryAt, timezone),
        strike:
          item.targetPrice == null
            ? "—"
            : `${formatPrice(item.targetPrice)} ${item.quoteSymbol ?? ""}`.trim(),
        direction:
          item.direction === "buy_low"
            ? "Buy Low"
            : item.direction === "sell_high"
              ? "Sell High"
              : "—",
        apy: formatPercent(item.annualizedRate),
        principal: `${formatDecimal(item.principal)} ${item.investSymbol}`,
        subscribed: formatInstant(item.subscribedAt, timezone),
        status: (
          <span className={styles.statusGroup}>
            <span>{item.status.charAt(0).toUpperCase() + item.status.slice(1)}</span>
            {item.freshness === "stale" ? (
              <Badge tone="warn" title="Report carried forward (stale)">
                stale
              </Badge>
            ) : null}
          </span>
        ),
      }))}
      empty={
        loading
          ? "Loading dual positions."
          : "No outstanding Dual Investment positions match this selection."
      }
    />
  );
}
