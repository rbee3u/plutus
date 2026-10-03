import { expect, test } from "@playwright/test";
import type { PositionList, PositionRow } from "../src/api/generated/types.gen";

const portfolioId = "00000000-0000-7000-8000-000000000001";
const position: PositionRow = {
  accountId: "00000000-0000-7000-8000-000000000003",
  accountName: "Trading",
  accountType: "trading",
  identityId: "00000000-0000-7000-8000-000000000004",
  identityName: "Owner",
  platform: "okx",
  platformPositionRef: "BTC-USD-SWAP:net",
  instrumentId: "00000000-0000-7000-8000-000000000005",
  symbol: "BTC-USD-SWAP",
  kind: "inverse_perpetual",
  side: "short",
  positionMode: "one_way",
  quantity: "2",
  quantityUnit: "contracts",
  size: "-200",
  sizeSymbol: "USD",
  baseSymbol: "BTC",
  quoteSymbol: "USD",
  settlementSymbol: "BTC",
  contractSize: "100",
  leverage: "3",
  entryPrice: "50000.123456789123456789",
  markPrice: "50000",
  marginMode: "cross",
  marginBasis: "initial",
  marginCurrency: "USD",
  margin: "100",
  settleMargin: "0.002",
  portfolioMargin: false,
  freshness: "stale",
  reportObservedAt: "2026-10-03T00:00:00Z",
  notionalUsd: "200",
  issues: [{ code: "stale", detail: "Temporary collection failure; previous report retained." }],
};

const delivery: PositionRow = {
  ...position,
  instrumentId: "00000000-0000-7000-8000-000000000007",
  platformPositionRef: "BTCUSD_261225:SHORT",
  symbol: "BTCUSD_261225",
  kind: "inverse_delivery",
  expiryAt: "2026-12-25T08:00:00Z",
  positionMode: null,
  freshness: "fresh",
  size: "200",
  unrealizedPnl: "0.0000000043485748",
  issues: [{ code: "position_mode_unknown", detail: "Position mode was not recorded." }],
};

test("position details work with hover, keyboard, click, scroll, and narrow tables", async ({
  page,
}) => {
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/api/v1/auth/me")
      return route.fulfill({
        json: {
          user: { id: "user", username: "browser-check", status: "active" },
          sessionCount: 1,
        },
      });
    if (path === "/api/v1/portfolios")
      return route.fulfill({
        json: {
          items: [
            {
              id: portfolioId,
              name: "Positions check",
              timezone: "Asia/Shanghai",
              collectionIntervalSeconds: 3600,
              undeletable: true,
              version: 1,
              createdAt: "2026-10-03T00:00:00Z",
              updatedAt: "2026-10-03T00:00:00Z",
            },
          ],
        },
      });
    if (path === "/api/v1/platforms")
      return route.fulfill({
        json: { items: [{ id: "okx", name: "OKX", accountStructures: [] }] },
      });
    if (path.endsWith("/identities")) return route.fulfill({ json: { items: [] } });
    if (path.endsWith("/positions"))
      return route.fulfill({
        json: {
          context: {
            portfolioId,
            timezone: "Asia/Shanghai",
            pointId: "00000000-0000-7000-8000-000000000006",
          },
          items: [position, delivery],
          totalNotionalUsd: "400",
          completeness: "complete",
          issues: position.issues,
          page: { kind: "offset", limit: 50, offset: 0, total: 2 },
        } satisfies PositionList,
      });
    throw new Error(`Unexpected request ${path}`);
  });
  await page.setViewportSize({ width: 1280, height: 800 });
  await page.goto(`/p/${portfolioId}/holdings/positions`);
  const symbol = page.getByRole("button", { name: "Contract details for BTC-USD-SWAP" });
  await expect(page.getByRole("columnheader")).toHaveText([
    "Platform",
    "Identity",
    "Account",
    "Symbol",
    "Side",
    "Size",
    "Entry Price",
    "Mark Price",
    "Liq. Price",
    "Margin",
    "Unrealized PnL",
    "Notional (USD)",
    "Observed",
  ]);
  await symbol.hover();
  const details = page.getByRole("tooltip");
  await expect(details).toContainText("1 contract = 100 USD");
  await expect(details).toContainText("Inverse");
  await expect(details).toContainText("Perpetual");
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
  await expect(page.getByRole("button", { name: "Full amount" })).toHaveCount(0);
  await page.getByRole("button", { name: "Contract details for BTCUSD_261225" }).click();
  await expect(details).toContainText("Delivery");
  await expect(details).toContainText("2026-12-25 16:00 Asia/Shanghai");
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "Data issues for BTCUSD_261225" }).click();
  await expect(details).toContainText("Position mode was not recorded.");
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "Margin details for BTC-USD-SWAP" }).click();
  await expect(details).toContainText("Reported: 100 USD");
  await expect(details).toContainText("using Mark Price");
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "Stale report for BTC-USD-SWAP" }).click();
  await expect(details).toContainText("Temporary collection failure");
  await page.keyboard.press("Escape");
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByRole("columnheader")).toHaveCount(13);
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
