import type { DualPositionRow } from "@/api";
import { platformLabel, usePlatforms } from "@/shared/lib/usePlatforms";
import { formatDecimal, formatPercent, formatPrice } from "@/shared/lib/amount";
import { capitalize } from "@/shared/lib/text";
import { formatInstant } from "@/shared/lib/datetime";
import { ObservedCell } from "./ObservedCell";
import { NumericText } from "@/shared/ui/NumericText";
import { DataTable } from "@/shared/ui/DataTable";
import { holdingsAccountColumns, HoldingsFilterHeader } from "./HoldingsFilterHeader";
import type { HoldingsFilters } from "../hooks/useHoldingsFilters";

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
        ...(showAccount ? holdingsAccountColumns(filters) : []),
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
        { key: "rate", header: "Rate", numeric: true, nowrap: true },
        { key: "principal", header: "Principal", numeric: true, nowrap: true },
        { key: "subscribed", header: "Subscribed", nowrap: true },
        { key: "status", header: "Status" },
        { key: "issues", header: "Issues", nowrap: true },
      ]}
      rows={items.map((item) => ({
        id: `${item.accountId}-${item.platformPositionRef}`,
        platform: platformLabel(platforms.data?.items, item.platform),
        identityName: item.identityName,
        pair: `${item.baseAssetSymbol ?? "—"} / ${item.quoteAssetSymbol ?? "—"}`,
        expiry: formatInstant(item.expiryAt, timezone),
        strike:
          item.targetPrice == null
            ? "—"
            : `${formatPrice(item.targetPrice)} ${item.quoteAssetSymbol ?? ""}`.trim(),
        direction:
          item.direction === "buy_low"
            ? "Buy Low"
            : item.direction === "sell_high"
              ? "Sell High"
              : "—",
        rate: (
          <NumericText>
            {item.annualizedRate == null
              ? "—"
              : `${formatPercent(item.annualizedRate)}${item.rateKind === "apr" ? " APR" : item.rateKind === "apy" ? " APY" : item.rateKind === "annualized_yield" ? " annualized" : ""}`}
          </NumericText>
        ),
        principal: `${formatDecimal(item.principal)} ${item.investAssetSymbol}`,
        subscribed: formatInstant(item.subscribedAt, timezone),
        status: capitalize(item.status),
        issues: (
          <ObservedCell
            observedAt={item.reportObservedAt}
            freshness={item.freshness}
            issues={item.issues}
            name={`Dual ${item.platformPositionRef}`}
            timezone={timezone}
          />
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
