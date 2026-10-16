import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { AccountValuation } from "@/api";
import { AccountValuationsGrouped } from "./AccountValuationsGrouped";

const sampleAccounts: AccountValuation[] = [
  {
    sourceReportId: null,
    reportId: null,
    reportObservedAt: null,
    reportedEquityUsd: null,
    adjustedEquityUsd: null,
    availableEquityUsd: null,
    initialMarginRatio: null,
    maintenanceMarginRatio: null,
    marginRatio: null,
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
    sourceReportId: null,
    reportId: null,
    reportObservedAt: null,
    reportedEquityUsd: null,
    adjustedEquityUsd: null,
    availableEquityUsd: null,
    initialMarginRatio: null,
    maintenanceMarginRatio: null,
    marginRatio: null,
    accountId: "acc-2",
    accountName: "Portfolio margin",
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
    sourceReportId: null,
    reportId: null,
    reportObservedAt: null,
    reportedEquityUsd: null,
    adjustedEquityUsd: null,
    availableEquityUsd: null,
    initialMarginRatio: null,
    maintenanceMarginRatio: null,
    marginRatio: null,
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
  it("groups accounts by platform and identity into distinct cards with subtotals", () => {
    const html = renderToStaticMarkup(
      <AccountValuationsGrouped accounts={sampleAccounts} timezone="Asia/Shanghai" />,
    );

    expect(html).toContain("Binance");
    expect(html).toContain("Binance hedging desk");
    expect(html).toContain("OKX");
    expect(html).toContain("OKX main desk");

    // Subtotal of acc-1 ($6,268.18) + acc-2 ($6,363.92) = $12,632.1
    expect(html).toContain("12,632.1");

    // Healthy accounts show quiet dash
    expect(html).toContain("Spot");
    expect(html).toContain("Portfolio margin");

    // Unhealthy account shows issues badge
    expect(html).toContain("Failed");
    expect(html).toContain("Missing report");
    expect(html).not.toContain("Stale");
    expect(html).toContain("Incomplete");
    expect(html).toContain("1 issue");
  });

  it.each(["calculated", "missing"] as const)(
    "keeps stale feedback for a successful source with %s valuation",
    (method) => {
      const html = renderToStaticMarkup(
        <AccountValuationsGrouped
          accounts={[
            {
              ...sampleAccounts[2],
              method,
              reportObservedAt: "2026-10-13T02:59:02Z",
            },
          ]}
        />,
      );
      expect(html).toContain("Stale report for Spot");
      expect(html).not.toContain("Missing report");
    },
  );

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
