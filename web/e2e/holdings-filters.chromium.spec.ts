import { expect, test, type Page } from "@playwright/test";
import type { BalanceRow, Issue } from "../src/api/generated/types.gen";

const portfolioId = "00000000-0000-7000-8000-000000000001";
const bn = "00000000-0000-7000-8000-000000000002";
const ok = "00000000-0000-7000-8000-000000000003";
const archived = "00000000-0000-7000-8000-000000000004";

async function fixture(page: Page) {
  const state = {
    requests: [] as URL[],
    mutations: [] as string[],
    failCandidates: false,
    failRows: false,
    noPoint: false,
    delaySearch: false,
    balances: [] as BalanceRow[],
    balanceIssues: [] as Issue[],
  };
  await page.route("**/api/v1/**", async (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname;
    if (route.request().method() !== "GET") state.mutations.push(path);
    if (path.endsWith("/auth/me"))
      return route.fulfill({
        json: { user: { id: "user", username: "filters", status: "active" }, sessionCount: 1 },
      });
    if (path === "/api/v1/portfolios")
      return route.fulfill({
        json: {
          items: [
            {
              id: portfolioId,
              name: "Filters",
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
        json: {
          items: [
            { id: "binance", name: "Binance", accountStructures: [] },
            { id: "okx", name: "OKX", accountStructures: [] },
            { id: "kraken", name: "Kraken", accountStructures: [] },
          ],
        },
      });
    if (path.endsWith("/identities")) {
      if (state.failCandidates)
        return route.fulfill({ status: 503, json: { detail: "Candidates unavailable" } });
      const identity = (id: string, platform: string, displayName: string, status = "enabled") => ({
        id,
        platform,
        displayName,
        status,
        accounts: [],
        connection: { status: "disabled" },
      });
      return route.fulfill({
        json: {
          items: [
            identity(bn, "binance", "bn-main"),
            ...Array.from({ length: 24 }, (_, i) =>
              identity(`sub-${i}`, "binance", `bn-sub-${String(i).padStart(2, "0")}`),
            ),
            identity(ok, "okx", "ok-main"),
            identity(archived, "kraken", "old", "archived"),
          ],
        },
      });
    }
    if (/\/(balances|positions|dual-positions)$/.test(path)) {
      state.requests.push(url);
      if (state.delaySearch && url.searchParams.get("symbol"))
        await new Promise((resolve) => setTimeout(resolve, 500));
      if (state.failRows)
        return route.fulfill({ status: 503, json: { detail: "Rows unavailable" } });
      return route.fulfill({
        json: {
          context: {
            portfolioId,
            pointId: state.noPoint ? null : portfolioId,
            evaluatedAt: "2026-10-03T00:00:00Z",
            timezone: "Asia/Shanghai",
          },
          items: path.endsWith("/balances") ? state.balances : [],
          page: { offset: Number(url.searchParams.get("offset") ?? 0), limit: 50, total: 0 },
          totalUsd: "0",
          totalNotionalUsd: "741.37",
          completeness: "complete",
          issues: path.endsWith("/balances") ? state.balanceIssues : [],
        },
      });
    }
    return route.fulfill({ json: {} });
  });
  return state;
}

test("reload preserves mobile holdings filters and pagination without collection", async ({
  page,
}) => {
  const state = await fixture(page);
  await page.setViewportSize({ width: 390, height: 844 });
  const path = `/p/${portfolioId}/holdings/balances?platform=binance&identityId=${bn}&symbol=btc&offset=50`;
  await page.goto(path);
  await expect(page.getByRole("button", { name: "Filter Asset: btc" })).toBeVisible();
  await expect.poll(() => state.requests.length).toBeGreaterThan(0);
  state.requests.length = 0;
  await page.reload();
  await expect(page).toHaveURL(path);
  await expect
    .poll(() =>
      state.requests.some(
        (url) =>
          url.searchParams.get("platform") === "binance" &&
          url.searchParams.get("identityId") === bn &&
          url.searchParams.get("symbol") === "btc" &&
          url.searchParams.get("offset") === "50",
      ),
    )
    .toBe(true);
  expect(state.mutations).toHaveLength(0);
});

test("balance Asset search keeps symbol queries and Observed discloses the correct stale reason", async ({
  page,
}) => {
  const state = await fixture(page);
  state.balances = [
    {
      accountId: "funding",
      accountName: "Funding",
      accountType: "funding",
      identityId: bn,
      identityName: "bn-main",
      platform: "binance",
      listingId: "listing",
      symbol: "XBT",
      assetSymbol: "BTC",
      equity: "1",
      priceUsd: "50000",
      equityUsd: "50000",
      noPrice: false,
      freshness: "stale",
      reportObservedAt: "2026-10-02T00:00:00Z",
    },
  ];
  state.balanceIssues = [
    { code: "stale", accountId: "other", detail: "A different account failed." },
    { code: "stale", accountId: "funding", detail: "Funding rate limited; previous report used." },
  ];
  await page.goto(`/p/${portfolioId}/holdings/balances`);
  await expect(page.getByRole("columnheader")).toHaveText([
    /Platform.*All/,
    /Identity.*All/,
    "Account",
    /Asset.*All/,
    "Equity",
    "Price (USD)",
    "Equity (USD)",
    "Liability",
    "Unrealized PnL",
    "Observed",
  ]);
  await expect(page.getByRole("cell", { name: "XBT as BTC" })).toBeVisible();
  await expect(page.getByRole("cell", { name: "2026-10-02 08:00 Stale" })).toBeVisible();
  const stale = page.getByRole("button", { name: "Stale report for XBT" });
  await stale.focus();
  await expect(page.getByRole("tooltip")).toHaveText("Funding rate limited; previous report used.");
  await page.keyboard.press("Escape");
  await expect(page.getByRole("tooltip")).toBeHidden();
  await page.getByRole("button", { name: "Filter Asset: All" }).click();
  await page.getByRole("textbox", { name: "Search Asset" }).fill("btc");
  await expect
    .poll(() => state.requests.some((url) => url.searchParams.get("symbol") === "btc"))
    .toBe(true);
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "Clear Asset filter" }).click();
  await expect(page).not.toHaveURL(/symbol=/);

  state.balanceIssues = [];
  await page.reload();
  await stale.click();
  await expect(page.getByRole("tooltip")).toHaveText(
    "An earlier successful Account Report is carried forward.",
  );
  expect(state.mutations).toHaveLength(0);
});

test("column filters link configured candidates and clear independently", async ({ page }) => {
  await fixture(page);
  await page.goto(`/p/${portfolioId}/holdings/futures`);
  const platform = page.getByRole("button", { name: "Filter Platform: All", exact: true });
  const identity = page.getByRole("button", { name: "Filter Identity: All", exact: true });
  await identity.click();
  await page.getByRole("option", { name: "bn-main", exact: true }).click();
  await expect(platform).toBeVisible();
  await expect(page).toHaveURL(new RegExp(`identityId=${bn}`));
  await platform.click();
  await expect(page.getByRole("option")).toHaveText(["All", "Binance"]);
  await page.getByRole("option", { name: "Binance", exact: true }).click();
  await page.getByRole("button", { name: "Clear Platform filter" }).click();
  await expect(page.getByRole("button", { name: "Filter Identity: bn-main" })).toBeVisible();
  await page.getByRole("button", { name: "Clear Identity filter" }).click();
  await platform.focus();
  await page.keyboard.press("ArrowDown");
  await expect(page.getByRole("option")).toHaveText(["All", "Binance", "OKX"]);
  await page.keyboard.press("End");
  await page.keyboard.press("Enter");
  await identity.click();
  await expect(page.getByRole("option")).toHaveText(["All", "ok-main"]);
  await page.keyboard.press("Escape");
  await expect(identity).toBeFocused();
  await page.getByRole("button", { name: "Clear filters", exact: true }).click();
  await expect(page).toHaveURL(`/p/${portfolioId}/holdings/futures`);
  await expect(page.locator("select")).toHaveCount(0);
  await expect(page.getByRole("button", { name: /^Filter Account/ })).toHaveCount(0);
});

test("search debounces, respects composition, and keeps its input through loading", async ({
  page,
}) => {
  const state = await fixture(page);
  state.delaySearch = true;
  await page.goto(`/p/${portfolioId}/holdings/futures?offset=50`);
  await page.getByRole("button", { name: "Filter Symbol: All" }).click();
  const input = page.getByRole("textbox", { name: "Search Symbol" });
  await input.fill("btc");
  await expect(input).toBeFocused();
  await page.waitForTimeout(100);
  expect(state.requests.some((url) => url.searchParams.has("symbol"))).toBe(false);
  await expect
    .poll(() => state.requests.some((url) => url.searchParams.get("symbol") === "btc"))
    .toBe(true);
  await expect(page.getByRole("cell", { name: "Loading positions." })).toBeVisible();
  await expect(input).toBeFocused();
  await expect(page).not.toHaveURL(/offset=/);
  await input.dispatchEvent("compositionstart");
  await input.fill("牛来");
  await page.waitForTimeout(350);
  expect(state.requests.some((url) => url.searchParams.get("symbol") === "牛来")).toBe(false);
  await input.dispatchEvent("compositionend", { data: "牛来" });
  await expect
    .poll(() => state.requests.some((url) => url.searchParams.get("symbol") === "牛来"))
    .toBe(true);
  await input.press("Home");
  await expect(input).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("button", { name: "Filter Symbol: 牛来" })).toBeFocused();
  await page.getByRole("button", { name: "Clear Symbol filter" }).click();
  await expect(page).not.toHaveURL(/symbol=/);
});

