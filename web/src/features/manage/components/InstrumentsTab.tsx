import { keys, listInstruments } from "@/api";
import type { InstrumentList } from "@/api";
import { formatDecimal } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { capitalize } from "@/shared/lib/text";
import { platformLabel } from "@/shared/lib/usePlatforms";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { DataTable } from "@/shared/ui/DataTable";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { TableFilterFooter } from "@/shared/ui/TableFilterFooter";
import { TableFilterHeader } from "@/shared/ui/TableFilterHeader";
import { PAGE_SIZE, type TabProps } from "../lib/catalog";

export function InstrumentsTab({ filters, timezone }: TabProps) {
  const { platform, q } = filters.values;
  const offset = filters.offset;
  const platforms = filters.platforms;
  const query = usePortfolioQuery<InstrumentList>({
    key: (userId, portfolioId) => keys.catalogInstruments(userId, portfolioId, platform, q, offset),
    load: (portfolioId) =>
      listInstruments({
        path: { portfolioId },
        query: { platform: platform || undefined, q: q || undefined, offset, limit: PAGE_SIZE },
      }),
  });

  return (
    <>
      <QueryRecovery query={query} fallback="Instruments could not be loaded." />
      <DataTable
        caption="Instruments"
        columns={[
          {
            key: "platform",
            header: "Platform",
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
            headerContent: (
              <TableFilterHeader
                label="Symbol"
                display={filters.qDraft || "All"}
                active={Boolean(q || filters.qDraft)}
                open={filters.open === "q"}
                type="search"
                searchDraft={filters.qDraft}
                searchPlaceholder="Symbol or pair, e.g. BTCUSDT"
                searchHelp="Matches contract symbol, ignoring case."
                onToggle={() => filters.toggle("q")}
                onClose={filters.close}
                onClear={() => filters.change({ q: "" })}
                onSearchChange={filters.editQuery}
                onCompositionStart={filters.beginComposition}
                onCompositionEnd={filters.endComposition}
              />
            ),
          },
          { key: "kind", header: "Kind" },
          { key: "pair", header: "Pair" },
          { key: "settlement", header: "Settlement" },
          { key: "expiryAt", header: "Expiry" },
          { key: "contractSize", header: "Contract size", numeric: true },
          { key: "lastSeenAt", header: "Last seen", nowrap: true },
        ]}
        rows={(query.data?.items ?? []).map((item) => ({
          id: item.id,
          platform: platformLabel(platforms.data?.items, item.platform),
          symbol: item.symbol,
          kind: capitalize(item.kind.replaceAll("_", " ")),
          pair:
            item.baseSymbol || item.quoteSymbol
              ? `${item.baseSymbol ?? "—"} / ${item.quoteSymbol ?? "—"}`
              : "—",
          settlement: item.settlementSymbol ?? "—",
          expiryAt: item.expiryAt ? formatInstant(item.expiryAt, timezone) : "Perpetual",
          contractSize: formatDecimal(item.contractSize),
          lastSeenAt: formatInstant(item.lastSeenAt, timezone),
        }))}
        empty={
          query.isPending
            ? "Loading instruments."
            : q || platform
              ? "No instrument matches these filters."
              : "No instruments yet. Instruments appear after a Collection Cycle reports positions."
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
