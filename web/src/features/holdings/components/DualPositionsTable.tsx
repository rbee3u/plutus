import type { ReactNode } from "react";
import type { DualPositionRow } from "@/api";
import { platformLabel, usePlatforms } from "@/shared/lib/usePlatforms";
import { formatDecimal, formatPercent, formatPrice } from "@/shared/lib/amount";
import { capitalize } from "@/shared/lib/text";
import { formatInstant } from "@/shared/lib/datetime";
import { Badge, StatusBadge } from "@/shared/ui/Badge";
import { ObservedCell } from "./ObservedCell";
import { NumericText } from "@/shared/ui/NumericText";
import { DataTable } from "@/shared/ui/DataTable";
import { TableEmptyState } from "@/shared/ui/TableEmptyState";
import { holdingsAccountColumns, HoldingsFilterHeader } from "./HoldingsFilterHeader";
import type { HoldingsFilters } from "../hooks/useHoldingsFilters";

export function DualPositionsTable({
  items,
  timezone,
  showAccount = false,
  loading = false,
  empty,
  filters,
}: {
  items: DualPositionRow[];
  timezone?: string;
  showAccount?: boolean;
  loading?: boolean;
  empty?: ReactNode;
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
          width: "140px",
          headerContent: filters ? (
            <HoldingsFilterHeader label="Pair" filterKey="symbol" filters={filters} />
          ) : undefined,
        },
        { key: "expiry", header: "Expiry", nowrap: true, width: "160px" },
        { key: "strike", header: "Strike", numeric: true, nowrap: true, width: "120px" },
        { key: "direction", header: "Direction", nowrap: true, width: "110px" },
        { key: "rate", header: "Rate", numeric: true, nowrap: true, width: "110px" },
        { key: "principal", header: "Principal", numeric: true, nowrap: true, width: "130px" },
        { key: "subscribed", header: "Subscribed", nowrap: true, width: "160px" },
        { key: "status", header: "Status", nowrap: true, width: "100px" },
        { key: "issues", header: "Issues", nowrap: true, width: "100px" },
      ]}
      rows={items.map((item) => ({
        id: `${item.accountId}-${item.positionKey}`,
        platform: platformLabel(platforms.data?.items, item.platform),
        identityName: item.identityName,
        pair: `${item.baseAssetSymbol ?? "—"} / ${item.quoteAssetSymbol ?? "—"}`,
        expiry: formatInstant(item.expiryAt, timezone),
        strike:
          item.targetPrice == null
            ? "—"
            : `${formatPrice(item.targetPrice)} ${item.quoteAssetSymbol ?? ""}`.trim(),
        direction:
          item.direction === "buy_low" ? (
            <Badge tone="good">Buy Low</Badge>
          ) : item.direction === "sell_high" ? (
            <Badge tone="info">Sell High</Badge>
          ) : item.direction ? (
            <Badge tone="neutral">{capitalize(item.direction)}</Badge>
          ) : (
            "—"
          ),
        rate: (
          <NumericText>
            {item.annualizedRate == null
              ? "—"
              : `${formatPercent(item.annualizedRate)}${item.rateKind === "apr" ? " APR" : item.rateKind === "apy" ? " APY" : item.rateKind === "annualized_yield" ? " annualized" : ""}`}
          </NumericText>
        ),
        principal: `${formatDecimal(item.principal)} ${item.principalAssetSymbol}`,
        subscribed: formatInstant(item.subscribedAt, timezone),
        status: item.status ? <StatusBadge value={item.status} /> : "—",
        issues: (
          <ObservedCell
            observedAt={item.reportObservedAt}
            freshness={item.freshness}
            issues={item.issues}
            name={`Dual ${item.positionKey}`}
            timezone={timezone}
          />
        ),
      }))}
      empty={
        loading ? (
          <TableEmptyState title="Loading Dual Investment subscriptions..." />
        ) : (
          (empty ?? (
            <TableEmptyState
              title="No Dual Investment subscriptions recorded yet"
              help="Dual Investment subscriptions will appear automatically once platform identities are connected and sync."
            />
          ))
        )
      }
    />
  );
}