test("pending search stays with its own top-level tab and reset cancels it", async ({ page }) => {
  await fixture(page);
  await page.goto(`/p/${portfolioId}/holdings/futures`);
  const tabs = page.getByRole("navigation", { name: "Holdings views" });
  await expect(tabs.getByRole("link")).toHaveText(["Balances", "Futures", "Dual Investment"]);
  await expect(page.getByRole("tablist")).toHaveCount(0);
  await page.getByRole("button", { name: "Filter Symbol: All" }).click();
  await page.getByRole("textbox", { name: "Search Symbol" }).fill("eth");
  await tabs.getByRole("link", { name: "Dual Investment", exact: true }).click();
  await expect(page).toHaveURL(`/p/${portfolioId}/holdings/dual-investment`);
  await expect(page.getByRole("button", { name: "Filter Pair: All" })).toBeVisible();
  await page.getByRole("button", { name: "Filter Pair: All" }).click();
  await page.getByRole("textbox", { name: "Search Pair" }).fill("btc");
  await page.getByRole("button", { name: "Clear filters", exact: true }).click();
  await page.waitForTimeout(350);
  await expect(page).not.toHaveURL(/symbol=/);
  await tabs.getByRole("link", { name: "Futures", exact: true }).click();
  await expect(page.getByRole("button", { name: "Filter Symbol: eth" })).toBeVisible();
  await expect(page).toHaveURL(/futures\?symbol=eth$/);
  await tabs.getByRole("link", { name: "Balances", exact: true }).click();
  await expect(page.getByRole("button", { name: "Filter Asset: All" })).toBeVisible();
});

