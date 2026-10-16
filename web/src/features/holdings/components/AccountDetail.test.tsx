import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AccountValuation, Issue } from "@/api";
import { AccountDetail } from "./AccountDetail";
import { NO_ACCOUNT_REPORT } from "@/shared/lib/text";

const report = vi.hoisted(() => ({
  kind: "futures",
  balanceIssues: [] as Issue[],
  productIssues: [] as Issue[],
}));

vi.mock("../hooks/useAccountReport", () => ({
  useAccountReport: () => {
    const query = { isSuccess: true, isPending: false, isError: false };
    const context = { timezone: "UTC" };
    return {
      balances: { ...query, data: { context, items: [], issues: report.balanceIssues } },
      positions: { ...query, data: { context, items: [], issues: report.productIssues } },
      dualPositions: { ...query, data: { context, items: [], issues: report.productIssues } },
      earnPositions: { ...query, data: { context, items: [], issues: report.productIssues } },
      isManual: report.kind === "manual",
      isDual: report.kind === "dual",
      isEarn: report.kind === "earn",
    };
  },
}));

beforeEach(() => {
  report.kind = "futures";
  report.balanceIssues = [];
  report.productIssues = [];
});

const account: AccountValuation = {
  accountId: "account",
  accountName: "Trading",
  accountType: "trading",
  identityId: "identity",
  identityName: "Owner",
  platform: "okx",
  selected: true,
  status: "succeeded",
  method: "reported",
  freshness: "fresh",
  completeness: "complete",
  marginRatio: "4.2",
  omitted: [],
};

