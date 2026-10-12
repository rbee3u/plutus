import { keys, listFuturesContracts } from "@/api";
import type { FuturesContract, FuturesContractList } from "@/api";
import { formatDecimal } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { capitalize } from "@/shared/lib/text";
import { platformLabel } from "@/shared/lib/usePlatforms";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { ActionButton } from "@/shared/ui/ActionButton";
import { Badge } from "@/shared/ui/Badge";
import { DataTable } from "@/shared/ui/DataTable";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { TableFilterFooter } from "@/shared/ui/TableFilterFooter";
import { TableFilterHeader } from "@/shared/ui/TableFilterHeader";
import { PAGE_SIZE, type TabProps } from "../lib/catalog";
import shared from "@/shared/styles/page.module.css";

function formatContractSize(item: FuturesContract): string {
  const isInverse = item.kind.includes("inverse");
  const unit = isInverse ? item.quoteAssetSymbol : item.baseAssetSymbol;
  const size = formatDecimal(item.contractSize);
  return size !== "—" && unit ? `${size} ${unit}` : size;
}

function renderFuturesContractKind(kind: string) {
  const isPerp = kind.includes("perpetual");
  const isDelivery = kind.includes("delivery");
  const isInverse = kind.includes("inverse");
  const margin = isInverse ? "Inverse" : "Linear";
  const typeLabel = isPerp
    ? "PERP"
    : isDelivery
      ? "DELIVERY"
      : capitalize(kind.replaceAll("_", " "));

  return (
    <span className={shared.cellStack}>
      <Badge tone={isPerp ? "info" : isDelivery ? "warn" : "neutral"}>{typeLabel}</Badge>
      <span className={shared.cellMeta}>{margin}</span>
    </span>
  );
}

export function FuturesTab({ filters, timezone }: TabProps) {
  const { platform, q } = filters.values;
  const offset = filters.offset;
  const platforms = filters.platforms;
  const query = usePortfolioQuery<FuturesContractList>({
    key: (userId, portfolioId) =>
      keys.catalogFuturesContracts(userId, portfolioId, platform, q, offset),
    load: (portfolioId) =>
      listFuturesContracts({
        path: { portfolioId },
        query: { platform: platform || undefined, q: q || undefined, offset, limit: PAGE_SIZE },
      }),
  });

  return (
    <>
      <QueryRecovery query={query} fallback="Futures could not be loaded." />
      <DataTable
        caption="Futures"
        columns={[
          {
            key: "platform",
            header: "Platform",
            nowrap: true,
            width: "140px",
            headerContent: (
              <TableFilterHeader
                label="Platform"
                display={platform ? filters.platformName : "All"}
                active={Boolean(platform)}
                open={filters.open === "platform"}
                type="select"
                options={filters.platformOptions}
                selectedValue={platform}
                onToggle={() => filters.toggle("platform")}
                onClose={filters.close}
                onClear={() => filters.change({ platform: "" })}
                onSelect={(val) => filters.change({ platform: val })}
              />
            ),
          },
          {
            key: "symbol",
            header: "Symbol",
            nowrap: true,
            headerContent: (
              <TableFilterHeader
                label="Symbol"
                display={filters.qDraft}
                active={Boolean(q || filters.qDraft)}
                open={filters.open === "q"}
                type="search"
                searchDraft={filters.qDraft}
                searchPlaceholder="Filter symbol or pair"
                onToggle={() => filters.toggle("q")}
                onClose={filters.close}
                onClear={() => filters.change({ q: "" })}
                onSearchChange={filters.editQuery}
                onCompositionStart={filters.beginComposition}
                onCompositionEnd={filters.endComposition}
              />
            ),
          },
          { key: "kind", header: "Kind", nowrap: true, width: "100px" },
          { key: "underlying", header: "Underlying", nowrap: true, width: "120px" },
          { key: "expiryAt", header: "Expiry", nowrap: true, width: "160px" },
          { key: "contractSize", header: "Contract size", numeric: true, width: "130px" },
          { key: "lastObservedAt", header: "Last observed", nowrap: true, width: "170px" },
        ]}
        rows={(query.data?.items ?? []).map((item) => {
          const pair =
            item.baseAssetSymbol || item.quoteAssetSymbol
              ? `${item.baseAssetSymbol ?? "—"} / ${item.quoteAssetSymbol ?? "—"}`
              : "—";

          return {
            id: item.id,
            platform: platformLabel(platforms.data?.items, item.platform),
            symbol: (
              <strong
                style={{
                  fontFamily: "var(--font-mono)",
                  letterSpacing: "0.02em",
                }}
              >
                {item.symbol}
              </strong>
            ),
            kind: renderFuturesContractKind(item.kind),
            underlying: (
              <span className={shared.cellStack}>
                <span>{pair}</span>
                {item.settlementAssetSymbol ? (
                  <span className={shared.cellMeta}>Settle: {item.settlementAssetSymbol}</span>
                ) : null}
              </span>
            ),
            expiryAt: item.expiryAt ? (
              <span className={shared.tabularDate}>{formatInstant(item.expiryAt, timezone)}</span>
            ) : (
              "—"
            ),
            contractSize: formatContractSize(item),
            lastObservedAt: (
              <span className={shared.tabularDate}>
                {formatInstant(item.lastObservedAt, timezone)}
              </span>
            ),
          };
        })}
        empty={
          query.isPending ? (
            <div className={shared.emptyState}>
              <p className={shared.emptyStateTitle}>Loading futures...</p>
            </div>
          ) : q || platform ? (
            <div className={shared.emptyState}>
              <p className={shared.emptyStateTitle}>No futures contracts match these filters</p>
              <p className={shared.emptyStateHelp}>
                Try adjusting your search query or platform filter.
              </p>
              <ActionButton type="button" tone="secondary" onClick={filters.reset}>
                Clear filters
              </ActionButton>
            </div>
          ) : (
            <div className={shared.emptyState}>
              <p className={shared.emptyStateTitle}>No futures contracts recorded yet</p>
              <p className={shared.emptyStateHelp}>
                Market contracts will appear automatically once platform identities are connected
                and sync.
              </p>
            </div>
          )
        }
      />
      <TableFilterFooter
        page={query.data?.page}
        onPageChange={filters.page}
        hasFilters={filters.hasFilters}
        onReset={filters.reset}
      />
    </>
  );
}
