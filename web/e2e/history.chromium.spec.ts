import { expect, test, type Page } from "@playwright/test";
import type { ValuationPointSummary, ValueChange } from "../src/api/generated/types.gen";

test.use({ timezoneId: "America/Los_Angeles" });

const portfolioId = "00000000-0000-7000-8000-000000000001";
const base = `/api/v1/portfolios/${portfolioId}`;
const historyPath = `/p/${portfolioId}/history`;
const initialEnd = "2026-10-04T04:16:44Z";
const point = (index: number): ValuationPointSummary => ({
  id: `point-${index}`,
  jobId: `job-${index}`,
  evaluatedAt: new Date(Date.parse(initialEnd) - (560 - index) * 900_000).toISOString(),
  totalUsd: String(100_000 + index),
  completeness: "complete",
  accountCount: 1,
  staleAccountCount: 0,
  incompleteAccountCount: 0,
  boundaryContributionUsd: "0",
  boundaryWithdrawalUsd: "0",
});

async function fixture(page: Page) {
  const state = {
    now: initialEnd,
    count: 561,
    requests: [] as URL[],
    failPage: false,
    failSeries: false,
  };
  const context = { portfolioId, timezone: "Asia/Shanghai" };
  const portfolio = {
    id: portfolioId,
    name: "History check",
    timezone: context.timezone,
    collectionIntervalSeconds: 900,
    undeletable: true,
    version: 1,
    createdAt: initialEnd,
    updatedAt: initialEnd,
  };
  await page.route("**/api/v1/**", async (route) => {
    const url = new URL(route.request().url());
    state.requests.push(url);
    const path = url.pathname;
    if (route.request().method() !== "GET") throw new Error(`Unexpected mutation ${path}`);
    const params = url.searchParams;
    const from = params.get("from") ?? "2026-09-27T04:16:44Z";
    const to = params.get("to") ?? state.now;
    if (path === "/api/v1/auth/me") {
      await route.fulfill({
        json: { user: { id: "user", username: "history", status: "active" }, sessionCount: 1 },
      });
    } else if (path === "/api/v1/portfolios") {
      await route.fulfill({ json: { items: [portfolio] } });
    } else if (path === base) {
      await route.fulfill({ json: portfolio });
    } else if (path === `${base}/history-window`) {
      const localFrom = params.get("fromLocal");
      const localTo = params.get("toLocal");
      if (localFrom && localTo) {
        const start = new Date(`${localFrom}+08:00`);
        const end = new Date(`${localTo}+08:00`);
        if (start >= end)
          await route.fulfill({
            status: 400,
            json: { detail: "The end must be after the start." },
          });
        else
          await route.fulfill({
            json: { context, from: start.toISOString(), to: end.toISOString() },
          });
      } else {
        const days =
          {
            "1D": 1,
            "3D": 3,
            "1W": 7,
            "2W": 14,
            "1M": 30,
            "3M": 90,
            "6M": 180,
            "1d": 1,
            "3d": 3,
            "1w": 7,
            "2w": 14,
            "1Month": 30,
            "1Year": 365,
          }[params.get("preset") ?? "3D"] ?? 3;
        await route.fulfill({
          json: {
            context,
            from: new Date(Date.parse(state.now) - days * 86_400_000).toISOString(),
            to: state.now,
          },
        });
      }
    } else if (path === `${base}/valuation-points/page`) {
      if (state.failPage) {
        await route.fulfill({ status: 500, json: { detail: "Page temporarily unavailable." } });
        return;
      }
      const offset = Number(params.get("offset") ?? 0);
      const limit = Number(params.get("limit") ?? 20);
      const items = Array.from({ length: state.count }, (_, index) => point(index))
        .reverse()
        .slice(offset, offset + limit);
      await route.fulfill({
        json: {
          context,
          from,
          to,
          items,
          page: { kind: "offset", offset, limit, total: state.count },
        },
      });
    } else if (path === `${base}/valuation-points`) {
      if (state.failSeries) {
        await route.fulfill({ status: 500, json: { detail: "Series temporarily unavailable." } });
        return;
      }
      await route.fulfill({
        json: {
          context,
          from,
          to,
          granularity: params.get("granularity"),
          items: state.count
            ? [point(0), ...(state.count > 1 ? [point(state.count - 1)] : [])]
            : [],
        },
      });
    } else if (path === `${base}/period-closes`) {
      const items = Array.from({ length: 42 }, (_, index) => ({
        periodKind: params.get("periodKind"),
        label: `period-${index}`,
        periodStart: new Date(Date.parse(initialEnd) - (42 - index) * 86_400_000).toISOString(),
        periodEnd: new Date(Date.parse(initialEnd) - (41 - index) * 86_400_000).toISOString(),
        point: point(index),
        completeness: "complete",
        reasons: [],
      }));
      await route.fulfill({ json: { context, periodKind: params.get("periodKind"), items } });
    } else if (path === `${base}/value-changes`) {
      const items: ValueChange[] = Array.from({ length: 42 }, (_, index) => ({
        periodKind: "day",
        label: `change-${index}`,
        from: point(index).evaluatedAt,
        to: point(index + 1).evaluatedAt,
        amountUsd: "1",
        contributionsUsd: "0",
        withdrawalsUsd: "0",
        boundaryContributionsUsd: "0",
        boundaryWithdrawalsUsd: "0",
        completeness: "complete",
        reasons: [],
        boundaryEvents: [],
        startPoint: point(index),
        endPoint: point(index + 1),
      }));
      await route.fulfill({ json: { context, periodKind: params.get("periodKind"), items } });
    } else if (path.startsWith(`${base}/valuation-points/`)) {
      await route.fulfill({
        json: {
          context,
          point: point(Number(path.split("point-")[1])),
          accounts: [],
          issues: [],
          boundaryEvents: [],
        },
      });
    } else if (path === "/api/v1/platforms") {
      await route.fulfill({ json: { items: [] } });
    } else {
      await route.fulfill({ status: 404, json: { detail: `Fixture missing ${path}` } });
    }
  });
  return state;
}

