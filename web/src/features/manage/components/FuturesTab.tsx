import { keys, listFuturesContracts } from "@/api";
import type { FuturesContractList } from "@/api";
import { formatContractSize } from "@/shared/lib/contract";
import { formatInstant } from "@/shared/lib/datetime";
import { capitalize } from "@/shared/lib/text";
import { platformLabel } from "@/shared/lib/usePlatforms";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { DataTable } from "@/shared/ui/DataTable";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { TableEmptyState } from "@/shared/ui/TableEmptyState";
import { TableFilterFooter } from "@/shared/ui/TableFilterFooter";
import { TableFilterHeader } from "@/shared/ui/TableFilterHeader";
import { PAGE_SIZE, type TabProps } from "../lib/catalog";
import shared from "@/shared/styles/page.module.css";
import manageStyles from "../styles.module.css";

function renderFuturesContractKind(kind: string) {
  const isPerp = kind.includes("perpetual");
  const isDelivery = kind.includes("delivery");
  const isInverse = kind.includes("inverse");
  const payoff = isInverse ? "Inverse" : "Linear";
  const term = isPerp
    ? "Perpetual"
    : isDelivery
      ? "Delivery"
      : capitalize(kind.replaceAll("_", " "));

  return (
    <span className={manageStyles.contractKindTags}>
      <span className={manageStyles.contractKindTag}>{payoff}</span>
      <span className={manageStyles.contractKindTag}>{term}</span>
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
                display={platform ? filters.platformName : ""}
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
                searchPlaceholder="Contains, e.g. BTC"
                searchHelp="Search supports partial matches and ignores case."
                onToggle={() => filters.toggle("q")}
                onClose={filters.close}
                onClear={() => filters.change({ q: "" })}
                onSearchChange={filters.editQuery}
                onCompositionStart={filters.beginComposition}
                onCompositionEnd={filters.endComposition}
              />
            ),
          },
          { key: "kind", header: "Kind", nowrap: true, width: "180px" },
          { key: "base", header: "Base", nowrap: true, width: "90px" },
          { key: "quote", header: "Quote", nowrap: true, width: "90px" },
          { key: "settle", header: "Settle", nowrap: true, width: "90px" },
          { key: "expiryAt", header: "Expiry", nowrap: true, width: "160px" },
          { key: "contractSize", header: "Contract size", numeric: true, width: "130px" },
          { key: "lastObservedAt", header: "Last observed", nowrap: true, width: "170px" },
        ]}
        rows={(query.data?.items ?? []).map((item) => {
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
            base: item.baseAssetSymbol ?? "—",
            quote: item.quoteAssetSymbol ?? "—",
            settle: item.settlementAssetSymbol ?? "—",
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
            <TableEmptyState title="Loading futures..." />
          ) : q || platform ? (
            <TableEmptyState
              title="No futures contracts match these filters"
              help="Try adjusting your search query or platform filter."
            />
          ) : (
            <TableEmptyState
              title="No futures contracts recorded yet"
              help="Futures contracts are discovered from reported positions."
            />
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
