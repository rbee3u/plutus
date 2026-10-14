import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { AccountValuation } from "@/api";
import { AccountValuationsGrouped } from "./AccountValuationsGrouped";

const sampleAccounts: AccountValuation[] = [
  {
    accountId: "acc-1",
    accountName: "Spot",
    accountType: "spot",
    selected: true,
    identityId: "id-1",
    identityName: "Binance hedging desk",
    platform: "binance",
    status: "succeeded",
    method: "calculated",
    freshness: "fresh",
    completeness: "complete",
    valueUsd: "6268.18",
    omitted: [],
  },
  {
    accountId: "acc-2",
    accountName: "Portfolio Margin",
    accountType: "portfolio_margin",
    selected: true,
    identityId: "id-1",
    identityName: "Binance hedging desk",
    platform: "binance",
    status: "succeeded",
    method: "reported",
    freshness: "fresh",
    completeness: "complete",
    valueUsd: "6363.92",
    omitted: [],
  },
  {
    accountId: "acc-3",
    accountName: "Spot",
    accountType: "spot",
    selected: true,
    identityId: "id-2",
    identityName: "OKX main desk",
    platform: "okx",
    status: "failed",
    errorClass: "API_UNAVAILABLE",
    errorDetail: "Network timeout",
    method: "missing",
    freshness: "stale",
    completeness: "incomplete",
    valueUsd: null,
    omitted: [],
  },
];

describe("AccountValuationsGrouped", () => {
  it("groups accounts by Platform and Identity into distinct cards with subtotals", () => {
    const html = renderToStaticMarkup(
      <AccountValuationsGrouped accounts={sampleAccounts} timezone="Asia/Shanghai" />,
    );

    expect(html).toContain("Binance");
    expect(html).toContain("Binance hedging desk");
    expect(html).toContain("OKX");
    expect(html).toContain("OKX main desk");

    // Subtotal of acc-1 ($6,268.18) + acc-2 ($6,363.92) = $12,632.10
    expect(html).toContain("12,632.10");

    // Healthy accounts show quiet dash
    expect(html).toContain("Spot");
    expect(html).toContain("Portfolio Margin");

    // Unhealthy account shows issues badge
    expect(html).toContain("Failed");
    expect(html).toContain("Stale");
    expect(html).toContain("Incomplete");
    expect(html).toContain("1 Issue");
  });

  it("renders empty state when no accounts exist", () => {
    const html = renderToStaticMarkup(
      <AccountValuationsGrouped accounts={[]} empty="No accounts valued yet." />,
    );
    expect(html).toContain("No accounts valued yet.");
  });

  it("allows custom account action rendering", () => {
    const html = renderToStaticMarkup(
      <AccountValuationsGrouped
        accounts={sampleAccounts.slice(0, 1)}
        renderAccountAction={(acc) => <button type="button">Custom {acc.accountName}</button>}
      />,
    );
    expect(html).toContain("Custom Spot");
  });
});
