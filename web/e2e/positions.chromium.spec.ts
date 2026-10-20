import { mockApi } from "./support/api";
import { sessionFixture } from "./support/session";
import { expect, test } from "@playwright/test";
import type { PositionList, PositionRow } from "../src/api/generated/types.gen";

const portfolioId = "00000000-0000-7000-8000-000000000001";
const position: PositionRow = {
  liquidationPrice: null,
  unrealizedPnl: null,
  accountId: "00000000-0000-7000-8000-000000000003",
  accountName: "Trading",
  accountType: "trading",
  identityId: "00000000-0000-7000-8000-000000000004",
  identityName: "Owner",
  platform: "okx",
  positionKey: "BTC-USD-SWAP:net",
  futuresContractId: "00000000-0000-7000-8000-000000000005",
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
  contractSize: "100",
  displayLeverage: "3",
  marginUnavailableCode: null,
  reportedLeverage: "3",
  entryPrice: "50000.123456789123456789",
  markPrice: "50000",
  marginSetting: "cross",
  marginBasis: "initial",
  marginAssetSymbol: "USD",
  reportedMargin: "100",
  settlementMargin: "0.002",
  marginSource: "reported",
  portfolioMargin: false,
  freshness: "stale",
  reportObservedAt: "2026-10-03T00:00:00Z",
  notionalUsd: "200",
  issues: [{ code: "stale", detail: "Temporary collection failure; previous report retained." }],
};

const delivery: PositionRow = {
  ...position,
  futuresContractId: "00000000-0000-7000-8000-000000000007",
  positionKey: "BTCUSD_261225:SHORT",
  futuresSymbol: "BTCUSD_261225",
  kind: "inverse_delivery",
  expiryAt: "2026-12-25T08:00:00Z",
  positionMode: null,
  freshness: "fresh",
  size: "200",
  unrealizedPnl: "0.0000000043485748",
  issues: [{ code: "position_mode_unknown", detail: "Position mode was not recorded." }],
};

const calculated: PositionRow = {
  ...position,
  displayLeverage: null,
  futuresSymbol: "ZECUSDT",
  kind: "linear_perpetual",
  platform: "binance",
  accountType: "portfolio_margin",
  positionKey: "um:ZECUSDT:BOTH",
  futuresContractId: "00000000-0000-7000-8000-000000000008",
  baseAssetSymbol: "ZEC",
  quoteAssetSymbol: "USDT",
  settlementAssetSymbol: "USDT",
  sourceQuantity: "0.5",
  sourceQuantityUnit: "base",
  size: "-0.5",
  sizeSymbol: "ZEC",
  contractSize: null,
  markPrice: "1317.81",
  reportedLeverage: "3",
  portfolioMargin: true,
  settlementMargin: "219.635",
  reportedMargin: null,
  marginAssetSymbol: null,
  marginBasis: null,
  marginSource: "calculated",
  freshness: "fresh",
  issues: [],
};

const isolated: PositionRow = {
  ...position,
  displayLeverage: "1.88",
  futuresSymbol: "ETHUSDT",
  kind: "linear_perpetual",
  baseAssetSymbol: "ETH",
  quoteAssetSymbol: "USDT",
  sourceQuantityUnit: "base",
  sizeSymbol: "ETH",
  notionalUsd: "1880",
  reportedLeverage: "10",
  positionKey: "um:ETHUSDT:LONG",
  futuresContractId: "00000000-0000-7000-8000-000000000009",
  marginSetting: "isolated",
  marginBasis: "allocated",
  reportedMargin: "1000",
  marginAssetSymbol: "USDT",
  settlementAssetSymbol: "USDT",
  settlementMargin: "1000",
  portfolioMargin: true,
};

const unavailable: PositionRow = {
  ...position,
  displayLeverage: null,
  marginUnavailableCode: "shared_portfolio_margin",
  futuresSymbol: "BTCUSDT",
  positionKey: "BTCUSDT:0",
  futuresContractId: "00000000-0000-7000-8000-000000000010",
  settlementMargin: null,
  marginSource: null,
  marginUnavailableReason:
    "This portfolio margin regime does not provide an independent position initial requirement.",
  portfolioMargin: true,
};

