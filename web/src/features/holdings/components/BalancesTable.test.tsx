import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { BalanceRow } from "@/api";
import { DataTable } from "@/shared/ui/DataTable";
import { BALANCE_COLUMNS, balanceRow } from "./BalancesTable";

const balance: BalanceRow = {
  accountId: "account",
  accountName: "Funding",
  accountType: "funding",
  identityId: "identity",
  identityName: "Owner",
  platform: "okx",
  listingId: "listing",
  symbol: "XBT",
  assetSymbol: "BTC",
  equity: "0.123456789",
  availableToWithdraw: "0.12",
  availableToTrade: "0.1",
  liability: "0",
  unrealizedPnl: null,
  priceUsd: "50000",
  equityUsd: "6172.83945",
  noPrice: false,
  freshness: "fresh",
  reportObservedAt: "2026-10-03T00:00:00Z",
};

function render(item: BalanceRow) {
  return renderToStaticMarkup(
    <DataTable
      caption="Balances"
      columns={BALANCE_COLUMNS}
      rows={[balanceRow(item, "Asia/Shanghai")]}
    />,
  );
}

describe("shared balance cells", () => {
  it("prioritizes USD valuation and displays the source time without a fresh badge", () => {
    expect(BALANCE_COLUMNS.map((column) => column.header)).toEqual([
      "Asset",
      "Equity",
      "Price (USD)",
      "Equity (USD)",
      "Liability",
      "Unrealized PnL",
      "Observed",
    ]);
    const html = render(balance);
    expect(html).toContain("XBT");
    expect(html).toContain("as BTC");
    expect(html).toContain("0.123457");
    expect(html).toContain("6,172.84");
    expect(html).toContain("2026-10-03 08:00");
    expect(html).not.toContain("Withdrawable");
    expect(html).not.toContain("Tradable");
    expect(html).not.toContain("Freshness");
    expect(html).not.toContain(">fresh</span>");
    expect(html).not.toContain("Stale report");
  });

  it("retains the carried-forward time and exposes a Stale disclosure", () => {
    const html = render({ ...balance, freshness: "stale" });
    expect(html).toContain("2026-10-03 08:00");
    expect(html).toContain('aria-label="Stale report for XBT"');
    expect(html).toContain("Stale</span>");
  });

  it("keeps unknown time and unpriced equity explicit", () => {
    const html = render({
      ...balance,
      reportObservedAt: null,
      noPrice: true,
      priceUsd: null,
      equityUsd: null,
    });
    expect(html).toContain("no price");
    expect(html).toContain("Account Report observed at an unknown time");
    expect(html).not.toContain("2026-10-03 08:00");
    expect(html.match(/<td[^>]*>—<\/td>/g)).toHaveLength(2);
  });
});
