import { expect, test, type Page } from "@playwright/test";
import type { CurrentUser, Overview, Portfolio } from "../src/api/generated/types.gen";

const portfolioId = "00000000-0000-7000-8000-000000000001";
const userId = "00000000-0000-7000-8000-000000000002";
const portfolio = {
  id: portfolioId,
  name: "Browser check",
  timezone: "UTC",
  collectionIntervalSeconds: 3600,
  undeletable: true,
  version: 1,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
} satisfies Portfolio;
const currentUser = {
  user: { id: userId, username: "browser-check", status: "active" },
  sessionCount: 1,
} satisfies CurrentUser;
const overview = {
  context: { portfolioId, timezone: "UTC" },
  point: null,
  completeness: "complete",
  accountCount: 0,
  staleAccountCount: 0,
  incompleteAccountCount: 0,
  byPlatform: [],
  byIdentity: [],
  issues: [],
  sparkline: [],
} satisfies Overview;

async function mockShell(page: Page) {
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (request.method() !== "GET") {
      throw new Error(`Unexpected API request: ${request.method()} ${path}`);
    }
    if (path === "/api/v1/auth/me") {
      await route.fulfill({ json: currentUser });
    } else if (path === "/api/v1/portfolios") {
      await route.fulfill({ json: { items: [portfolio] } });
    } else if (path === `/api/v1/portfolios/${portfolioId}/overview`) {
      await route.fulfill({ json: overview });
    } else {
      throw new Error(`Unexpected API request: GET ${path}`);
    }
  });
}

test("mobile navigation manages visibility and focus across viewport changes", async ({ page }) => {
  await mockShell(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`/p/${portfolioId}/overview`);

  const sidebar = page.locator('aside[aria-label="Application navigation"]');
  const open = page.getByRole("button", { name: "Open navigation" });
  await expect(page.getByRole("heading", { name: "Overview" })).toBeVisible();
  await expect(open).toBeVisible();
  await expect(sidebar).toHaveAttribute("aria-hidden", "true");

  await open.click();
  await expect(sidebar).toBeVisible();
  await expect(open).toHaveAttribute("aria-expanded", "true");
  await expect(page.getByRole("link", { name: "Overview" })).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(sidebar).toHaveAttribute("aria-hidden", "true");
  await expect(open).toBeFocused();

  await page.setViewportSize({ width: 812, height: 375 });
  await open.click();
  await expect(page.getByRole("button", { name: "Close navigation" })).toBeVisible();
  await expect
    .poll(() => sidebar.evaluate((element) => getComputedStyle(element).overflowY))
    .toBe("auto");
  await page.getByRole("button", { name: "Close navigation" }).click();
  await expect(open).toBeFocused();
  await expect
    .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth))
    .toBe(true);

  await page.setViewportSize({ width: 1024, height: 768 });
  await expect(open).toBeHidden();
  await expect(sidebar).toBeVisible();
});