test("Position details work with hover, keyboard, click, scroll, and narrow tables", async ({
  page,
}) => {
  await mockApi(
    page,
    sessionFixture({
      user: { username: "browser-check" },
      portfolio: { id: portfolioId, name: "Positions check", timezone: "Asia/Shanghai" },
    }),
    async (route) => {
      const path = new URL(route.request().url()).pathname;
      if (path === "/api/v1/platforms")
        return route.fulfill({
          json: { items: [{ id: "okx", name: "OKX", accountStructures: [] }] },
        });
      if (path.endsWith("/identities")) return route.fulfill({ json: { items: [] } });
      if (path.endsWith("/positions"))
        return route.fulfill({
          json: {
            context: {
              evaluatedAt: null,
              portfolioId,
              timezone: "Asia/Shanghai",
              cycleId: "00000000-0000-7000-8000-000000000006",
            },
            items: [position, delivery, calculated, isolated, unavailable],
            filteredTotalUsd: "1000",
            coverage: { complete: true, missingAccountIds: [] },
            filteredTotalCompleteness: "complete",
            filteredTotalFreshness: "fresh",
            filteredTotalIssues: [],
            issues: position.issues,
            page: { kind: "offset", limit: 50, offset: 0, total: 5 },
          } satisfies PositionList,
        });
      return route.fallback();
    },
  );
  await page.setViewportSize({ width: 1280, height: 800 });
  await page.goto(`/p/${portfolioId}/holdings/futures`);
  const symbol = page.getByRole("button", { name: "Contract details for BTC-USD-SWAP" });
  await expect(page.getByRole("columnheader")).toHaveText([
    /Platform/,
    /Identity/,
    "Account",
    "Symbol",
    "Side",
    "Size",
    "Entry price",
    "Mark price",
    "Unrealized PnL",
    "Margin",
    "Issues",
  ]);
  await symbol.hover();
  const details = page.getByRole("tooltip");
  await expect(details).toContainText("100 USD");
  await expect(details).not.toContainText("contract =");
  await expect(details).toContainText("Inverse");
  await expect(details).toContainText("Perpetual");
  await expect(details).not.toContainText("Leverage");
  await page.keyboard.press("Escape");
  await expect(details).toBeHidden();
  await symbol.focus();
  await expect(details).toBeVisible();
  await page.keyboard.press("Escape");
  await symbol.click();
  await expect(details).toBeVisible();
  await page.keyboard.press("Escape");
  const row = page.getByRole("row").filter({ has: symbol });
  await expect(row.getByRole("cell").nth(6)).toHaveText("50,000.12");
  await page
    .getByRole("button", { name: "Entry price details for BTC-USD-SWAP: 50,000.12" })
    .click();
  await expect(details).toHaveText("Notional: $200");
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "Mark price details for BTC-USD-SWAP: 50,000" }).click();
  await expect(details).toHaveText("Notional: $200");
  await page.keyboard.press("Escape");
  await expect(page.getByRole("button", { name: "Full amount" })).toHaveCount(0);
  await page.getByRole("button", { name: "Contract details for BTCUSD_261225" }).click();
  await expect(details).toContainText("Delivery");
  await expect(details).toContainText("2026-12-25 16:00");
  await expect(details).not.toContainText("Asia/Shanghai");
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "Data issues for BTCUSD_261225" }).click();
  await expect(details).toContainText("Position mode was not recorded.");
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "Margin details for BTC-USD-SWAP" }).click();
  await expect(details).toHaveText("3x cross");
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "Margin details for ZECUSDT" }).click();
  await expect(details).toHaveText("portfolio margin");
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "Margin details for BTCUSDT" }).focus();
  await expect(details).toHaveText("portfolio margin");
  await page.keyboard.press("Escape");
  const amounts = page.getByRole("button", { name: /^Margin details for/ });
  const rightEdges = await amounts.evaluateAll((elements) =>
    elements.map((element) => element.getBoundingClientRect().right),
  );
  expect(Math.max(...rightEdges) - Math.min(...rightEdges)).toBeLessThan(0.5);
  await expect(page.getByRole("button", { name: "Margin details for ZECUSDT" })).toHaveText(
    "219.635 USDT",
  );
  await expect(page.getByRole("button", { name: "Margin details for ETHUSDT" })).toHaveText(
    "1,000 USDT",
  );
  await expect(page.getByRole("button", { name: "Margin details for BTC-USD-SWAP" })).toHaveText(
    "0.002 BTC",
  );
  await expect(page.getByRole("button", { name: "Margin details for BTCUSDT" })).toHaveText("—");
  // The margin column needs only its longest amount and the table's cell padding.
  // This catches both an empty tag track and spare-width auto-layout expansion.
  for (const width of [1280, 1920]) {
    await page.setViewportSize({ width, height: 800 });
    const widths = await amounts.evaluateAll((elements) => ({
      longest: Math.max(...elements.map((element) => element.getBoundingClientRect().width)),
      column: elements[0].parentElement!.getBoundingClientRect().width,
      padding:
        parseFloat(getComputedStyle(elements[0].parentElement!).paddingLeft) +
        parseFloat(getComputedStyle(elements[0].parentElement!).paddingRight),
    }));
    expect(widths.column - widths.longest - widths.padding).toBeLessThan(2);
    const contentOverhang = await page.getByRole("row").evaluateAll((rows) => {
      let maxOverhang = 0;
      for (const row of rows) {
        const cells = Array.from(row.children) as HTMLElement[];
        for (const cell of cells) {
          const cellRight = cell.getBoundingClientRect().right;
          for (const child of Array.from(cell.children) as HTMLElement[]) {
            const childRight = child.getBoundingClientRect().right;
            if (childRight > cellRight + 1) {
              maxOverhang = Math.max(maxOverhang, childRight - cellRight);
            }
          }
        }
      }
      return maxOverhang;
    });
    expect(contentOverhang).toBe(0);
  }
  await page.getByRole("button", { name: "Margin details for ETHUSDT" }).click();
  await expect(details).toHaveText("1.88x isolated");
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "Stale report for BTC-USD-SWAP" }).click();
  await expect(details).toContainText("Temporary collection failure");
  await page.keyboard.press("Escape");
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByRole("columnheader")).toHaveCount(11);
  await expect
    .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth))
    .toBe(true);
  await symbol.click();
  await expect(details).toBeVisible();
  const table = page.getByRole("table").locator("..");
  await table.evaluate((element) => {
    element.scrollLeft += 120;
  });
  await expect(details).toBeHidden();
});
