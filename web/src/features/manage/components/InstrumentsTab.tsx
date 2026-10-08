import { keys, listInstruments } from "@/api";
import type { InstrumentList } from "@/api";
import { formatDecimal } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { capitalize } from "@/shared/lib/text";
import { platformLabel, usePlatforms } from "@/shared/lib/usePlatforms";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { DataTable } from "@/shared/ui/DataTable";
import { Pager } from "@/shared/ui/Pager";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { PAGE_SIZE, type TabProps } from "../lib/catalog";

export function InstrumentsTab({ platform, q, offset, timezone, onOffset }: TabProps) {
  const platforms = usePlatforms();
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
          { key: "platform", header: "Platform" },
          { key: "symbol", header: "Symbol" },
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
      <Pager
        offset={query.data?.page.offset ?? offset}
        limit={query.data?.page.limit ?? PAGE_SIZE}
        total={query.data?.page.total ?? 0}
        onChange={onOffset}
      />
    </>
  );
}
