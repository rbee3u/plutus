import { mockApi } from "./support/api";
import { sessionFixture } from "./support/session";
import { expect, test, type Page } from "@playwright/test";
import type { Overview } from "../src/api/generated/types.gen";

const portfolioId = "00000000-0000-7000-8000-000000000001";
const userId = "00000000-0000-7000-8000-000000000002";
const session = sessionFixture({
  user: { id: userId },
  portfolio: {
    id: portfolioId,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
});
const overview = {
  context: {
    cycleId: null,
    evaluatedAt: null,
    portfolioId,
    timezone: "UTC",
  },
  point: null,
  platforms: [],
  issues: [],
} satisfies Overview;

async function mockShell(page: Page, long = false) {
  const data: Overview = long
    ? {
        ...overview,
        context: {
          ...overview.context,
          evaluatedAt: "2026-01-01T00:00:00Z",
          cycleId: portfolioId,
        },
        point: {
          cycleId: portfolioId,

          calculationVersion: 1,
          detailState: "full" as const,
          evaluatedAt: "2026-01-01T00:00:00Z",
          totalUsd: "40",
          completeness: "complete",
          accountCount: 40,
          staleAccountCount: 0,
          incompleteAccountCount: 0,
          boundaryContributionUsd: "0",
          boundaryWithdrawalUsd: "0",
        },
        platforms: [
          {
            platform: "binance",
            label: "Binance",
            valueUsd: "40",
            completeness: "complete",
            identities: Array.from({ length: 40 }, (_, index) => ({
              id: `identity-${index}`,
              displayName: `Identity-${index}-${"x".repeat(60)}`,
              status: "enabled",
              connectionStatus: "enabled",
              valueUsd: "1",
              completeness: "complete",
              accounts: [],
            })),
          },
        ],
      }
    : overview;
  const scenarioSession = {
    ...session,
    portfolios: {
      items: [
        ...session.portfolios.items,
        ...(long
          ? Array.from({ length: 30 }, (_, index) => ({
              ...session.portfolios.items[0],
              id: `extra-${index}`,
              name: `Extra portfolio ${index}`,
            }))
          : []),
      ],
    },
  };
  await mockApi(page, scenarioSession, async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (request.method() !== "GET") {
      throw new Error(`Unexpected API request: ${request.method()} ${path}`);
    }
    if (path.endsWith("/identities")) {
      return route.fulfill({
        json: {
          items: data.platforms.flatMap((platform) =>
            platform.identities.map((identity) => ({
              id: identity.id,
              portfolioId,
              platform: platform.platform,
              displayName: identity.displayName,
              accountStructure: "classic",
              status: identity.status,
              connection: { status: identity.connectionStatus, credentialsConfigured: true },
              accounts: [],
              version: 1,
              createdAt: "2026-01-01T00:00:00Z",
              updatedAt: "2026-01-01T00:00:00Z",
            })),
          ),
        },
      });
    }
    if (path === `/api/v1/portfolios/${portfolioId}/history-window`) {
      await route.fulfill({
        json: { context: data.context, from: "2026-09-30T00:00:00Z", to: "2026-10-03T00:00:00Z" },
      });
    } else if (path === `/api/v1/portfolios/${portfolioId}/valuation-points/series`) {
      await route.fulfill({
        json: {
          context: data.context,
          from: "2026-09-30T00:00:00Z",
          to: "2026-10-03T00:00:00Z",
          granularity: "raw",
          items: [],
        },
      });
    } else if (path === `/api/v1/portfolios/${portfolioId}/overview`) {
      await route.fulfill({ json: data });
    } else {
      await route.fallback();
    }
  });
}