const rawTable = (page: Page) =>
  page.getByRole("table", { name: "Raw Valuation Points", exact: true });
const pager = (page: Page, label: string) =>
  page.getByLabel(`${label} pagination`, { exact: true });

test("561 raw points are paged independently of the curve and closes", async ({ page }) => {
  const state = await fixture(page);
  await page.addInitScript(() =>
    Object.defineProperty(window.crypto, "randomUUID", { value: undefined }),
  );
  await page.goto(historyPath);
  await expect(page.getByRole("button", { name: "3D", exact: true })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await expect(page.getByLabel("From", { exact: true })).toHaveValue("2026-10-01T12:16:44");
  await expect(rawTable(page).locator("tbody > tr")).toHaveCount(20);
  await expect(rawTable(page).locator("tbody > tr").first()).toContainText("100,560");
  await expect(pager(page, "Raw Valuation Points")).toContainText("1–20 of 561");
  const headings = await page.getByRole("heading", { level: 2 }).allTextContents();
  expect(headings).toEqual(["Portfolio value", "Period closes", "Raw Valuation Points"]);
  const chartRequests = state.requests.filter(
    (url) => url.pathname === `${base}/valuation-points`,
  ).length;
  const resolved = state.requests.find((url) => url.pathname === `${base}/valuation-points`)!;
  const closes = state.requests.find((url) => url.pathname === `${base}/period-closes`)!;
  expect(closes.searchParams.get("from")).toBe(resolved.searchParams.get("from"));
  expect(resolved.searchParams.get("granularity")).toBe("raw");
  await pager(page, "Raw Valuation Points")
    .getByRole("button", { name: "Next", exact: true })
    .click();
  await expect(pager(page, "Raw Valuation Points")).toContainText("21–40 of 561");
  await rawTable(page)
    .getByRole("button", { name: /Expand the Valuation Point/ })
    .first()
    .click();
  await expect(page).toHaveURL(/pointsOffset=20/);
  await expect(rawTable(page)).toContainText("No accounts were inside the boundary at this point.");
  await pager(page, "Period closes").getByRole("button", { name: "Next", exact: true }).click();
  await expect(pager(page, "Period closes")).toContainText("21–40 of 42");
  await expect(pager(page, "Raw Valuation Points")).toContainText("21–40 of 561");
  expect(state.requests.filter((url) => url.pathname === `${base}/valuation-points`)).toHaveLength(
    chartRequests,
  );
  await page.getByRole("combobox", { name: "Period", exact: true }).selectOption("week");
  await expect(pager(page, "Period closes")).toContainText("1–20 of 42");
  await expect(pager(page, "Raw Valuation Points")).toContainText("21–40 of 561");
  await page.getByLabel("Raw Valuation Points rows per page").selectOption("10");
  await expect(rawTable(page).locator("tbody > tr")).toHaveCount(10);
  await expect(pager(page, "Raw Valuation Points")).toContainText("1–10 of 561");
  await page.getByRole("button", { name: "3M", exact: true }).click();
  await expect(page.getByRole("combobox", { name: "Period", exact: true })).toHaveValue("month");
  await expect(page.getByText(/day granularity/)).toBeVisible();
});

test("date drafts apply once; custom endpoints survive reload, presets reanchor", async ({
  page,
}) => {
  const state = await fixture(page);
  await page.goto(historyPath);
  await expect(rawTable(page).locator("tbody > tr")).toHaveCount(20);
  const count = state.requests.length;
  await page.getByLabel("From", { exact: true }).fill("2026-09-01T12:00");
  await page.getByLabel("To", { exact: true }).fill("2026-10-01T12:00");
  expect(state.requests).toHaveLength(count);
  await page.getByRole("button", { name: "Apply", exact: true }).click();
  await expect(page).toHaveURL(/from=2026-09-01T04/);
  await expect(page.getByRole("button", { name: "3D", exact: true })).toHaveAttribute(
    "aria-pressed",
    "false",
  );
  await page.reload();
  await expect(page.getByLabel("From", { exact: true })).toHaveValue(/2026-09-01T12:00(:00)?/);
  await page.getByLabel("From", { exact: true }).fill("2026-10-02T12:00");
  await page.getByRole("button", { name: "Apply", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("The end must be after the start.");
  await expect(page).toHaveURL(/from=2026-09-01T04/);
  await page.getByRole("button", { name: "3D", exact: true }).click();
  await expect(page.getByText(/raw granularity/)).toBeVisible();
  state.now = "2026-10-05T04:16:44Z";
  await page.reload();
  await expect(page.getByLabel("To", { exact: true })).toHaveValue("2026-10-05T12:16:44");
  await expect(page.getByRole("button", { name: "3D", exact: true })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
});

test("Value Changes pages its table, keeps its chart, and leaves custom comparison independent", async ({
  page,
}) => {
  const state = await fixture(page);
  await page.goto(`${historyPath}/value-changes`);
  const table = page.getByRole("table", { name: "Value change per day values" });
  await expect(table.locator("tbody > tr")).toHaveCount(20);
  await expect(table.locator("tbody > tr").first()).toContainText("change-41");
  const reads = state.requests.filter((url) => url.pathname === `${base}/value-changes`).length;
  await pager(page, "Value Changes").getByRole("button", { name: "Next", exact: true }).click();
  await expect(pager(page, "Value Changes")).toContainText("21–40 of 42");
  expect(state.requests.filter((url) => url.pathname === `${base}/value-changes`)).toHaveLength(
    reads,
  );
  await page.getByLabel("From", { exact: true }).last().fill("2026-08-01T12:00");
  await page.getByRole("button", { name: "1M", exact: true }).click();
  await expect(page.getByRole("combobox", { name: "Period", exact: true })).toHaveValue("week");
  await expect(page.getByLabel("From", { exact: true }).last()).toHaveValue("2026-08-01T12:00");
  await expect(pager(page, "Value Changes")).toContainText("1–20 of 42");
});

test("empty, single-point and last-page URL states stay usable", async ({ page }) => {
  const state = await fixture(page);
  state.count = 0;
  await page.goto(historyPath);
  await expect(
    page
      .getByText(
        "No Valuation Point in this window. Enable a connection and run a Collection Cycle.",
        { exact: true },
      )
      .first(),
  ).toBeVisible();
  await expect(pager(page, "Raw Valuation Points")).toContainText("0 rows");
  state.count = 1;
  await page.reload();
  await expect(
    page.getByText("Only one Valuation Point in this window; the chart needs two."),
  ).toBeVisible();
  state.count = 561;
  await page.goto(`${historyPath}?pointsOffset=560`);
  await expect(rawTable(page).locator("tbody > tr")).toHaveCount(1);
  await expect(
    pager(page, "Raw Valuation Points").getByRole("button", { name: "Next", exact: true }),
  ).toBeDisabled();
  await page.goto(`${historyPath}?pointsOffset=900`);
  await expect(pager(page, "Raw Valuation Points")).toContainText("561–561 of 561");
});

test("a failed raw page retries the same applied window", async ({ page }) => {
  const state = await fixture(page);
  await page.goto(historyPath);
  await expect(rawTable(page).locator("tbody > tr")).toHaveCount(20);
  const resolved = state.requests.find((url) => url.pathname === `${base}/valuation-points`)!;
  state.failPage = true;
  await pager(page, "Raw Valuation Points")
    .getByRole("button", { name: "Next", exact: true })
    .click();
  await expect(page.getByRole("alert")).toContainText("Page temporarily unavailable.", {
    timeout: 15_000,
  });
  state.failPage = false;
  state.now = "2026-10-05T04:16:44Z";
  await page.getByRole("alert").getByRole("button", { name: "Retry", exact: true }).click();
  await expect(pager(page, "Raw Valuation Points")).toContainText("21–40 of 561");
  const retried = state.requests
    .filter((url) => url.pathname === `${base}/valuation-points/page`)
    .at(-1)!;
  expect(retried.searchParams.get("from")).toBe(resolved.searchParams.get("from"));
  expect(retried.searchParams.get("to")).toBe(resolved.searchParams.get("to"));
});

for (const theme of ["light", "dark"]) {
  test(`History controls and pagers stay contained on a phone in ${theme} mode`, async ({
    page,
  }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await page.addInitScript((selected) => localStorage.setItem("plutus-theme", selected), theme);
    await fixture(page);
    await page.goto(historyPath);
    await expect(rawTable(page).locator("tbody > tr")).toHaveCount(20);
    await page.getByRole("button", { name: "6M", exact: true }).click();
    await expect(page.getByText(/day granularity/)).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(
      375,
    );
    await expect(
      pager(page, "Raw Valuation Points").getByRole("button", { name: "Next", exact: true }),
    ).toBeEnabled();
  });
}
