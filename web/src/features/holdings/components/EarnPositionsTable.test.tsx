import { MemoryRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { EarnPositionRow } from "@/api";
import { EarnPositionsTable } from "./EarnPositionsTable";
import { DualPositionsTable } from "./DualPositionsTable";

const item: EarnPositionRow = {
  accountId: "account",
  accountName: "Earn",
  accountType: "earn",
  identityId: "identity",
  identityName: "Owner",
  platform: "okx",
  positionKey: "onchain:one",
  type: "on_chain",
  granularity: "position",
  amountBasis: "principal",
  productName: "Pool",
  assets: [
    { assetSymbol: "ZETH", canonicalAssetSymbol: "ETH", amount: "1" },
    { assetSymbol: "USDT", amount: "2" },
  ],
  rates: [{ value: "0.04", kind: "apy", qualifier: "estimated", label: "Estimated APY" }],
  rewards: [
    { assetSymbol: "USDT", amount: "0.2", kind: "cumulative" },
    { assetSymbol: "COMP", amount: "0.1", kind: "estimated" },
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
  it("keeps one multiasset product row, distinct reward scopes and portfolio times", () => {
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
      "Issues",
    ]);
    expect(html.match(/<tr>/g)).toHaveLength(2);
    expect(html).toContain("1 ZETH");
    expect(html).toContain("Canonical asset: ETH");
    expect(html).not.toContain("1 ETH");
    expect(html).not.toContain("Canonical asset: USDT");
    expect(html).toContain("2 USDT");
    expect(html).toContain("Est. 4% APY");
    expect(html).toContain("Cumulative");
    expect(html).toContain("Estimated");
    expect(html).toContain("—");
    expect(html).not.toContain("1 issue");
  });
  it("omits ownership in account details and does not alarm on ordinary missing terms", () => {
    const html = render(
      <EarnPositionsTable
        items={[{ ...item, type: "flexible", rates: [], rewards: [], status: null }]}
      />,
    );
    expect(html).not.toContain(">platform</th>");
    expect(html).not.toContain(">identity</th>");
    expect(html).not.toContain("issue");
  });
  it("keeps stale and metadata indicators in Issues", () => {
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
    const issues = cells[headers.indexOf("Issues")];
    expect(status).toContain("Redeeming");
    expect(status).not.toContain("Stale");
    expect(status).not.toContain("issue");
    expect(issues).toContain("Stale");
    expect(issues).toContain("1 issue");
  });
  it("labels dual APR accurately and retains product status independently", () => {
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
            positionKey: "one",
            principalAssetSymbol: "USDT",
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
    expect(html).toContain(">Issues</th>");
    expect(html).toContain("18% APR");
    expect(html).toContain("Active");
    expect(html).toContain("Stale");
    expect(html).toContain("1 issue");
  });
});
