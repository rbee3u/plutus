import { MemoryRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { EarnPositionRow } from "@/api";
import { EarnPositionsTable, EarnBalanceSummaries } from "./EarnPositionsTable";
import { DualPositionsTable } from "./DualPositionsTable";

const item: EarnPositionRow = {
  accountId: "account",
  accountName: "Earn",
  accountType: "earn",
  identityId: "identity",
  identityName: "Owner",
  platform: "okx",
  platformPositionRef: "onchain:one",
  type: "on_chain",
  granularity: "position",
  amountBasis: "principal",
  productName: "Pool",
  assets: [
    { symbol: "ETH", amount: "1" },
    { symbol: "USDT", amount: "2" },
  ],
  rates: [{ value: "0.04", kind: "apy", qualifier: "estimated", label: "Estimated APY" }],
  rewards: [
    { symbol: "USDT", amount: "0.2", kind: "cumulative" },
    { symbol: "COMP", amount: "0.1", kind: "estimated" },
  ],
  details: [],
  freshness: "fresh",
  reportObservedAt: "2026-10-03T00:00:00Z",
  issues: [],
};

function render(value: React.ReactNode) {
  return renderToStaticMarkup(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter>{value}</MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("Earn presentation", () => {
  it("keeps one multiasset product row, distinct reward scopes and Portfolio times", () => {
    const html = render(<EarnPositionsTable items={[item]} showAccount timezone="Asia/Shanghai" />);
    const headers = [...html.matchAll(/<th\b[^>]*>(.*?)<\/th>/g)].map((m) => m[1]);
    expect(headers).toEqual([
      "Platform",
      "Identity",
      "Asset",
      "Type",
      "Maturity",
      "Rate",
      "Amount",
      "Rewards",
      "Subscribed",
      "Status",
      "Observed",
    ]);
    expect(html.match(/<tr>/g)).toHaveLength(2);
    expect(html).toContain("1 ETH");
    expect(html).toContain("2 USDT");
    expect(html).toContain("Est. 4% APY");
    expect(html).toContain("Cumulative");
    expect(html).toContain("Estimated");
    expect(html).toContain("2026-10-03 08:00");
    expect(html).not.toContain("1 issue");
  });
  it("omits ownership in Account details and does not alarm on ordinary missing terms", () => {
    const html = render(
      <EarnPositionsTable
        items={[{ ...item, type: "flexible", rates: [], rewards: [], status: null }]}
      />,
    );
    expect(html).not.toContain(">Platform</th>");
    expect(html).not.toContain(">Identity</th>");
    expect(html).not.toContain("issue");
  });
  it("keeps stale and metadata indicators in Observed", () => {
    const html = render(
      <EarnPositionsTable
        items={[
          {
            ...item,
            freshness: "stale",
            status: "Redeeming",
            issues: [
              { code: "stale", detail: "Earlier report." },
              { code: "earn_metadata_unavailable", detail: "APR unavailable." },
            ],
          },
        ]}
      />,
    );
    const headers = [...html.matchAll(/<th\b[^>]*>(.*?)<\/th>/g)].map((m) => m[1]);
    const cells = [...html.matchAll(/<td\b[^>]*>(.*?)<\/td>/g)].map((m) => m[1]);
    const status = cells[headers.indexOf("Status")];
    const observed = cells[headers.indexOf("Observed")];
    expect(status).toContain("Redeeming");
    expect(status).not.toContain("Stale");
    expect(status).not.toContain("issue");
    expect(observed).toContain("Stale");
    expect(observed).toContain("1 issue");
  });
  it("labels legacy Balance summaries separately", () => {
    const html = render(
      <EarnBalanceSummaries
        items={[
          {
            accountId: "old",
            accountName: "Earn",
            identityName: "Owner",
            platform: "okx",
            assets: [{ symbol: "USDT", amount: "12" }],
            freshness: "fresh",
            issues: [{ code: "earn_details_unavailable", detail: "Not collected." }],
          },
        ]}
      />,
    );
    expect(html).toContain("Balance summaries, not product positions");
    expect(html).toContain("12 USDT");
    expect(html).not.toContain(">Type</th>");
  });
  it("labels Dual APR accurately and retains product status independently", () => {
    const html = render(
      <DualPositionsTable
        timezone="Asia/Shanghai"
        items={[
          {
            accountId: "dual",
            accountName: "Dual",
            accountType: "dual_investment",
            identityId: "identity",
            identityName: "Owner",
            platform: "binance",
            platformPositionRef: "one",
            investSymbol: "USDT",
            principal: "10",
            annualizedRate: "0.18",
            rateKind: "apr",
            status: "active",
            freshness: "stale",
            reportObservedAt: "2026-10-03T00:00:00Z",
            issues: [{ code: "dual_terms_missing", detail: "Expiry unavailable." }],
          },
        ]}
      />,
    );
    expect(html).toContain(">Rate</th>");
    expect(html).toContain(">Observed</th>");
    expect(html).toContain("18% APR");
    expect(html).toContain("Active");
    expect(html).toContain("Stale");
    expect(html).toContain("1 issue");
  });
});
