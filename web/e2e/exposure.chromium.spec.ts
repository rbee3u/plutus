import { mockApi } from "./support/api";
import { sessionFixture } from "./support/session";
import { expect, test, type Page } from "@playwright/test";
import type { ExposureDetail, ExposureList } from "../src/api/generated/types.gen";

const portfolioId = "00000000-0000-7000-8000-000000000001";
const binanceIdentity = "00000000-0000-7000-8000-000000000002";
const binanceAccount = "00000000-0000-7000-8000-000000000003";

const list: ExposureList = {
  context: {
    portfolioId,
    cycleId: "00000000-0000-7000-8000-000000000010",
    evaluatedAt: "2026-11-24T02:30:00Z",
    timezone: "Asia/Shanghai",
  },
  summary: {
    assetCount: 6,
    longCount: 2,
    shortCount: 1,
    flatCount: 1,
    partialCount: 1,
    unknownCount: 1,
    completeness: "incomplete",
    freshness: "stale",
  },
  coverage: {
    complete: false,
    missingAccountIds: ["00000000-0000-7000-8000-000000000099"],
    missingAccountCount: 1,
    staleAccountCount: 1,
    excludedAssetCount: 1,
    unattributedPositionCount: 0,
  },
  items: [
    {
      canonicalAssetSymbol: "BTC",
      platformSymbols: ["BTC", "XBT"],
      netQuantity: "0.5",
      state: "complete",
      priceUsd: "68000",
      priceSource: "spot",
      priceSourceSymbol: "BTC",
      priceSourcePlatform: "binance",
      valueUsd: "34000",
      accountCount: 2,
      unknownAccountCount: 0,
      freshness: "fresh",
      issues: [],
    },
    {
      canonicalAssetSymbol: "DOGE",
      platformSymbols: ["DOGE"],
      netQuantity: "12000",
      state: "partial",
      priceUsd: "0.1421",
      priceSource: "spot",
      priceSourceSymbol: "DOGE",
      priceSourcePlatform: "binance",
      valueUsd: "1705.2",
      accountCount: 2,
      unknownAccountCount: 1,
      freshness: "fresh",
      issues: [
        {
          code: "exposure_unknown_position",
          accountId: binanceAccount,
          accountName: "Unified",
          symbol: "DOGE",
          detail: "position inputs unavailable",
        },
      ],
    },
    {
      canonicalAssetSymbol: "ETH",
      platformSymbols: ["ETH"],
      netQuantity: "-1",
      state: "complete",
      priceUsd: "3300",
      priceSource: "spot",
      priceSourceSymbol: "ETH",
      priceSourcePlatform: "binance",
      valueUsd: "-3300",
      accountCount: 1,
      unknownAccountCount: 0,
      freshness: "fresh",
      issues: [],
    },
    {
      canonicalAssetSymbol: "AVAX",
      platformSymbols: ["AVAX"],
      netQuantity: "0",
      state: "complete",
      priceUsd: "34.2",
      priceSource: "spot",
      priceSourceSymbol: "AVAX",
      priceSourcePlatform: "binance",
      valueUsd: "0",
      accountCount: 1,
      unknownAccountCount: 0,
      freshness: "fresh",
      issues: [],
    },
    {
      canonicalAssetSymbol: "USDT",
      platformSymbols: ["USDT"],
      netQuantity: "1000",
      state: "complete",
      priceUsd: "1",
      priceSource: "spot",
      priceSourceSymbol: "USDT",
      priceSourcePlatform: "binance",
      valueUsd: "1000",
      accountCount: 1,
      unknownAccountCount: 0,
      freshness: "stale",
      issues: [],
    },
    {
      canonicalAssetSymbol: "LINK",
      platformSymbols: ["LINK"],
      netQuantity: null,
      state: "unknown",
      priceUsd: null,
      priceSource: null,
      priceSourceSymbol: null,
      priceSourcePlatform: null,
      valueUsd: null,
      accountCount: 1,
      unknownAccountCount: 1,
      freshness: "fresh",
      issues: [
        {
          code: "exposure_unknown_position",
          accountId: binanceAccount,
          accountName: "Unified",
          symbol: "LINK",
          detail: "position inputs unavailable",
        },
      ],
    },
  ],
  page: { kind: "offset", offset: 0, limit: 50, total: 6 },
  issues: [
    {
      code: "missing",
      detail:
        "No compatible successful account report supplies holdings for the filtered exposure total.",
      accountId: "00000000-0000-7000-8000-000000000099",
      accountName: "Bybit never collected",
    },
    {
      code: "stale",
      detail:
        "The account report failed this cycle; the previous successful report is carried forward.",
      accountId: "00000000-0000-7000-8000-000000000098",
      accountName: "Binance delayed reports",
    },
  ],
};

