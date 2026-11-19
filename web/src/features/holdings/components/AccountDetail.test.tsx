import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AccountValuation, Issue, PositionRow } from "@/api";
import { AccountDetail } from "./AccountDetail";
import { NO_ACCOUNT_REPORT } from "@/shared/lib/text";

const report = vi.hoisted(() => ({
  kind: "futures",
  balanceIssues: [] as Issue[],
  productIssues: [] as Issue[],
  positions: [] as PositionRow[],
  hasData: true,
  args: [] as unknown[],
}));

vi.mock("../hooks/useAccountReport", () => ({
  useAccountReport: (...args: unknown[]) => {
    report.args = args;
    const query = { isSuccess: true, isPending: false, isError: false };
    const context = { timezone: "UTC" };
    return {
      report: {
        ...query,
        data: report.hasData
          ? {
              context,
              balances: [],
              positions: report.positions,
              dualPositions: [],
              earnPositions: [],
              issues: [
                ...report.balanceIssues,
                ...report.productIssues.filter(
                  (issue) =>
                    !report.balanceIssues.some(
                      (existing) =>
                        existing.code === issue.code && existing.detail === issue.detail,
                    ),
                ),
              ],
            }
          : undefined,
      },
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
  report.positions = [];
  report.hasData = true;
});

const account: AccountValuation = {
  sourceReportId: null,
  reportId: null,
  valueUsd: null,
  reportObservedAt: null,
  reportedEquityUsd: null,
  adjustedEquityUsd: null,
  availableEquityUsd: null,
  initialMarginRatio: null,
  maintenanceMarginRatio: null,
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

const position: PositionRow = {
  displayLeverage: "3",
  marginUnavailableCode: null,
  contractSize: null,
  entryPrice: null,
  liquidationPrice: null,
  accountId: "account",
  accountName: "Trading",
  accountType: "portfolio_margin",
  identityId: "identity",
  identityName: "Owner",
  platform: "binance",
  positionKey: "BTC-USD-SWAP:net",
  futuresContractId: "futuresContract",
  futuresSymbol: "BTC-USD-SWAP",
  kind: "inverse_perpetual",
  side: "short",
  positionMode: "one_way",
  sourceQuantity: "2",
  sourceQuantityUnit: "contracts",
  size: "-200",
  sizeSymbol: "USD",
  baseAssetSymbol: "BTC",
  quoteAssetSymbol: "USD",
  settlementAssetSymbol: "BTC",
  markPrice: "50000",
  unrealizedPnl: "0.000123456789",
  marginSetting: "cross",
  portfolioMargin: true,
  settlementMargin: "0.002",
  reportedMargin: "100",
  marginAssetSymbol: "USD",
  marginBasis: "initial",
  marginSource: "reported",
  reportedLeverage: "3",
  notionalUsd: "50807.4868059",
  freshness: "fresh",
  reportObservedAt: "2026-10-03T00:00:00Z",
  issues: [],
};

const portfolioMargin: AccountValuation = {
  ...account,
  platform: "binance",
  accountType: "portfolio_margin",
  adjustedEquityUsd: "39349.44",
  marginRatio: "67.48",
};

const okxUnified: AccountValuation = {
  ...account,
  platform: "okx",
  accountType: "trading",
  adjustedEquityUsd: "39349.44",
  marginRatio: "67.48",
};

const krakenUnified: AccountValuation = {
  ...account,
  platform: "kraken",
  accountType: "main",
  adjustedEquityUsd: "39349.44",
  marginRatio: null,
  initialMarginRatio: "0.12",
  maintenanceMarginRatio: "0.04",
};

const bybitUnified: AccountValuation = {
  ...account,
  platform: "bybit",
  accountType: "unified",
  adjustedEquityUsd: "39349.44",
  marginRatio: null,
  initialMarginRatio: "0.1",
  maintenanceMarginRatio: "0.01",
};

function markup(item: AccountValuation, historical = false, highlightLeverageRisk = false) {
  return renderToStaticMarkup(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter>
        <AccountDetail
          portfolioId="portfolio"
          account={item}
          cycleId="cycle"
          historical={historical}
          highlightLeverageRisk={highlightLeverageRisk}
        />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function render(item: AccountValuation, historical = false) {
  return markup(item, historical).replace(/<[^>]*>/g, "");
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
      expect(text).not.toContain("No balances in this account report");
      expect(text).not.toContain("No open positions in this account report");
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
      expect(text).toContain("No balances in this account report");
      expect(text).not.toContain(NO_ACCOUNT_REPORT);
      expect(text.includes("Earlier successful account report retained")).toBe(
        freshness === "stale",
      );
    },
  );

  it("defaults historical reads to the point's manual state", () => {
    render({ ...account });
    expect(report.args[3]).toBe(true);
    render({ ...account }, true);
    expect(report.args[3]).toBe(false);
  });

  it("suppresses current connection and archive labels in history", () => {
    const item = {
      ...account,
      selected: false,
      identityStatus: "archived",
      connectionStatus: "invalid",
    };
    const current = render(item);
    expect(current).toContain("Account is not selected.");
    expect(current).toContain("Identity is archived.");
    expect(current).toContain("Connection is invalid.");
    const historical = render(item, true);
    expect(historical).not.toContain("Account is not selected.");
    expect(historical).not.toContain("Identity is archived.");
    expect(historical).not.toContain("Connection is invalid.");
  });

  it("describes an uncollected empty manual account by its current records", () => {
    report.kind = "manual";
    const text = render({ ...account, accountType: "manual", method: "missing", reportId: null });
    expect(text).toContain("No manual balances entered.");
    expect(text).not.toContain(NO_ACCOUNT_REPORT);
    expect(text).not.toContain("No balances in this account report");
  });

  it.each([
    ["futures", "Futures"],
    ["earn", "Earn"],
    ["dual", "Dual investment"],
  ] as const)("keeps %s issues in their own section, including empty reports", (kind, label) => {
    report.kind = kind;
    const shared = { code: "stale", detail: "Earlier report retained." };
    report.balanceIssues = [
      shared,
      {
        code: "no_price",
        detail: "Balance price unavailable.",
        omittedBalance: { assetSymbol: "DEMO", equity: "2", reason: "no_price" },
      },
    ];
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
    expect(product.includes("Balance price unavailable.")).toBe(kind !== "futures");
  });

  it("keeps balance-price omissions visible without opening an empty futures section", () => {
    report.balanceIssues = [
      {
        code: "no_price",
        detail: "Balance price unavailable.",
        omittedBalance: { assetSymbol: "DEMO", equity: "2", reason: "no_price" },
      },
    ];
    const html = markup(account);
    expect(html).toContain("Balance price unavailable.");
    expect(html).not.toContain('aria-label="Futures"');
  });

  it("keeps manual balance issues without creating a product section", () => {
    report.kind = "manual";
    report.balanceIssues = [
      {
        code: "no_price",
        detail: "Balance price unavailable.",
        omittedBalance: { assetSymbol: "DEMO", equity: "2", reason: "no_price" },
      },
    ];
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

describe("Account real leverage", () => {
  it("divides position notional by adjusted equity after UniMMR", () => {
    report.positions = [position];
    const text = render(portfolioMargin);
    expect(text).toContain("Real leverage1.29x");
    expect(text.indexOf("Real leverage")).toBeGreaterThan(text.indexOf("UniMMR"));
  });

  it.each([
    ["binance", "spot"],
    ["okx", "funding"],
    ["kraken", "coin_m"],
    ["bybit", "funding"],
  ])("omits the measure outside a supported unified scope: %s %s", (platform, accountType) => {
    report.positions = [position];
    expect(
      render({ ...account, platform, accountType, adjustedEquityUsd: "39349.44" }),
    ).not.toContain("Real leverage");
  });

  it.each([
    ["okx trading", okxUnified],
    ["kraken main", krakenUnified],
    ["bybit unified", bybitUnified],
  ])("shows the measure for %s", (_label, item) => {
    report.positions = [{ ...position, platform: item.platform, accountType: item.accountType }];
    expect(render(item)).toContain("Real leverage1.29x");
  });

  it("keeps Kraken separate-wallet main unknown", () => {
    report.positions = [position];
    expect(render({ ...krakenUnified, adjustedEquityUsd: null })).not.toContain("Real leverage");
  });

  it("shows zero leverage for a loaded empty position set", () => {
    expect(render(portfolioMargin)).toContain("Real leverage0x");
  });

  it("omits the measure when adjusted equity is unknown", () => {
    report.positions = [position];
    expect(render({ ...portfolioMargin, adjustedEquityUsd: null })).not.toContain("Real leverage");
  });

  it("omits the measure when a position notional is unknown", () => {
    report.positions = [{ ...position, notionalUsd: null }];
    expect(render(portfolioMargin)).not.toContain("Real leverage");
  });

  it("keeps the measure on a stale retained report", () => {
    report.positions = [position];
    const text = render({
      ...portfolioMargin,
      reportId: "report",
      reportObservedAt: "2026-10-03T00:00:00Z",
      freshness: "stale",
      status: "failed",
    });
    expect(text).toContain("Real leverage");
    expect(text).toContain("Earlier successful account report retained");
  });

  it("keeps the measure in history", () => {
    report.positions = [position];
    expect(render(portfolioMargin, true)).toContain("Real leverage");
  });

  it("does not render while the account report is loading", () => {
    report.hasData = false;
    expect(render(portfolioMargin)).not.toContain("Real leverage");
  });
});

describe("Account real leverage warning", () => {
  it("reddens the chip only when requested and above 3x", () => {
    report.positions = [{ ...position, notionalUsd: "324" }];
    const risky = { ...portfolioMargin, adjustedEquityUsd: "100" };

    const highlighted = markup(risky, false, true);
    expect(highlighted).toContain('data-leverage-risk="true"');
    expect(highlighted).toContain("Real leverage");
    expect(highlighted).toContain("3.24x");

    const neutral = markup(risky);
    expect(neutral).not.toContain('data-leverage-risk="true"');
    expect(neutral).toContain("Real leverage");
    expect(neutral).toContain("3.24x");
  });

  it("does not warn at exactly 3x", () => {
    report.positions = [{ ...position, notionalUsd: "300" }];
    const html = markup({ ...portfolioMargin, adjustedEquityUsd: "100" }, false, true);
    expect(html).toContain("Real leverage");
    expect(html).not.toContain('data-leverage-risk="true"');
  });
});
