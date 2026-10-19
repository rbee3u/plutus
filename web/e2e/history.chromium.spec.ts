import { mockApi } from "./support/api";
import { sessionFixture } from "./support/session";
import { expect, test, type Page } from "@playwright/test";
import type { ValuationPointSummary, ValueChange } from "../src/api/generated/types.gen";

test.use({ timezoneId: "America/Los_Angeles" });

const portfolioId = "00000000-0000-7000-8000-000000000001";
const base = `/api/v1/portfolios/${portfolioId}`;
const historyPath = `/p/${portfolioId}/history`;
const initialEnd = "2026-10-04T04:16:44Z";
const point = (index: number): ValuationPointSummary => ({
  cycleId: `point-${index}`,

  calculationVersion: 1,
  detailState: "full" as const,
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
    summary: {} as Partial<ValueChange>,
    failSummary: false,
  };
  const context = { portfolioId, timezone: "Asia/Shanghai" };
  const session = sessionFixture({
    user: { username: "history" },
    portfolio: {
      id: portfolioId,
      name: "History check",
      timezone: context.timezone,
      collectionIntervalSeconds: 900,
      createdAt: initialEnd,
      updatedAt: initialEnd,
    },
  });
  await mockApi(page, session, async (route) => {
    const url = new URL(route.request().url());
    state.requests.push(url);
    const path = url.pathname;
    if (route.request().method() !== "GET") throw new Error(`Unexpected mutation ${path}`);
    const params = url.searchParams;
    const from = params.get("from") ?? "2026-09-27T04:16:44Z";
    const to = params.get("to") ?? state.now;
    if (path === `${base}/history-window`) {
      const localFrom = params.get("fromLocal");
      const localTo = params.get("toLocal");
      if (localFrom && localTo) {
        const start = new Date(`${localFrom.trim().replace(" ", "T")}+08:00`);
        const end = new Date(`${localTo.trim().replace(" ", "T")}+08:00`);
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
          }[params.get("preset") ?? "1W"] ?? 7;
        await route.fulfill({
          json: {
            context,
            from: new Date(Date.parse(state.now) - days * 86_400_000).toISOString(),
            to: state.now,
          },
        });
      }
    } else if (path === `${base}/valuation-points`) {
      if (state.failPage) {
        await route.fulfill({ status: 500, json: { detail: "Page temporarily unavailable." } });
        return;
      }
      const offset = Number(params.get("offset") ?? 0);
      const limit = Number(params.get("limit") ?? 20);
      const items = Array.from({ length: state.count }, (_, index) => ({
        ...point(index),
      }))
        .reverse()
        .slice(offset, offset + limit);
      await route.fulfill({
        json: {
          context,
          from,
          to,
          items,
          precedingPoint:
            offset + limit < state.count ? point(state.count - offset - limit - 1) : null,
          page: { kind: "offset", offset, limit, total: state.count },
        },
      });
    } else if (path === `${base}/valuation-points/series`) {
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
        issues: [],
      }));
      await route.fulfill({ json: { context, periodKind: params.get("periodKind"), items } });
    } else if (path === `${base}/value-changes`) {
      const items: ValueChange[] = Array.from({ length: 42 }, (_, index) => ({
        periodKind: "day",
        label: `change-${index}`,
        from: point(index).evaluatedAt,
        to: point(index + 1).evaluatedAt,
        amountUsd: "1",
        comparable: true,
        flowsCompleteness: "complete",
        contributionsUsd: "0",
        withdrawalsUsd: "0",
        boundaryContributionsUsd: "0",
        boundaryWithdrawalsUsd: "0",
        completeness: "complete",
        issues: [],
        boundaryEvents: [],
        startPoint: point(index),
        endPoint: point(index + 1),
      }));
      await route.fulfill({ json: { context, periodKind: params.get("periodKind"), items } });
    } else if (path === `${base}/value-changes/range`) {
      if (state.failSummary) {
        await route.fulfill({ status: 500, json: { detail: "Summary temporarily unavailable." } });
        return;
      }
      const summary: ValueChange = {
        periodKind: "custom",
        label: "custom",
        from,
        to,
        startPoint: {
          ...point(0),
          evaluatedAt: new Date(Date.parse(from) - 60_000).toISOString(),
        },
        endPoint: {
          ...point(560),
          evaluatedAt: new Date(Date.parse(to) - 60_000).toISOString(),
        },
        amountUsd: "500",
        comparable: true,
        flowsCompleteness: "complete",
        contributionsUsd: "100",
        withdrawalsUsd: "40",
        boundaryContributionsUsd: "0",
        boundaryWithdrawalsUsd: "0",
        completeness: "complete",
        issues: [],
        boundaryEvents: [],
        ...state.summary,
      };
      await route.fulfill({ json: summary });
    } else if (path.startsWith(`${base}/valuation-points/`)) {
      await route.fulfill({
        json: {
          context,
          point: {
            ...point(Number(path.split("point-")[1])),
          },
          accounts: [],
          issues: [],
          boundaryEvents: [],
        },
      });
    } else if (path === "/api/v1/platforms") {
      await route.fulfill({ json: { items: [] } });
    } else {
      await route.fallback();
    }
  });
  return state;
}