const detail: ExposureDetail = {
  context: list.context,
  canonicalAssetSymbol: "BTC",
  platformSymbols: ["BTC", "XBT"],
  netQuantity: "0.5",
  state: "complete",
  accountCount: 2,
  unknownAccountCount: 0,
  freshness: "fresh",
  coverage: list.coverage,
  contributions: [
    {
      platform: "binance",
      platformLabel: "Binance",
      identityId: binanceIdentity,
      identityName: "main",
      accountId: binanceAccount,
      accountName: "Spot",
      accountType: "spot",
      accountStructure: "binance_classic",
      assetSymbol: "BTC",
      quantity: "0.3",
      state: "complete",
      contractOnly: false,
      freshness: "fresh",
      reportObservedAt: "2026-11-24T02:29:00Z",
      issues: [],
    },
    {
      platform: "bybit",
      platformLabel: "Bybit",
      identityId: "00000000-0000-7000-8000-000000000004",
      identityName: "main",
      accountId: "00000000-0000-7000-8000-000000000005",
      accountName: "Unified",
      accountType: "unified",
      accountStructure: "bybit_uta_2_0",
      assetSymbol: "BTC",
      quantity: "0.2",
      state: "complete",
      contractOnly: true,
      freshness: "fresh",
      reportObservedAt: "2026-11-24T02:29:00Z",
      issues: [],
    },
  ],
  issues: [],
};

async function fixture(page: Page) {
  await mockApi(
    page,
    sessionFixture({
      user: { username: "exposure" },
      portfolio: { id: portfolioId, name: "Exposure", timezone: "Asia/Shanghai" },
    }),
    async (route) => {
      const url = new URL(route.request().url());
      const path = url.pathname;
      if (path === "/api/v1/platforms") {
        return route.fulfill({
          json: {
            items: [
              { id: "binance", name: "Binance", accountStructures: [] },
              { id: "bybit", name: "Bybit", accountStructures: [] },
            ],
          },
        });
      }
      if (path.endsWith("/identities")) {
        return route.fulfill({
          json: {
            items: [
              {
                id: binanceIdentity,
                platform: "binance",
                displayName: "main",
                status: "enabled",
                accounts: [],
                connection: { status: "enabled" },
              },
            ],
          },
        });
      }
      if (path === `/api/v1/portfolios/${portfolioId}/exposure`) {
        return route.fulfill({ json: list });
      }
      if (path === `/api/v1/portfolios/${portfolioId}/exposure/BTC`) {
        return route.fulfill({ json: detail });
      }
      return route.fallback();
    },
  );
}

test("Exposure discloses states, reference values, and account contributions", async ({ page }) => {
  await fixture(page);
  await page.goto(`/p/${portfolioId}/holdings/exposure`);

  await expect(page.getByText("PORTFOLIO EXPOSURE", { exact: true })).toBeVisible();
  await expect(page.getByText("2 long · 1 short · 1 flat", { exact: true })).toBeVisible();
  await expect(page.getByText("1 partial", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("1 unknown", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("Incomplete", { exact: true })).toBeVisible();
  await expect(page.getByText("Stale", { exact: true }).first()).toBeVisible();
  await expect(page.getByRole("columnheader")).toHaveText([
    "",
    "Asset",
    "Net exposure",
    "Price (USD)",
    "Value (USD)",
    "Accounts",
    "Issues",
  ]);
  await expect(page.getByRole("row")).toHaveCount(7);
  await expect(page.getByText("Partial", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("Unknown", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("Flat", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("Known totals may omit their holdings.")).toBeVisible();
  await expect(page.getByText("1 excluded asset omitted")).toBeVisible();

  await page.getByRole("row").filter({ hasText: "BTC" }).click();
  await expect(page.getByText("Contributions", { exact: true })).toBeVisible();
  await expect(page.getByText("Binance", { exact: true })).toBeVisible();
  await expect(page.getByText("Bybit", { exact: true })).toBeVisible();
  await expect(page.getByText("Contract only", { exact: true })).toBeVisible();
  await expect(page.getByText("0.3", { exact: true })).toBeVisible();
  await expect(page.getByText("0.2", { exact: true })).toBeVisible();
  await expect(page.getByText("0.5", { exact: true }).last()).toBeVisible();

  await page.getByRole("button", { name: "How it works" }).click();
  await expect(page.getByRole("dialog")).toContainText(
    "Portfolio exposure is the signed coin quantity",
  );
  await expect(page.getByRole("dialog")).toContainText("Options are not collected");
  await page.keyboard.press("Escape");
});

test("Exposure uses cards and keeps contributions on mobile", async ({ page }) => {
  await fixture(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`/p/${portfolioId}/holdings/exposure`);
  await expect(page.getByRole("table")).toHaveCount(0);
  await page.getByRole("button", { name: "Expand BTC contributions" }).click();
  await expect(page.getByText("Contributions", { exact: true })).toHaveCount(0);
  await expect(page.getByText("Binance · main · Spot")).toBeVisible();
  await expect(page.getByText("Bybit · main · Unified")).toBeVisible();
  await expect(page.getByText("Contract only", { exact: true })).toBeVisible();
  await expect
    .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth))
    .toBe(true);
});
