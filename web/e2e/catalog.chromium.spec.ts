import { expect, test } from "@playwright/test";
import type { Asset, AssetList } from "../src/api";
import { mockApi } from "./support/api";
import { sessionFixture } from "./support/session";

test("Catalog status filters reset pagination, survive reload, and clear independently", async ({
  page,
}) => {
  const session = sessionFixture();
  const portfolioId = session.portfolios.items[0].id;
  const assets: Asset[] = [false, true].map((excluded) => ({
    id: excluded ? "excluded" : "active",
    platform: "binance",
    symbol: excluded ? "TOKEN-X" : "TOKEN-A",
    canonicalAssetSymbol: excluded ? "TOKEN-X" : "TOKEN-A",
    mappingSource: "auto",
    firstObservedAt: "2026-10-03T00:00:00Z",
    lastObservedAt: "2026-10-03T00:00:00Z",
    mappingVersion: 1,
    excluded,
    exclusionVersion: excluded ? 1 : 0,
  }));
  const reads: URL[] = [];
  await mockApi(page, session, async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname === "/api/v1/platforms")
      return route.fulfill({
        json: { items: [{ id: "binance", name: "Binance", accountStructures: [] }] },
      });
    if (url.pathname === `/api/v1/portfolios/${portfolioId}/catalog/assets`) {
      reads.push(url);
      const status = url.searchParams.get("status");
      const items = assets.filter(
        (asset) => status === null || asset.excluded === (status === "excluded"),
      );
      const offset = Number(url.searchParams.get("offset"));
      const body: AssetList = {
        items: items.slice(offset),
        page: { kind: "offset", offset, limit: 50, total: items.length },
      };
      return route.fulfill({ json: body });
    }
    return route.fallback();
  });

  const base = `/p/${portfolioId}/data/catalog`;
  await page.goto(`${base}?platform=binance&q=TOKEN&offset=50`);
  await expect(page.getByRole("button", { name: "Filter Status: All" })).toBeVisible();
  await page.getByRole("button", { name: "Filter Status: All" }).click();
  await page.getByRole("option", { name: "Active", exact: true }).click();
  await expect(page).toHaveURL(`${base}?platform=binance&q=TOKEN&status=active`);
  await expect(page.getByRole("cell", { name: "TOKEN-A", exact: true })).toBeVisible();
  await expect(page.getByRole("cell", { name: "TOKEN-X", exact: true })).toHaveCount(0);
  expect(reads.at(-1)?.searchParams.get("status")).toBe("active");
  expect(reads.at(-1)?.searchParams.get("offset")).toBe("0");

  await page.getByRole("button", { name: "Filter Status: Active" }).click();
  await page.getByRole("option", { name: "Excluded", exact: true }).click();
  await expect(page.getByRole("cell", { name: "TOKEN-X", exact: true })).toBeVisible();
  await expect(page.getByRole("cell", { name: "TOKEN-A", exact: true })).toHaveCount(0);
  expect(reads.at(-1)?.searchParams.get("status")).toBe("excluded");
  await page.reload();
  await expect(page.getByRole("button", { name: "Filter Status: Excluded" })).toBeVisible();
  await expect(page.getByRole("cell", { name: "TOKEN-X", exact: true })).toBeVisible();

  await page.getByRole("button", { name: "Filter Status: Excluded" }).click();
  await page.getByRole("button", { name: "Clear Status filter", exact: true }).click();
  await expect(page).toHaveURL(`${base}?platform=binance&q=TOKEN`);
  await expect(page.getByRole("cell", { name: "TOKEN-A", exact: true })).toBeVisible();
  await expect(page.getByRole("cell", { name: "TOKEN-X", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Clear filters", exact: true }).click();
  await expect(page).toHaveURL(base);
  await expect(page.getByRole("button", { name: "Filter Platform: All" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Search Asset", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Filter Status: All" })).toBeVisible();
});