test("each tab restores its filters and page; history and explicit URLs win", async ({ page }) => {
  await fixture(page);
  await page.goto(
    `/p/${portfolioId}/holdings/futures?platform=binance&identityId=${bn}&symbol=eth&offset=50`,
  );
  const tabs = page.getByRole("navigation", { name: "Holdings views" });
  await expect(page.getByRole("button", { name: "Filter Symbol: eth" })).toBeVisible();
  await tabs.getByRole("link", { name: "Balances", exact: true }).click();
  await expect(page.getByRole("button", { name: "Filter Platform: All" })).toBeVisible();
  await page.getByRole("button", { name: "Filter Asset: All" }).click();
  await page.getByRole("textbox", { name: "Search Asset" }).fill("btc");
  await tabs.getByRole("link", { name: "Futures", exact: true }).click();
  await expect(page).toHaveURL(
    new RegExp(`futures\\?platform=binance&identityId=${bn}&symbol=eth&offset=50$`),
  );
  await page.goBack();
  await expect(page).toHaveURL(/balances\?symbol=btc$/);
  await page.goForward();
  await expect(page).toHaveURL(/offset=50$/);
  await tabs.getByRole("link", { name: "Balances", exact: true }).click();
  await expect(page).toHaveURL(/balances\?symbol=btc$/);
  await page.goto(`/p/${portfolioId}/holdings`);
  await expect(page).toHaveURL(`/p/${portfolioId}/holdings/balances`);
});

