import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { AccountValuation, Platform } from "@/api";
import { AccountValuationsTree } from "./AccountValuationsTree";

const platforms = [
  { id: "binance", name: "Binance" },
  { id: "okx", name: "OKX" },
] as Platform[];

const base: AccountValuation = {
  sourceReportId: null,
  reportId: "report-1",
  reportObservedAt: "2026-10-14T15:00:00Z",
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
  identityName: "Main",
  platform: "binance",
  status: "succeeded",
  method: "calculated",
  freshness: "fresh",
  completeness: "complete",
  valueUsd: "6268.18",
  omitted: [],
};

const account = (overrides: Partial<AccountValuation>): AccountValuation => ({
  ...base,
  ...overrides,
});

const margin = account({
  accountId: "acc-2",
  accountName: "Portfolio margin",
  accountType: "portfolio_margin",
  method: "reported",
  valueUsd: "6363.92",
});

const failed = account({
  accountId: "acc-3",
  accountName: "Funding",
  identityId: "id-2",
  identityName: "OKX main desk",
  platform: "okx",
  sourceReportId: null,
  reportObservedAt: null,
  status: "failed",
  method: "missing",
  freshness: "stale",
  completeness: "incomplete",
  valueUsd: null,
  errorClass: "API_UNAVAILABLE",
  errorDetail: "Network timeout",
});

const accounts = [base, margin, failed];

describe("AccountValuationsTree", () => {
  it("renders the platform, identity, and account hierarchy with subtotals", () => {
    const html = renderToStaticMarkup(
      <AccountValuationsTree
        accounts={accounts}
        platforms={platforms}
        timezone="Asia/Shanghai"
        renderDetail={(item) => <div>Detail for {item.accountName}</div>}
      />,
    );

    expect(html).toContain("Binance");
    expect(html).toContain("Main");
    expect(html).toContain("Portfolio margin");
    expect(html).toContain("OKX");
    expect(html).toContain("OKX main desk");
    expect(html).toContain("Funding");

    expect(html).toContain("$12,632.1");
    expect(html).toContain('aria-label="Collapse Binance"');
    expect(html).toContain('aria-label="Collapse Main"');
    expect(html).toContain('aria-label="Expand balances and positions for Portfolio margin"');
    expect(html).not.toContain("Detail for Portfolio margin");
  });

  it("shows identity issues as one compact badge and marks partial platform subtotals", () => {
    const html = renderToStaticMarkup(
      <AccountValuationsTree accounts={accounts} platforms={platforms} />,
    );

    expect(html).toContain("1 issue · Incomplete");
    expect(html).toContain("Failed");
    expect(html).toContain("Missing report");
    expect(html).toContain("Incomplete");
    expect(html).toContain("$12,632.1");
  });

  it("renders non-expandable account rows when no detail is supplied", () => {
    const html = renderToStaticMarkup(
      <AccountValuationsTree accounts={[base]} platforms={platforms} />,
    );

    expect(html).toContain("Spot");
    expect(html).toContain("$6,268.18");
    expect(html).not.toContain("Expand balances and positions for Spot");
  });

  it("keeps badge details operable on non-expandable rows", () => {
    const html = renderToStaticMarkup(
      <AccountValuationsTree accounts={[failed]} platforms={platforms} />,
    );

    expect(html).toContain('aria-label="Failed for Funding"');
    expect(html).toContain('aria-label="Missing report for Funding"');
    expect(html).not.toContain("Expand balances and positions for Funding");
  });

  it("renders the empty message inside the card", () => {
    const html = renderToStaticMarkup(
      <AccountValuationsTree accounts={[]} platforms={platforms} empty="No accounts valued yet." />,
    );

    expect(html).toContain("Accounts");
    expect(html).toContain("No accounts valued yet.");
  });
});
