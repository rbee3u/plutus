import { expect, test, type Page } from "@playwright/test";
import type {
  CapitalFlow,
  Overview,
  Portfolio,
  ValuationPointSummary,
} from "../src/api/generated/types.gen";

const portfolioId = "00000000-0000-7000-8000-000000000001";
const portfolio: Portfolio = {
  id: portfolioId,
  name: "Numbers check",
  timezone: "UTC",
  collectionIntervalSeconds: 3600,
  undeletable: true,
  version: 1,
  createdAt: "2026-10-03T00:00:00Z",
  updatedAt: "2026-10-03T00:00:00Z",
};
const point: ValuationPointSummary = {
  id: "00000000-0000-7000-8000-000000000003",
  jobId: "00000000-0000-7000-8000-000000000004",
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
  context: { portfolioId, timezone: "UTC" },
  point,
  totalUsd: point.totalUsd,
  completeness: "complete",
  accountCount: 1234,
  staleAccountCount: 0,
  incompleteAccountCount: 0,
  byPlatform: [
    {
      key: "okx",
      label: "OKX",
      valueUsd: point.totalUsd,
      accountCount: 1234,
      rowCount: 1234,
      completeness: "complete",
    },
  ],
  byIdentity: [],
  issues: [],
  sparkline: [
    {
      ...point,
      totalUsd: "1.234999999999999999999999999999",
      evaluatedAt: "2026-10-02T00:00:00Z",
    },
    point,
  ],
};
const flow: CapitalFlow = {
  id: "00000000-0000-7000-8000-000000000005",
  portfolioId,
  direction: "contribution",
  assetSymbol: "BTC",
  quantity: "0.000000004444567890123456789123",
  valueUsd: "1.005",
  effectiveAt: "2026-10-03T00:00:00Z",
  version: 1,
  createdAt: "2026-10-03T00:00:00Z",
  updatedAt: "2026-10-03T00:00:00Z",
};

async function mockApi(page: Page) {
  let submitted: unknown;
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (path === "/api/v1/auth/me")
      return route.fulfill({
        json: {
          user: { id: "user", username: "numbers-check", status: "active" },
          sessionCount: 1,
        },
      });
    if (path === "/api/v1/auth/csrf")
      return route.fulfill({ json: { csrfToken: "offline-check" } });
    if (path === "/api/v1/portfolios") return route.fulfill({ json: { items: [portfolio] } });
    if (path === `/api/v1/portfolios/${portfolioId}`) return route.fulfill({ json: portfolio });
    if (path.endsWith("/overview")) return route.fulfill({ json: overview });
    if (
      path === "/api/v1/platforms" ||
      path.endsWith("/identities") ||
      path.endsWith("/boundary-events")
    )
      return route.fulfill({ json: { items: [] } });
    if (path.endsWith(`/capital-flows/${flow.id}`)) {
      if (request.method() === "PATCH") submitted = request.postDataJSON();
      return route.fulfill({ json: flow, headers: { ETag: '"capital-flow:1"' } });
    }
    if (path.endsWith("/capital-flows")) return route.fulfill({ json: { items: [flow] } });
    throw new Error(`Unexpected request ${request.method()} ${path}`);
  });
  return () => submitted;
}

test("cards, tables and source-based chart tooltips share rounding and readable subscripts", async ({
  page,
}, testInfo) => {
  await mockApi(page);
  await page.goto(`/p/${portfolioId}/overview`);
  const total = page
    .getByRole("article")
    .filter({ has: page.getByRole("heading", { name: "Total value", exact: true }) });
  await expect(total.locator('[aria-hidden="true"]')).toHaveText("0.0₅444");
  await expect(total).toMatchAriaSnapshot(
    '- article:\n  - heading "Total value" [level=2]\n  - paragraph: 0.00000444 USD\n  - paragraph: complete',
  );
  await expect(
    page.getByRole("table").first().getByRole("cell", { name: "0.00000444", exact: true }),
  ).toHaveCount(1);
  await expect(
    page
      .getByRole("article")
      .filter({ has: page.getByRole("heading", { name: "Accounts", exact: true }) }),
  ).toContainText("1,234");
  const chart = page.getByRole("img", { name: "Recent Valuation Points" });
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
        const countBox = await page
          .getByRole("article")
          .filter({ has: page.getByRole("heading", { name: "Accounts", exact: true }) })
          .boundingBox();
        return valueBox != null && countBox != null && countBox.y >= valueBox.y + valueBox.height;
      })
      .toBe(true);
    await total.scrollIntoViewIfNeeded();
    await expect(total.locator('[aria-hidden="true"]')).toHaveText("0.0₅444");
    await expect
      .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth))
      .toBe(true);
    await page.screenshot({ path: testInfo.outputPath(`numbers-${theme}.png`) });
  }
});

test("Capital Flow editing and submission retain source precision while display is rounded", async ({
  page,
}) => {
  const submitted = await mockApi(page);
  await page.goto(`/p/${portfolioId}/capital`);
  const row = page
    .getByRole("row")
    .filter({ has: page.getByRole("button", { name: "Edit", exact: true }) });
  await expect(row).toContainText("0.0₈444457");
  await expect(row).toContainText("1.01");
  await row.getByRole("button", { name: "Edit", exact: true }).click();
  await expect(page.getByRole("textbox", { name: "Quantity", exact: true })).toHaveValue(
    flow.quantity,
  );
  await expect(page.getByRole("textbox", { name: "Value (USD, optional)" })).toHaveValue("1.005");
  await page.getByRole("button", { name: "Save flow", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeHidden();
  expect(submitted()).toMatchObject({ quantity: flow.quantity, valueUsd: "1.005" });
});
