import { mockApi } from "./support/api";
import { sessionFixture } from "./support/session";
import { overviewTreeFixture } from "./fixtures/overviewTree";
import { expect, test, type Page } from "@playwright/test";
import type { CapitalFlow, Overview, ValuationPointSummary } from "../src/api/generated/types.gen";

const portfolioId = "00000000-0000-7000-8000-000000000001";
const session = sessionFixture({
  user: { username: "numbers-check" },
  portfolio: { id: portfolioId, name: "Numbers check" },
});
const point: ValuationPointSummary = {
  cycleId: "00000000-0000-7000-8000-000000000003",

  calculationVersion: 1,
  detailState: "full" as const,
  evaluatedAt: "2026-10-03T00:00:00Z",
  totalUsd: "0.00000444",
  completeness: "complete",
  accountCount: 1234,
  staleAccountCount: 0,
  incompleteAccountCount: 0,
  boundaryContributionUsd: "0",
  boundaryWithdrawalUsd: "0",
};
const overview: Overview = {
  context: {
    evaluatedAt: null,
    portfolioId,
    timezone: "UTC",
    cycleId: point.cycleId,
  },
  point,
  platforms: overviewTreeFixture(point, [], "okx"),
  issues: [],
};
const series = [
  { ...point, totalUsd: "1.234999999999999999999999999999", evaluatedAt: "2026-10-02T00:00:00Z" },
  point,
];

const flow: CapitalFlow = {
  id: "00000000-0000-7000-8000-000000000005",
  portfolioId,
  direction: "contribution",
  pricingMethod: "explicit",
  pricingAttemptedAt: "2026-10-10T00:00:00Z",
  calculationVersion: 1,
  canonicalAssetSymbol: "BTC",
  quantity: "0.000000004444567890123456789123",
  valueUsd: "1.005",
  effectiveAt: "2026-10-03T00:00:00Z",
  version: 1,
  createdAt: "2026-10-03T00:00:00Z",
  updatedAt: "2026-10-03T00:00:00Z",
};

async function fixture(page: Page) {
  let submitted: unknown;
  await mockApi(page, session, async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (path.endsWith("/history-window"))
      return route.fulfill({
        json: {
          context: overview.context,
          from: "2026-09-30T00:00:00Z",
          to: "2026-10-03T00:00:00Z",
        },
      });
    if (path.endsWith("/valuation-points/series"))
      return route.fulfill({
        json: {
          context: overview.context,
          from: "2026-09-30T00:00:00Z",
          to: "2026-10-03T00:00:00Z",
          granularity: "raw",
          items: series,
        },
      });
    if (path.endsWith("/overview")) return route.fulfill({ json: overview });
    if (path.endsWith("/identities"))
      return route.fulfill({
        json: {
          items: [
            {
              id: "owner",
              portfolioId,
              platform: "okx",
              displayName: "Owner",
              accountStructure: "unified",
              status: "enabled",
              connection: { status: "enabled", credentialsConfigured: true },
              accounts: [],
              version: 1,
              createdAt: point.evaluatedAt,
              updatedAt: point.evaluatedAt,
            },
          ],
        },
      });
    if (path === "/api/v1/platforms") return route.fulfill({ json: { items: [] } });
    if (path.endsWith("/boundary-events"))
      return route.fulfill({
        json: {
          context: overview.context,
          from: "2026-09-30T00:00:00Z",
          to: "2026-10-03T00:00:00Z",
          items: [],
          page: { kind: "offset", offset: 0, limit: 50, total: 0 },
        },
      });
    if (path.endsWith(`/capital-flows/${flow.id}`)) {
      if (request.method() === "PATCH") submitted = request.postDataJSON();
      return route.fulfill({ json: flow, headers: { ETag: "1" } });
    }
    if (path.endsWith("/capital-flows"))
      return route.fulfill({
        json: {
          context: overview.context,
          from: "2026-09-30T00:00:00Z",
          to: "2026-10-03T00:00:00Z",
          items: [flow],
          page: { kind: "offset", offset: 0, limit: 50, total: 1 },
        },
      });
    return route.fallback();
  });
  return () => submitted;
}

test("Cards, tables and source-based chart tooltips share rounding and readable subscripts", async ({
  page,
}, testInfo) => {
  await fixture(page);
  await page.goto(`/p/${portfolioId}/overview`);
  const total = page
    .getByRole("article")
    .filter({ has: page.getByRole("heading", { name: "Total net worth", exact: true }) });
  await expect(total.locator('span[aria-hidden="true"]').first()).toHaveText("0.05444");
  await expect(total).toContainText("0.00000444");
  await expect(
    page
      .getByRole("button", { name: "Collapse OKX", exact: true })
      .locator('span[aria-hidden="true"]'),
  ).toHaveText("$0.05444");
  await expect(
    page.getByRole("button", { name: "Collapse OKX", exact: true }),
  ).toHaveAccessibleDescription("$0.00000444");
  await expect(total).toContainText("1,234 accounts");
  const chart = page.getByRole("img", { name: "Recent valuation points" });
  await chart.scrollIntoViewIfNeeded();
  const bounds = await chart.boundingBox();
  if (!bounds) throw new Error("Chart has no bounds");
  await page.mouse.move(bounds.x + bounds.width * 0.3, bounds.y + bounds.height / 2);
  // Plotting converts this source to 1.235; the tooltip must still round to 1.23.
  await expect(chart).toContainText("1.23");
  await page.mouse.move(bounds.x + bounds.width * 0.8, bounds.y + bounds.height / 2);
  await expect(chart).toContainText("0.0₅444");
  await page.mouse.move(0, 0);
  for (const theme of ["light", "dark"] as const) {
    await page.emulateMedia({ colorScheme: theme });
    await page.setViewportSize({ width: 375, height: 812 });
    await expect
      .poll(async () => {
        const valueBox = await total.boundingBox();
        const chartBox = await chart.boundingBox();
        return valueBox != null && chartBox != null && chartBox.y >= valueBox.y + valueBox.height;
      })
      .toBe(true);
    await total.scrollIntoViewIfNeeded();
    await expect(total.locator('span[aria-hidden="true"]').first()).toHaveText("0.05444");
    await expect
      .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth))
      .toBe(true);
    await page.screenshot({ path: testInfo.outputPath(`numbers-${theme}.png`) });
  }
});

test("Capital flow editing and submission retain source precision while display is rounded", async ({
  page,
}) => {
  const submitted = await fixture(page);
  await page.goto(`/p/${portfolioId}/capital`);
  const row = page
    .getByRole("row")
    .filter({ has: page.getByRole("button", { name: /^Actions for flow/i }) });
  await expect(row).toContainText("0.08444457");
  await expect(row).toContainText("1.01");
  await row.getByRole("button", { name: /^Actions for flow/i }).click();
  await page.getByRole("menuitem", { name: "Edit", exact: true }).click();
  await expect(page.getByRole("textbox", { name: "Quantity", exact: true })).toHaveValue(
    flow.quantity,
  );
  await expect(page.getByRole("textbox", { name: "Value (USD, optional)" })).toHaveValue("1.005");
  const editedQuantity = "0.000000004444567890123456789124";
  await page.getByRole("textbox", { name: "Quantity", exact: true }).fill(editedQuantity);
  await page.getByRole("textbox", { name: "Value (USD, optional)" }).fill("1.006");
  await page.getByRole("button", { name: "Save flow", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeHidden();
  expect(submitted()).toMatchObject({ quantity: editedQuantity, valueUsd: "1.006" });
});