const rawTable = (page: Page) =>
  page.getByRole("table", { name: "Raw valuation points", exact: true });
const pager = (page: Page, label: string) =>
  page.getByLabel(`${label} pagination`, { exact: true });

test("561 raw points are paged independently of the curve and closes", async ({ page }) => {
  const state = await fixture(page);
  await page.addInitScript(() =>
    Object.defineProperty(window.crypto, "randomUUID", { value: undefined }),
  );
  await page.goto(historyPath);
  await expect(page.getByRole("button", { name: "1W", exact: true })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await expect(page.getByLabel("From", { exact: true })).toHaveValue("2026-09-27 12:16");
  await expect(rawTable(page).locator("tbody > tr")).toHaveCount(20);
  await expect(rawTable(page).locator("tbody > tr").first()).toContainText("100,560");
  await expect(pager(page, "Raw valuation points")).toContainText("1–20 of 561");
  const headings = await page.getByRole("heading", { level: 2 }).allTextContents();
  expect(headings).toEqual(["Portfolio value", "Period closes", "Raw valuation points"]);
  const chartRequests = state.requests.filter(
    (url) => url.pathname === `${base}/valuation-points/series`,
  ).length;
  const resolved = state.requests.find(
    (url) => url.pathname === `${base}/valuation-points/series`,
  )!;
  const closes = state.requests.find((url) => url.pathname === `${base}/period-closes`)!;
  expect(closes.searchParams.get("from")).toBe(resolved.searchParams.get("from"));
  expect(resolved.searchParams.get("granularity")).toBe("hour");
  await pager(page, "Raw valuation points")
    .getByRole("button", { name: "Next", exact: true })
    .click();
  await expect(pager(page, "Raw valuation points")).toContainText("21–40 of 561");
  await rawTable(page)
    .getByRole("button", { name: /Expand the valuation point/ })
    .first()
    .click();
  await expect(page).toHaveURL(/pointsOffset=20/);
  await expect(rawTable(page)).toContainText("No accounts were inside the boundary at this point.");
  await pager(page, "Period closes").getByRole("button", { name: "Next", exact: true }).click();
  await expect(pager(page, "Period closes")).toContainText("21–40 of 42");
  await expect(pager(page, "Raw valuation points")).toContainText("21–40 of 561");
  expect(
    state.requests.filter((url) => url.pathname === `${base}/valuation-points/series`),
  ).toHaveLength(chartRequests);
  await page.getByRole("combobox", { name: "Period", exact: true }).selectOption("week");
  await expect(pager(page, "Period closes")).toContainText("1–20 of 42");
  await expect(pager(page, "Raw valuation points")).toContainText("21–40 of 561");
  await page.getByLabel("Raw valuation points rows per page").selectOption("10");
  await expect(rawTable(page).locator("tbody > tr")).toHaveCount(10);
  await expect(pager(page, "Raw valuation points")).toContainText("1–10 of 561");
  await page.getByRole("button", { name: "3M", exact: true }).click();
  await expect(page.getByRole("combobox", { name: "Period", exact: true })).toHaveValue("month");
  await expect(page.getByText(/ · Daily$/)).toBeVisible();
});

test("Date drafts apply once; custom endpoints survive reload, presets reanchor", async ({
  page,
}) => {
  const state = await fixture(page);
  await page.goto(historyPath);
  await expect(rawTable(page).locator("tbody > tr")).toHaveCount(20);
  const count = state.requests.length;
  await page.getByLabel("From", { exact: true }).fill("");
  await page.getByLabel("From", { exact: true }).pressSequentially("2026-09-01 12:00");
  await page.getByLabel("To", { exact: true }).fill("2026-10-01 12:00:42.123456");
  await expect(page.getByLabel("To", { exact: true })).toHaveValue("2026-10-01 12:00");
  expect(state.requests).toHaveLength(count);
  await page.getByRole("button", { name: "Apply", exact: true }).click();
  await expect(page).toHaveURL(/from=2026-09-01T04/);
  const applied = state.requests.find((url) => url.searchParams.has("fromLocal"))!;
  expect(applied.searchParams.get("fromLocal")).toBe("2026-09-01 12:00");
  expect(applied.searchParams.get("toLocal")).toBe("2026-10-01 12:00");
  await expect(page.getByRole("button", { name: "1W", exact: true })).toHaveAttribute(
    "aria-pressed",
    "false",
  );
  await page.reload();
  await expect(page.getByLabel("From", { exact: true })).toHaveValue("2026-09-01 12:00");
  await page.getByLabel("From", { exact: true }).fill("2026-10-02 12:00");
  await page.getByRole("button", { name: "Apply", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("The end must be after the start.");
  await expect(page).toHaveURL(/from=2026-09-01T04/);
  await page.getByRole("button", { name: "1W", exact: true }).click();
  await expect(page.getByText(/ · Hourly$/)).toBeVisible();
  state.now = "2026-10-05T04:16:44Z";
  await page.reload();
  await expect(page.getByLabel("To", { exact: true })).toHaveValue("2026-10-05 12:16");
  await expect(page.getByRole("button", { name: "1W", exact: true })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
});

test("Value changes shares the applied range with its summary, independently of period and pagination", async ({
  page,
}) => {
  const state = await fixture(page);
  await page.goto(`${historyPath}/value-changes`);
  const table = page.getByRole("table", { name: "Value change per day values" });
  const summary = page.getByRole("region", { name: "Selected range summary" });
  await expect(summary).toContainText("+$500");
  await expect(summary).toContainText("at 2026-09-27 12:15");
  await expect(page.getByLabel("From", { exact: true })).toHaveCount(1);
  await expect(page.getByLabel("To", { exact: true })).toHaveCount(1);
  await expect(page.getByRole("button", { name: "Compute", exact: true })).toHaveCount(0);
  expect(await page.getByRole("heading", { level: 2 }).allTextContents()).toEqual([
    "Range summary",
    "Value change per day",
  ]);
  await expect(table.locator("tbody > tr")).toHaveCount(20);
  await expect(table.locator("tbody > tr").first()).toContainText("change-41");
  const reads = state.requests.filter((url) => url.pathname === `${base}/value-changes`).length;
  const summaryReads = state.requests.filter((url) => url.pathname.endsWith("/range")).length;
  const resolved = state.requests.find((url) => url.pathname === `${base}/value-changes`)!;
  const comparison = state.requests.find((url) => url.pathname.endsWith("/range"))!;
  expect(comparison.searchParams.get("from")).toBe(resolved.searchParams.get("from"));
  expect(comparison.searchParams.get("to")).toBe(resolved.searchParams.get("to"));
  await pager(page, "Value changes").getByRole("button", { name: "Next", exact: true }).click();
  await expect(pager(page, "Value changes")).toContainText("21–40 of 42");
  expect(state.requests.filter((url) => url.pathname === `${base}/value-changes`)).toHaveLength(
    reads,
  );
  await page.getByRole("combobox", { name: "Period", exact: true }).selectOption("month");
  await expect(page.getByRole("table", { name: "Value change per month values" })).toBeVisible();
  expect(state.requests.filter((url) => url.pathname.endsWith("/range"))).toHaveLength(
    summaryReads,
  );
  await page.getByRole("button", { name: "1M", exact: true }).click();
  await expect(page.getByRole("combobox", { name: "Period", exact: true })).toHaveValue("week");
  await expect(summary).toContainText("at 2026-09-04 12:15");
  await expect(pager(page, "Value changes")).toContainText("1–20 of 42");
  const appliedReads = state.requests.filter((url) => url.pathname.endsWith("/range")).length;
  await page.getByLabel("From", { exact: true }).fill("2026-09-01 13:00");
  await page.getByLabel("To", { exact: true }).fill("2026-10-01 15:00");
  expect(state.requests.filter((url) => url.pathname.endsWith("/range"))).toHaveLength(
    appliedReads,
  );
  await page.getByRole("button", { name: "Apply", exact: true }).click();
  await expect(summary).toContainText("at 2026-09-01 12:59");
  await expect(summary).toContainText("at 2026-10-01 14:59");
  await page.reload();
  await expect(summary).toContainText("at 2026-09-01 12:59");
  await expect(summary).toContainText("at 2026-10-01 14:59");
});

for (const scenario of ["Missing start", "Missing both endpoints", "Same point"] as const) {
  test(`Range summary shows unknown change and flows for ${scenario.toLowerCase()}`, async ({
    page,
  }) => {
    const state = await fixture(page);
    state.summary = {
      completeness: "incomplete",
      startPoint: scenario === "Same point" ? point(0) : null,
      ...(scenario === "Missing both endpoints" ? { endPoint: null } : {}),
      ...(scenario === "Same point" ? { endPoint: point(0) } : {}),
      amountUsd: null,
      comparable: false,
      flowsCompleteness: "complete",
      contributionsUsd: "0",
      withdrawalsUsd: "0",
      issues:
        scenario === "Same point"
          ? [{ code: "no_point_in_period", detail: "no_point_in_period" }]
          : scenario === "Missing both endpoints"
            ? [
                { code: "no_start_point", detail: "no_start_point" },
                { code: "no_end_point", detail: "no_end_point" },
              ]
            : [{ code: "no_start_point", detail: "no_start_point" }],
    };
    await page.goto(`${historyPath}/value-changes`);
    const summary = page.getByRole("region", { name: "Selected range summary" });
    await expect(summary).toContainText("Incomplete");
    const amount = summary.getByText("Value change", { exact: true }).locator("..");
    await expect(amount).toContainText("—");
    await expect(amount).not.toContainText("$0");
    await expect(summary.getByText("Flows", { exact: true }).locator("..")).toContainText(
      "Flows unavailable",
    );
    await expect(summary).toContainText(
      scenario === "Same point"
        ? "Two distinct valuation points are required"
        : "No start valuation point at or before the selected start",
    );
    if (scenario === "Missing both endpoints") {
      await expect(summary).toContainText("No end valuation point at or before the selected end");
    }
    const request = state.requests.find((url) => url.pathname.endsWith("/range"))!;
    expect(request.searchParams.get("from")).toBe("2026-09-27T04:16:44.000Z");
    await expect(page.getByRole("table", { name: "Value change per day values" })).toBeVisible();
  });
}

test("Range summary retains known incomplete results and expands limitations and boundary details on demand", async ({
  page,
}) => {
  const state = await fixture(page);
  state.summary = {
    amountUsd: "490",
    completeness: "incomplete",
    flowsCompleteness: "incomplete",
    issues: [
      { code: "start_point_incomplete", detail: "start_point_incomplete" },
      { code: "unvalued_flow", symbol: "DEMO", detail: "2" },
    ],
    boundaryContributionsUsd: "10",
    boundaryEvents: [
      {
        cycleId: "entry",
        evaluatedAt: initialEnd,
        accountId: "account",
        accountName: "Entered account",
        direction: "contribution",
        valueUsd: "10",
        completeness: "complete",
        method: "reported",
      },
    ],
  };
  await page.goto(`${historyPath}/value-changes`);
  const summary = page.getByRole("region", { name: "Selected range summary" });
  await expect(summary).toContainText("+$490");
  await expect(summary).toContainText("Incomplete");
  await expect(summary).toContainText("Known flows shown; unvalued amounts: —");
  await expect(summary).toContainText("Capital flow");
  await expect(summary).toContainText("Boundary event");
  await expect(summary.getByText("Unvalued flow", { exact: true })).toBeHidden();
  await summary.getByText("Limitations (2)", { exact: true }).click();
  await expect(summary.getByText("Unvalued flow", { exact: true })).toBeVisible();
  await expect(summary.getByText("DEMO", { exact: true })).toBeVisible();
  const events = summary.getByRole("table", { name: "Boundary events in the range" });
  await expect(events).toBeHidden();
  await summary.getByText("Boundary events (1)", { exact: true }).click();
  await expect(events).toContainText("Entered account");
});

test("Range summary qualifies partially valued boundary flows", async ({ page }) => {
  const state = await fixture(page);
  state.summary = {
    amountUsd: "490",
    completeness: "incomplete",
    flowsCompleteness: "incomplete",
    issues: [
      {
        code: "boundary_incomplete",
        accountName: "Funding",
        detail: "The boundary value is incomplete.",
      },
    ],
    boundaryContributionsUsd: "10",
    boundaryEvents: [
      {
        cycleId: "entry",
        evaluatedAt: initialEnd,
        accountId: "account",
        accountName: "Entered account",
        direction: "contribution",
        valueUsd: "10",
        completeness: "incomplete",
        method: "calculated",
      },
    ],
  };
  await page.goto(`${historyPath}/value-changes`);
  const summary = page.getByRole("region", { name: "Selected range summary" });
  await expect(summary).toContainText("+$490");
  await expect(summary).toContainText("+$10");
  await expect(summary).toContainText("Known flows shown; unvalued amounts: —");
  await summary.getByText("Limitations (1)", { exact: true }).click();
  await expect(summary.getByText("Funding", { exact: true })).toBeVisible();
});

test("Range boundary events reverse cycles while retaining each cycle's account order", async ({
  page,
}) => {
  const state = await fixture(page);
  state.summary = {
    boundaryEvents: ["older", "newer"].flatMap((cycleId) =>
      ["Spot", "Funding", "Manual"].map((accountName) => ({
        cycleId,
        evaluatedAt: initialEnd,
        accountId: `${cycleId}-${accountName}`,
        accountName: `${cycleId} ${accountName}`,
        direction: "withdrawal" as const,
        valueUsd: "10",
        completeness: "complete" as const,
        method: "calculated" as const,
      })),
    ),
  };
  await page.goto(`${historyPath}/value-changes`);
  const summary = page.getByRole("region", { name: "Selected range summary" });
  await summary.getByText("Boundary events (6)", { exact: true }).click();
  const table = summary.getByRole("table", { name: "Boundary events in the range" });
  const accountColumn =
    (await table.getByRole("columnheader").allTextContents()).indexOf("Account") + 1;
  await expect(table.locator(`tbody tr td:nth-child(${accountColumn})`)).toHaveText([
    "newer Spot",
    "newer Funding",
    "newer Manual",
    "older Spot",
    "older Funding",
    "older Manual",
  ]);
});

test("A failed range summary retries its applied window while calendar details remain available", async ({
  page,
}) => {
  const state = await fixture(page);
  state.failSummary = true;
  await page.goto(`${historyPath}/value-changes`);
  const summary = page.getByRole("region", { name: "Selected range summary" });
  await expect(summary.getByRole("alert")).toContainText("Summary temporarily unavailable.", {
    timeout: 15_000,
  });
  await expect(page.getByRole("table", { name: "Value change per day values" })).toBeVisible();
  const first = state.requests.find((url) => url.pathname.endsWith("/range"))!;
  state.failSummary = false;
  state.now = "2026-10-05T04:16:44Z";
  await summary.getByRole("button", { name: "Retry", exact: true }).click();
  await expect(summary).toContainText("+$500");
  const last = state.requests.filter((url) => url.pathname.endsWith("/range")).at(-1)!;
  expect(last.search).toBe(first.search);
});

test("A known zero change between distinct valuation points remains zero", async ({ page }) => {
  const state = await fixture(page);
  state.summary = {
    amountUsd: "0",
    comparable: true,
    flowsCompleteness: "complete",
    contributionsUsd: "0",
    withdrawalsUsd: "0",
    endPoint: { ...point(560), totalUsd: point(0).totalUsd },
  };
  await page.goto(`${historyPath}/value-changes`);
  const summary = page.getByRole("region", { name: "Selected range summary" });
  await expect(summary.getByText("Value change", { exact: true }).locator("..")).toContainText(
    "$0 (0%)",
  );
  await expect(summary).not.toContainText("Incomplete");
});

test("Empty, single-point and last-page URL states stay usable", async ({ page }) => {
  const state = await fixture(page);
  state.count = 0;
  await page.goto(historyPath);
  await expect(
    page
      .getByText("No valuation points in this range.", {
        exact: true,
      })
      .first(),
  ).toBeVisible();
  await expect(pager(page, "Raw valuation points")).toContainText("0 rows");
  state.count = 1;
  await page.reload();
  await expect(
    page.getByText("Only one valuation point in this window; the chart needs two."),
  ).toBeVisible();
  state.count = 561;
  await page.goto(`${historyPath}?pointsOffset=560`);
  await expect(rawTable(page).locator("tbody > tr")).toHaveCount(1);
  await expect(
    pager(page, "Raw valuation points").getByRole("button", { name: "Next", exact: true }),
  ).toBeDisabled();
  await page.goto(`${historyPath}?pointsOffset=900`);
  await expect(pager(page, "Raw valuation points")).toContainText("561–561 of 561");
});

test("A failed raw page retries the same applied window", async ({ page }) => {
  const state = await fixture(page);
  await page.goto(historyPath);
  await expect(rawTable(page).locator("tbody > tr")).toHaveCount(20);
  const resolved = state.requests.find(
    (url) => url.pathname === `${base}/valuation-points/series`,
  )!;
  state.failPage = true;
  await pager(page, "Raw valuation points")
    .getByRole("button", { name: "Next", exact: true })
    .click();
  await expect(page.getByRole("alert")).toContainText("Page temporarily unavailable.", {
    timeout: 15_000,
  });
  state.failPage = false;
  state.now = "2026-10-05T04:16:44Z";
  await page.getByRole("alert").getByRole("button", { name: "Retry", exact: true }).click();
  await expect(pager(page, "Raw valuation points")).toContainText("21–40 of 561");
  const retried = state.requests
    .filter((url) => url.pathname === `${base}/valuation-points`)
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
    await expect(page.getByText(/ · Daily$/)).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(
      375,
    );
    await expect(
      pager(page, "Raw valuation points").getByRole("button", { name: "Next", exact: true }),
    ).toBeEnabled();
    await page.goto(`${historyPath}/value-changes`);
    await expect(page.getByRole("region", { name: "Selected range summary" })).toContainText(
      "+$500",
    );
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(
      375,
    );
    await page.setViewportSize({ width: 812, height: 375 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(
      812,
    );
  });
}

test("Expanding an immutable point does not refresh the raw record page", async ({ page }) => {
  const state = await fixture(page);
  await page.goto(historyPath);
  await expect(rawTable(page).locator("tbody > tr")).toHaveCount(20);
  const before = state.requests.filter((url) => url.pathname === `${base}/valuation-points`).length;
  await rawTable(page)
    .getByRole("button", { name: /Expand the valuation point/ })
    .first()
    .click();
  await expect(rawTable(page)).toContainText("No accounts were inside the boundary at this point.");
  expect(state.requests.filter((url) => url.pathname === `${base}/valuation-points`)).toHaveLength(
    before,
  );
});
