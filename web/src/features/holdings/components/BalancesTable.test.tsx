import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { BalanceRow } from "@/api";
import { DataTable } from "@/shared/ui/DataTable";
import { BALANCE_COLUMNS, balanceRow } from "./BalancesTable";

const balance: BalanceRow = {
  interest: null,
  accountId: "account",
  accountName: "Funding",
  accountType: "funding",
  identityId: "identity",
  identityName: "Owner",
  platform: "okx",
  assetId: "asset",
  assetSymbol: "XBT",
  canonicalAssetSymbol: "BTC",
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
  it("prioritizes USD valuation and displays quiet Issues column when healthy", () => {
    expect(BALANCE_COLUMNS.map(({ key, header }) => [key, header])).toEqual([
      ["symbol", "Asset"],
      ["equity", "Equity"],
      ["priceUsd", "Price (USD)"],
      ["equityUsd", "Equity (USD)"],
      ["liability", "Liability"],
      ["unrealizedPnl", "Unrealized PnL"],
      ["riskExposure", "Risk exposure"],
      ["issues", "Issues"],
    ]);
    const row = balanceRow(balance, "Asia/Shanghai");
    expect(row.equity).toBe("0.123457");
    expect(row.priceUsd).toBe("50,000");
    expect(row.equityUsd).toBe("6,172.84");
    expect(row.liability).toBe("0");
    expect(row.unrealizedPnl).toBe("—");
    expect(row.riskExposure).toBe("—");
    const symbol = renderToStaticMarkup(<>{row.symbol}</>);
    expect(symbol).toContain("XBT");
    expect(symbol.indexOf("XBT")).toBeLessThan(symbol.indexOf("Canonical asset: BTC"));
    expect(symbol).toContain("Canonical asset: BTC");
    const issues = renderToStaticMarkup(<>{row.issues}</>);
    expect(issues.replace(/<[^>]*>/g, "")).toBe("—");
    expect(issues).not.toContain("Stale");
    expect(issues).not.toContain(">fresh</span>");
    // Keep one composition check; the named cells above own their financial values.
    expect(render(balance)).toContain("6,172.84");
  });

  it("exposes a stale disclosure in Issues for carried-forward reports", () => {
    const row = balanceRow({ ...balance, freshness: "stale" }, "Asia/Shanghai");
    const html = renderToStaticMarkup(<>{row.issues}</>);
    expect(html).toContain('aria-label="Stale report for XBT"');
    expect(html).toContain("Stale</span>");
  });

  it("discloses unpaid interest inside the liability and never adds a column", () => {
    const row = balanceRow({ ...balance, liability: "10.5", interest: "0.5" }, "Asia/Shanghai");
    const html = renderToStaticMarkup(<>{row.liability}</>);
    expect(html).toContain('aria-label="XBT liability 10.5"');
    expect(html).toContain("10.5");
    const withoutInterest = balanceRow(
      { ...balance, liability: "10.5", interest: "0" },
      "Asia/Shanghai",
    );
    expect(withoutInterest.liability).toBe("10.5");
  });

  it("keeps healthy issue column quiet and unpriced equity explicit", () => {
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
    expect(renderToStaticMarkup(<>{row.priceUsd}</>)).toContain("Missing price");
    const issues = renderToStaticMarkup(<>{row.issues}</>);
    expect(issues.replace(/<[^>]*>/g, "")).toBe("—");
  });

  it("renders the signed exposure without profit color and explains an unavailable value", () => {
    const known = balanceRow(
      { ...balance, riskExposure: "-1.20309", priceSource: "spot" },
      "Asia/Shanghai",
    );
    expect(known.riskExposure).toBe("-1.20309");
    expect(renderToStaticMarkup(<>{known.riskExposure}</>)).not.toContain("pnlNegative");

    const unavailable = balanceRow(
      {
        ...balance,
        riskExposure: null,
        riskExposureUnavailableReason: "not_unified",
      },
      "Asia/Shanghai",
    );
    const html = renderToStaticMarkup(<>{unavailable.riskExposure}</>);
    expect(html).toContain('aria-label="XBT risk exposure unavailable"');
    expect(html.replace(/<[^>]*>/g, "")).toBe("—");
  });

  it("marks a contract-only row and uses the contract mark price with a disclosure", () => {
    const item: BalanceRow = {
      ...balance,
      equity: "0",
      liability: "0",
      equityUsd: "0",
      riskExposure: "0.5",
      contractOnly: true,
      noPrice: true,
      priceUsd: "50000",
      priceSource: "contract",
      priceSourceSymbol: "XBTUSDT",
    };
    const row = balanceRow(item, "Asia/Shanghai");
    expect(renderToStaticMarkup(<>{row.symbol}</>)).toContain("Contract only");
    const price = renderToStaticMarkup(<>{row.priceUsd}</>);
    expect(price).toContain('aria-label="XBT contract mark price source"');
    expect(price).toContain("50,000");
    expect(price).not.toContain("Missing price");
  });
});