test("Mobile navigation manages visibility and focus across viewport changes", async ({ page }) => {
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

for (const viewport of [
  { width: 390, height: 844 },
  { width: 812, height: 375 },
]) {
  test(`Mobile document scrolling and overlays preserve position at ${viewport.width}x${viewport.height}`, async ({
    page,
  }) => {
    await mockShell(page, true);
    await page.setViewportSize(viewport);
    await page.goto(`/p/${portfolioId}/overview`);
    const tree = page.getByRole("list", { name: "Holdings tree at the latest valuation point" });
    await expect(tree.getByRole("button", { name: /^(Expand|Collapse) / })).toHaveCount(41);
    await expect(tree.getByRole("button", { name: /^Add manual balance for / })).toHaveCount(40);
    await page.mouse.move(viewport.width - 20, viewport.height - 40);
    await page.mouse.wheel(0, 600);
    await expect.poll(() => page.evaluate(() => window.scrollY)).toBeGreaterThan(100);
    expect(await page.locator("main").evaluate((element) => element.scrollTop)).toBe(0);
    const header = page.locator("header").first();
    await expect
      .poll(() => header.evaluate((element) => element.getBoundingClientRect().top))
      .toBe(0);
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
    ).toBe(true);

    const position = await page.evaluate(() => window.scrollY);
    const treeTop = await tree.evaluate((element) => element.getBoundingClientRect().top);
    const open = page.getByRole("button", { name: "Open navigation" });
    await open.click();
    await expect(page.getByRole("link", { name: "Overview", exact: true })).toBeFocused();
    await expect
      .poll(() => tree.evaluate((element) => element.getBoundingClientRect().top))
      .toBe(treeTop);
    await expect
      .poll(() => header.evaluate((element) => element.getBoundingClientRect().top))
      .toBe(0);
    await page.mouse.move(viewport.width - 20, viewport.height - 40);
    await page.mouse.wheel(0, 500);
    await expect
      .poll(() => tree.evaluate((element) => element.getBoundingClientRect().top))
      .toBe(treeTop);
    if (viewport.height < 480) {
      const sidebar = page.locator("aside");
      await sidebar.evaluate((element) => {
        element.scrollTop = element.scrollHeight;
      });
      await expect.poll(() => sidebar.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);
    }
    await page.keyboard.press("Escape");
    await expect(open).toBeFocused();
    await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(position);

    await page.getByTitle("Switch portfolio").click();
    const menu = page.getByRole("menu", { name: "Portfolios" });
    await expect(menu).toBeVisible();
    await menu.evaluate((element) => {
      element.scrollTop = 80;
    });
    await expect.poll(() => menu.evaluate((element) => element.scrollTop)).toBe(80);
    await expect(menu).toBeVisible();
    await page.getByRole("menuitem", { name: "New portfolio", exact: true }).click();
    const dialog = page.getByRole("dialog", { name: "New portfolio" });
    await expect(dialog).toBeVisible();
    await expect
      .poll(() => tree.evaluate((element) => element.getBoundingClientRect().top))
      .toBe(treeTop);
    await dialog.getByRole("textbox", { name: "Name", exact: true }).fill("Unsaved draft");
    await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
    await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(position);
    await expect(page.getByTitle("Switch portfolio")).toBeFocused();

    await page.getByTitle("Switch portfolio").click();
    await expect(menu).toBeVisible();
    await page.evaluate(() => window.scrollBy(0, 100));
    await expect(menu).toBeHidden();

    // Long identity names reflow in the tree instead of creating a wide outer table.
    expect(await tree.evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
    ).toBe(true);

    await open.click();
    await page.setViewportSize({ width: 1024, height: 768 });
    await expect(open).toBeHidden();
    await expect.poll(() => page.evaluate(() => document.body.style.position)).toBe("");
    await page.mouse.move(900, 600);
    await page.mouse.wheel(0, 400);
    await expect
      .poll(() => page.locator("main").evaluate((element) => element.scrollTop))
      .toBeGreaterThan(0);
    expect(await page.evaluate(() => window.scrollY)).toBe(0);
  });
}

test("Short mobile pages fill the viewport and login remains scrollable in landscape", async ({
  page,
}) => {
  await mockShell(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`/p/${portfolioId}/overview`);
  await expect(page.getByRole("heading", { name: "Overview" })).toBeVisible();
  expect(
    await page.locator("main").evaluate((element) => element.getBoundingClientRect().bottom),
  ).toBe(844);
  expect(
    await page.evaluate(() => getComputedStyle(document.documentElement).overscrollBehaviorY),
  ).toBe("auto");
  await page.unroute("**/api/v1/**");
  await page.route("**/api/v1/auth/me", (route) => route.fulfill({ status: 401, json: {} }));
  await page.route("**/api/v1/auth/csrf", (route) => route.fulfill({ json: { token: "test" } }));
  await page.goto("/login");
  await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();
  const form = page.locator("form");
  const bounds = await form.boundingBox();
  expect(bounds).not.toBeNull();
  expect(Math.abs(bounds!.y + bounds!.height / 2 - 422)).toBeLessThan(2);
  await page.setViewportSize({ width: 812, height: 375 });
  await page.getByRole("textbox", { name: "Username" }).fill("browser-check");
  await page.getByRole("button", { name: "Sign in", exact: true }).scrollIntoViewIfNeeded();
  await expect(page.getByRole("button", { name: "Sign in", exact: true })).toBeInViewport();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(
    true,
  );
});
