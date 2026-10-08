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
    expect(BALANCE_COLUMNS.map(({ key, header }) => [key, header])).toEqual([
      ["symbol", "Asset"],
      ["equity", "Equity"],
      ["priceUsd", "Price (USD)"],
      ["equityUsd", "Equity (USD)"],
      ["liability", "Liability"],
      ["unrealizedPnl", "Unrealized PnL"],
      ["observed", "Observed"],
    ]);
    const row = balanceRow(balance, "Asia/Shanghai");
    expect(row.equity).toBe("0.123457");
    expect(row.priceUsd).toBe("50,000");
    expect(row.equityUsd).toBe("6,172.84");
    expect(row.liability).toBe("0");
    expect(row.unrealizedPnl).toBe("—");
    const symbol = renderToStaticMarkup(<>{row.symbol}</>);
    expect(symbol).toContain("XBT");
    expect(symbol).toContain("as BTC");
    const observed = renderToStaticMarkup(<>{row.observed}</>);
    expect(observed).toContain("2026-10-03 08:00");
    expect(observed).not.toContain("Stale");
    expect(observed).not.toContain(">fresh</span>");
    // Keep one composition check; the named cells above own their financial values.
    expect(render(balance)).toContain("6,172.84");
  });

  it("retains the carried-forward time and exposes a Stale disclosure", () => {
    const row = balanceRow({ ...balance, freshness: "stale" }, "Asia/Shanghai");
    const html = renderToStaticMarkup(<>{row.observed}</>);
    expect(html).toContain("2026-10-03 08:00");
    expect(html).toContain('aria-label="Stale report for XBT"');
    expect(html).toContain("Stale</span>");
  });

  it("keeps unknown time and unpriced equity explicit", () => {
    const row = balanceRow(
      {
        ...balance,
        reportObservedAt: null,
        noPrice: true,
        priceUsd: null,
        equityUsd: null,
      },
      "Asia/Shanghai",
    );
    expect(row.equity).toBe("0.123457");
    expect(row.equityUsd).toBe("—");
    expect(renderToStaticMarkup(<>{row.priceUsd}</>)).toContain("no price");
    const observed = renderToStaticMarkup(<>{row.observed}</>);
    expect(observed).toContain("Account Report observed at an unknown time");
    expect(observed.replace(/<[^>]*>/g, "")).toBe("—");
  });
});