test("old and invalid URLs normalize after candidates load; Balances has Identity and no groups", async ({
  page,
}) => {
  const state = await fixture(page);
  await page.goto(
    `/p/${portfolioId}/holdings/balances?accountId=old&groupBy=asset&platform=okx&identityId=${bn}&offset=50`,
  );
  await expect(page).toHaveURL(new RegExp(`/holdings/balances\\?identityId=${bn}$`));
  await expect(page.getByRole("columnheader").nth(1)).toContainText("Identity");
  await expect(page.getByRole("heading", { name: "Groups", exact: true })).toHaveCount(0);
  expect(
    state.requests.every(
      (url) =>
        !url.searchParams.has("accountId") &&
        !url.searchParams.has("groupBy") &&
        !url.searchParams.has("platform"),
    ),
  ).toBe(true);
  await page.goto(`/p/${portfolioId}/holdings/futures?identityId=malformed`);
  await expect(page).toHaveURL(`/p/${portfolioId}/holdings/futures`);
  expect(state.requests.some((url) => url.searchParams.get("identityId") === "malformed")).toBe(
    false,
  );
});

test("headers survive errors/no point and popovers fit narrow viewports", async ({ page }) => {
  const state = await fixture(page);
  state.noPoint = true;
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`/p/${portfolioId}/holdings/futures`);
  const identity = page.getByRole("button", { name: "Filter Identity: All" });
  await identity.click();
  const choices = page.getByRole("listbox", { name: "Choose Identity" });
  await expect(choices).toBeVisible();
  const bounds = await choices.boundingBox();
  expect(bounds!.x).toBeGreaterThanOrEqual(8);
  expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(382);
  expect(bounds!.y + bounds!.height).toBeLessThanOrEqual(836);
  await choices.evaluate((element) => {
    element.scrollTop = 200;
  });
  await expect(choices).toBeVisible();
  await page
    .getByRole("table")
    .locator("..")
    .evaluate((element) => {
      element.scrollLeft += 100;
    });
  await expect(choices).toBeHidden();
  await expect
    .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth))
    .toBe(true);
  state.failRows = true;
  await page.getByRole("button", { name: "Filter Symbol: All" }).click();
  await page.getByRole("textbox", { name: "Search Symbol" }).fill("no-match");
  await expect(page.getByRole("alert")).toContainText("Rows unavailable");
  await expect(page.getByRole("textbox", { name: "Search Symbol" })).toBeFocused();
});

test("candidate failure keeps URL conditions until a successful retry", async ({ page }) => {
  const state = await fixture(page);
  state.failCandidates = true;
  await page.goto(`/p/${portfolioId}/holdings/futures?identityId=${bn}`);
  await expect(page.getByRole("alert")).toContainText("Candidates unavailable");
  await expect(page).toHaveURL(new RegExp(`identityId=${bn}`));
  expect(state.requests).toHaveLength(0);
  state.failCandidates = false;
  await page.getByRole("button", { name: "Retry", exact: true }).click();
  await expect(page.getByRole("button", { name: "Filter Identity: bn-main" })).toBeEnabled();
  await expect.poll(() => state.requests.length).toBeGreaterThan(0);
});