function markup(item: AccountValuation) {
  return renderToStaticMarkup(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter>
        <AccountDetail
          portfolioId="portfolio"
          account={item}
          cycleId="cycle"
          valuationRevision="cycle:1"
        />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function render(item: AccountValuation) {
  return markup(item).replace(/<[^>]*>/g, "");
}

describe("Account holding issues", () => {
  it.each(["futures", "earn", "dual"])(
    "describes missing %s reports as unknown holdings, not stale or empty",
    (kind) => {
      report.kind = kind;
      const missing = { code: "missing", detail: "Account value is unknown." };
      report.balanceIssues = [missing];
      report.productIssues = [missing];
      const text = render({
        ...account,
        reportId: "failed-report",
        method: "missing",
        freshness: "stale",
        status: "failed",
        completeness: "incomplete",
        reportObservedAt: null,
      });
      expect(text).toContain("holdings and value are unknown");
      expect(text.split(NO_ACCOUNT_REPORT)).toHaveLength(3);
      expect(text).toContain("Failed");
      expect(text).toContain("Incomplete");
      expect(text).not.toContain("Stale");
      expect(text).not.toContain("Earlier successful account report retained");
      expect(text).not.toContain("No balances in the current account report");
      expect(text).not.toContain("No open positions in the current account report");
    },
  );

  it.each(["fresh", "stale"] as const)(
    "keeps confirmed-empty wording for a %s successful source",
    (freshness) => {
      const text = render({
        ...account,
        reportObservedAt: "2026-10-13T02:59:02Z",
        freshness,
        status: freshness === "stale" ? "failed" : "succeeded",
      });
      expect(text).toContain("No balances in the current account report");
      expect(text).not.toContain(NO_ACCOUNT_REPORT);
      expect(text.includes("Earlier successful account report retained")).toBe(
        freshness === "stale",
      );
    },
  );

  it("describes an uncollected empty manual account by its current records", () => {
    report.kind = "manual";
    const text = render({ ...account, accountType: "manual", method: "missing", reportId: null });
    expect(text).toContain("No manual balances entered.");
    expect(text).not.toContain(NO_ACCOUNT_REPORT);
    expect(text).not.toContain("No balances in the current account report");
  });

  it.each([
    ["futures", "Futures"],
    ["earn", "Earn"],
    ["dual", "Dual investment"],
  ] as const)("keeps %s issues in their own section, including empty reports", (kind, label) => {
    report.kind = kind;
    const shared = { code: "stale", detail: "Earlier report retained." };
    report.balanceIssues = [shared, { code: "no_price", detail: "Balance price unavailable." }];
    report.productIssues = [
      shared,
      { code: "metadata_missing", detail: "Product terms unavailable." },
    ];
    const html = markup(account);
    const section = (name: string) =>
      html.match(new RegExp(`<section[^>]*aria-label="${name}">([\\s\\S]*?)</section>`))?.[1] ?? "";
    const balances = section("Balances");
    const product = section(label);
    for (const content of [balances, product]) {
      expect(content.match(/Earlier report retained\./g)).toHaveLength(1);
      expect(content.indexOf("</table>")).toBeLessThan(content.indexOf(">Issues</h4>"));
    }
    expect(balances).toContain("Balance price unavailable.");
    expect(balances).not.toContain("Product terms unavailable.");
    expect(product).toContain("Product terms unavailable.");
    expect(product).not.toContain("Balance price unavailable.");
  });

  it("keeps manual balance issues without creating a product section", () => {
    report.kind = "manual";
    report.balanceIssues = [{ code: "no_price", detail: "Balance price unavailable." }];
    const html = markup({ ...account, accountType: "manual" });
    expect(html).toContain("Balance price unavailable.");
    for (const label of ["Futures", "Earn", "Dual investment"]) {
      expect(html).not.toContain(`aria-label="${label}"`);
    }
  });
});

describe("Account margin measures", () => {
  it.each([
    ["binance", "cross_margin", "Margin level", "999"],
    ["binance", "portfolio_margin", "UniMMR", "4.2"],
    ["okx", "trading", "Margin ratio", "4.2"],
  ])(
    "labels %s %s by its scope and leaves the source ratio unscaled",
    (platform, accountType, label, value) => {
      for (const freshness of ["fresh", "stale"] as const) {
        const text = render({
          ...account,
          platform,
          accountType,
          marginRatio: value,
          freshness,
          status: freshness === "stale" ? "failed" : "succeeded",
        });
        expect(text).toContain(`${label}${value}`);
        expect(text).not.toContain("%");
        for (const other of ["Margin level", "UniMMR", "Margin ratio"]) {
          if (other !== label) expect(text).not.toContain(other);
        }
      }
    },
  );

  it("omits unknown measures while preserving a reported zero", () => {
    expect(render({ ...account, marginRatio: null })).not.toContain("Margin ratio");
    expect(render({ ...account, marginRatio: "0" })).toContain("Margin ratio0");
  });
});

describe("Account feedback scope", () => {
  it.each(["unconfigured", "disabled", "invalid"])(
    "keeps %s connection notices on platform accounts only",
    (connectionStatus) => {
      expect(render({ ...account, connectionStatus })).toContain(
        `Connection is ${connectionStatus}.`,
      );
      report.kind = "manual";
      expect(
        render({ ...account, accountType: "manual", reportId: "report", connectionStatus }),
      ).not.toContain("Connection is");
    },
  );

  it.each([
    { selected: false, message: "Account is not selected." },
    { identityStatus: "archived", message: "Identity is archived." },
  ])("retains manual account boundary feedback: $message", ({ message, ...overrides }) => {
    report.kind = "manual";
    const text = render({
      ...account,
      accountType: "manual",
      reportId: "report",
      connectionStatus: "disabled",
      ...overrides,
    });
    expect(text).toContain(message);
    expect(text).not.toContain("Connection is");
  });

  it("retains manual report health and valuation omissions", () => {
    report.kind = "manual";
    const text = render({
      ...account,
      accountType: "manual",
      reportId: "report",
      connectionStatus: "invalid",
      freshness: "stale",
      completeness: "incomplete",
      errorClass: "no_price",
      errorDetail: "No eligible price is available.",
      omitted: [{ assetSymbol: "BTC", equity: "1", reason: "no_price" }],
    });
    expect(text).toContain("Earlier successful account report retained");
    expect(text).toContain("No eligible price is available.");
    expect(text).toContain("Omitted from the total:");
    expect(text).toContain("BTC");
    expect(text).not.toContain("Connection is");
  });
});
