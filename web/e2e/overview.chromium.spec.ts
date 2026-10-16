import { mockApi } from "./support/api";
import { sessionFixture } from "./support/session";
import { expect, test, type Page } from "@playwright/test";
import type {
  ManualBalanceState,
  AccountValuation,
  BalanceRow,
  DualPositionRow,
  EarnPositionRow,
  Overview,
  PositionRow,
  IdentityList,
  ValuationPointSummary,
} from "../src/api/generated/types.gen";

const portfolioId = "00000000-0000-7000-8000-000000000001";
const observed = "2026-10-04T00:00:00Z";
async function fixture(page: Page) {
  const state = {
    point: "point-1" as string | null,
    value: "100",
    detailReport: undefined as string | undefined,
    failOverview: false,
    failPositions: false,
    positionsReady: Promise.resolve(),
    emptyTree: false,
    hasIdentities: true,
    archivedIdentity: false,
    stale: false,
    unknown: false,
    ownerName: "Owner",
    requests: [] as string[],
    positions: [] as PositionRow[],
    dual: [] as DualPositionRow[],
    earns: [] as EarnPositionRow[],
    includeEarn: false,
    chartPoints: [] as ValuationPointSummary[],
    failChartWindow: false,
    failChartPoints: false,
  };
  const context = () => ({
    portfolioId,
    timezone: "UTC",
    cycleId: state.point,
    evaluatedAt: observed,
  });
  const account = (dual = false): AccountValuation => ({
    sourceReportId: null,
    adjustedEquityUsd: null,
    availableEquityUsd: null,
    initialMarginRatio: null,
    maintenanceMarginRatio: null,
    marginRatio: null,
    accountId: dual ? "dual" : "spot",
    accountName: dual ? "Dual account" : "Spot",
    accountType: dual ? "dual_investment" : "spot",
    reportId: `${state.point}-${dual ? "dual" : "spot"}`,
    identityId: "owner",
    identityName: state.ownerName,
    platform: "binance",
    selected: true,
    status: state.stale || (state.unknown && !dual) ? "failed" : "succeeded",
    method: state.unknown && !dual ? "missing" : "reported",
    freshness: state.stale || (state.unknown && !dual) ? "stale" : "fresh",
    completeness: state.unknown && !dual ? "incomplete" : "complete",
    reportObservedAt: state.unknown && !dual ? null : observed,
    valueUsd: dual ? "0" : state.unknown ? null : state.value,
    reportedEquityUsd: dual ? "0" : state.unknown ? null : state.value,
    omitted: [],
  });
  const overview = (): Overview => ({
    context: context(),
    point: state.point
      ? {
          cycleId: state.point,

          calculationVersion: 1,
          detailState: "full" as const,
          evaluatedAt: observed,
          totalUsd: state.value,
          completeness: "complete",
          accountCount: 2,
          staleAccountCount: 0,
          incompleteAccountCount: 0,
          boundaryContributionUsd: "0",
          boundaryWithdrawalUsd: "0",
        }
      : null,
    platforms: [],
    issues: [],
  });
  const tree = (): Pick<Overview, "platforms" | "issues"> => ({
    issues: [
      {
        code: "test_omission",
        accountName: "Spot",
        detail: "Named omission remains visible while accounts are folded.",
      },
    ],
    platforms: state.emptyTree
      ? []
      : [
          {
            platform: "binance",
            label: "Binance",
            valueUsd: state.value,
            completeness: "complete",
            identities: [
              {
                id: "owner",
                displayName: state.ownerName,
                status: "enabled",
                connectionStatus: "enabled",
                valueUsd: state.value,
                completeness: "complete",
                accounts: [
                  account(),
                  account(true),
                  ...(state.includeEarn
                    ? [
                        {
                          ...account(),
                          accountId: "earn",
                          accountName: "Earn",
                          accountType: "earn",
                          reportId: `${state.point}-earn`,
                        },
                      ]
                    : []),
                ],
              },
            ],
          },
        ],
  });
  await mockApi(
    page,
    sessionFixture({
      user: { username: "overview" },
      portfolio: {
        id: portfolioId,
        name: "Overview check",
        createdAt: observed,
        updatedAt: observed,
      },
    }),
    async (route) => {
      const request = route.request();
      const path = new URL(request.url()).pathname;
      state.requests.push(`${request.method()} ${path}`);
      if (path.endsWith("/overview"))
        return route.fulfill(
          state.failOverview
            ? { status: 503, json: { detail: "Overview unavailable" } }
            : { json: { ...overview(), ...tree() } },
        );
      if (path.includes("/accounts/")) {
        const dual = path.includes("/accounts/dual/");
        const a = path.includes("/accounts/earn/")
          ? {
              ...account(),
              accountId: "earn",
              accountName: "Earn",
              accountType: "earn",
              reportId: `${state.point}-earn`,
            }
          : account(dual);
        if (state.detailReport) a.reportId = state.detailReport;
        await state.positionsReady;
        if (state.failPositions)
          return route.fulfill({ status: 503, json: { detail: "Positions unavailable" } });
        const balance: BalanceRow = {
          availableToWithdraw: null,
          availableToTrade: null,
          liability: null,
          interest: null,
          unrealizedPnl: null,
          accountId: a.accountId,
          accountName: a.accountName,
          accountType: a.accountType,
          identityId: "owner",
          identityName: state.ownerName,
          platform: "binance",
          assetId: "usd",
          assetSymbol: "USD",
          canonicalAssetSymbol: "USD",
          equity: a.reportedEquityUsd!,
          equityUsd: a.reportedEquityUsd,
          priceUsd: "1",
          noPrice: false,
          freshness: a.freshness,
          reportObservedAt: observed,
        };
        return route.fulfill({
          json: {
            context: context(),
            account: a,
            balances: [balance],
            positions: dual || a.accountType === "earn" ? [] : state.positions,
            dualPositions: dual ? state.dual : [],
            earnPositions: a.accountType === "earn" ? state.earns : [],
            issues: [],
          },
        });
      }
      const cycle = {
        id: "cycle",
        portfolioId,
        trigger: "user",
        createdAt: observed,
        startedAt: observed,
        attemptCount: 1,
      };
      if (path.endsWith("/collection-cycles") && request.method() === "POST")
        return route.fulfill({ status: 202, json: { ...cycle, status: "queued" } });
      if (path.endsWith("/collection-cycles/cycle")) {
        state.point = "point-2";
        state.value = "200";
        return route.fulfill({
          json: {
            cycle: { ...cycle, status: "succeeded", finishedAt: observed, point: overview().point },
            runs: [],
            issues: [],
          },
        });
      }
      if (path.endsWith("/platforms")) return route.fulfill({ json: { items: [] } });
      if (path.endsWith("/history-window")) {
        const preset = new URL(request.url()).searchParams.get("preset");
        const days = preset === "1D" ? 1 : preset === "1M" ? 30 : 7;
        const from = new Date(Date.parse(observed) - days * 86400000).toISOString();
        return route.fulfill(
          state.failChartWindow
            ? { status: 503, json: { detail: "Chart window unavailable" } }
            : { json: { context: context(), from, to: observed } },
        );
      }
      if (path.endsWith("/valuation-points/series"))
        return route.fulfill(
          state.failChartPoints
            ? { status: 503, json: { detail: "Chart points unavailable" } }
            : {
                json: {
                  context: context(),
                  from: observed,
                  to: observed,
                  granularity: "raw",
                  items: state.chartPoints,
                },
              },
        );
      if (path.endsWith("/manual-balances")) {
        const identityId = path.split("/").at(-2)!;
        return route.fulfill({
          json: {
            identityId,
            accountId: `manual-${identityId}`,
            version: 1,
            appliedVersion: null,
            items: [],
            appliedItems: [],
            pending: false,
          } satisfies ManualBalanceState,
        });
      }
      if (request.method() === "GET" && path.endsWith("/collection-cycles"))
        return route.fulfill({
          json: {
            items: [],
            page: { kind: "cursor", limit: 50, nextCursor: null, hasMore: false },
          },
        });
      if (path === `/api/v1/portfolios/${portfolioId}/identities`)
        return route.fulfill({
          json: {
            items: state.hasIdentities
              ? [
                  {
                    id: "owner",
                    portfolioId,
                    platform: "binance",
                    displayName: state.ownerName,
                    accountStructure: "classic_trading",
                    status: state.archivedIdentity ? "archived" : "enabled",
                    connection: { status: "enabled", credentialsConfigured: true },
                    accounts: [],
                    version: 1,
                    createdAt: observed,
                    updatedAt: observed,
                  },
                ]
              : [],
          } satisfies IdentityList,
        });
      if (path === `/api/v1/portfolios/${portfolioId}/balances`)
        return route.fulfill({
          json: {
            context: context(),
            items: [],
            page: { kind: "offset", offset: 0, limit: 50, total: 0 },
            filteredTotalCompleteness: "complete",
            filteredTotalIssues: [],
            manualOverlay: null,
            coverage: { complete: true, missingAccountIds: [] },
            filteredTotalUsd: "0",
            valuationCompleteness: "complete",
            issues: [],
          },
        });
      return route.fallback();
    },
  );
  return state;
}

async function openSpot(page: Page) {
  await page.getByRole("button", { name: "Expand Owner", exact: true }).click();
  await page
    .getByRole("button", { name: "Expand balances and positions for Spot", exact: true })
    .click();
}

test("Overview keeps chart ranges available after a sparse result", async ({ page }) => {
  const state = await fixture(page);
  await page.goto(`/p/${portfolioId}/overview`);
  await expect(page.getByRole("tab", { name: "1W", exact: true })).toBeVisible();
  await expect(
    page.getByText("At least two valuation points are needed to show a trend."),
  ).toBeVisible();
  state.chartPoints = [
    {
      cycleId: "earlier-point",
      calculationVersion: 1,
      detailState: "full",
      evaluatedAt: "2026-09-10T00:00:00Z",
      totalUsd: "90",
      completeness: "complete",
      accountCount: 2,
      staleAccountCount: 0,
      incompleteAccountCount: 0,
      boundaryContributionUsd: "0",
      boundaryWithdrawalUsd: "0",
    },
    {
      cycleId: "latest-point",
      calculationVersion: 1,
      detailState: "full",
      evaluatedAt: observed,
      totalUsd: "100",
      completeness: "complete",
      accountCount: 2,
      staleAccountCount: 0,
      incompleteAccountCount: 0,
      boundaryContributionUsd: "0",
      boundaryWithdrawalUsd: "0",
    },
  ];
  await page.getByRole("tab", { name: "1M", exact: true }).click();
  await expect(
    page.getByRole("img", { name: "Recent valuation points", exact: true }),
  ).toBeVisible();
  state.chartPoints = [];
  await page.getByRole("tab", { name: "1D", exact: true }).click();
  await expect(
    page.getByText("At least two valuation points are needed to show a trend."),
  ).toBeVisible();
  await expect(page.getByRole("tab", { name: "1M", exact: true })).toBeVisible();
});

test("Overview chart failures recover independently of the summary", async ({ page }) => {
  const state = await fixture(page);
  state.failChartWindow = true;
  state.failChartPoints = true;
  await page.goto(`/p/${portfolioId}/overview`);
  await expect(page.getByRole("heading", { name: "Total net worth", exact: true })).toBeVisible();
  const windowError = page.getByRole("alert").filter({ hasText: "Chart window unavailable" });
  await expect(windowError).toBeVisible();
  state.failChartWindow = false;
  await windowError.getByRole("button", { name: "Retry", exact: true }).click();
  const pointsError = page.getByRole("alert").filter({ hasText: "Chart points unavailable" });
  await expect(pointsError).toBeVisible();
  state.failChartPoints = false;
  await pointsError.getByRole("button", { name: "Retry", exact: true }).click();
  await expect(pointsError).toHaveCount(0);
  await expect(page.getByRole("tab", { name: "1W", exact: true })).toBeVisible();
  expect(state.requests.filter((path) => path.endsWith("/overview"))).toHaveLength(1);
});

test("Overview replaces groups with an identity-depth tree and accessible drilldown", async ({
  page,
}) => {
  const state = await fixture(page);
  await page.goto(`/p/${portfolioId}/overview`);
  await expect(page.getByRole("heading", { name: /^By / })).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "Holdings tree", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Collapse Binance" })).toHaveAttribute(
    "aria-expanded",
    "true",
  );
  await expect(page.getByRole("button", { name: "Expand Owner", exact: true })).toHaveAttribute(
    "aria-expanded",
    "false",
  );
  await expect(
    page.getByRole("button", { name: "Expand balances and positions for Spot" }),
  ).toHaveCount(0);
  expect(state.requests.filter((path) => path.includes("/accounts/"))).toHaveLength(0);
  await expect(
    page.getByText("Named omission remains visible while accounts are folded.").first(),
  ).toBeVisible();
  const identity = page.getByRole("button", { name: "Expand Owner", exact: true });
  await identity.focus();
  await page.keyboard.press("Enter");
  await page.getByRole("button", { name: "Expand balances and positions for Spot" }).click();
  await expect(page.getByRole("table", { name: "Balances of Spot" })).toContainText("USD");
  expect(state.requests.filter((path) => path.includes("/accounts/spot/"))).toHaveLength(1);
  await expect(page.getByRole("heading", { name: "Futures", exact: true })).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "Balances", exact: true })).toHaveCount(1);
  await page.getByRole("button", { name: "Collapse Binance" }).click();
  await expect(page.getByRole("button", { name: "Collapse Owner" })).toHaveCount(0);
  await page.getByRole("button", { name: "Expand Binance" }).click();
  await expect(page.getByRole("table", { name: "Balances of Spot" })).toBeVisible();
  await page
    .getByRole("button", { name: "Expand balances and positions for Dual account" })
    .click();
  await expect
    .poll(() => state.requests.some((path) => path.includes("/accounts/dual/report")))
    .toBe(true);
  await expect(page.getByRole("heading", { name: "Dual investment", exact: true })).toHaveCount(0);
  await page.getByRole("link", { name: "Holdings", exact: true }).click();
  await expect(page).toHaveURL(`/p/${portfolioId}/holdings/balances`);
  await page.getByRole("link", { name: "Overview", exact: true }).click();
  await expect(page.getByRole("button", { name: "Expand Owner", exact: true })).toBeVisible();
});

test("Manual collection updates both totals and open details without display polling", async ({
  page,
}) => {
  const state = await fixture(page);
  await page.clock.install();
  await page.goto(`/p/${portfolioId}/overview`);
  await openSpot(page);
  await expect(page.getByRole("table", { name: "Balances of Spot" })).toContainText("100");
  const requests = state.requests.filter((path) => /\/(overview|tree)$/.test(path)).length;
  await page.clock.runFor(16000);
  expect(state.requests.filter((path) => /\/(overview|tree)$/.test(path))).toHaveLength(requests);
  await page.getByRole("button", { name: "Collect now", exact: true }).click();
  await page.clock.runFor(2500);
  await expect(page.getByRole("table", { name: "Balances of Spot" })).toContainText("200");
  await expect(page.getByRole("button", { name: "Collapse Owner", exact: true })).toBeVisible();
  await expect(page.getByRole("article").first()).toContainText("200");
  expect(state.requests.filter((path) => path.startsWith("POST"))).toHaveLength(1);
  await page.reload();
  await expect(page.getByRole("button", { name: "Expand Owner", exact: true })).toBeVisible();
  expect(state.requests.filter((path) => path.startsWith("POST"))).toHaveLength(1);
});

test("Overview retries the summary and ownership tree as one response", async ({ page }) => {
  const state = await fixture(page);
  state.failOverview = true;
  await page.goto(`/p/${portfolioId}/overview`);
  await expect(page.getByRole("alert")).toContainText("Overview unavailable");
  await expect(page.getByRole("article")).toHaveCount(0);
  state.failOverview = false;
  await page.getByRole("button", { name: "Retry", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Holdings tree", exact: true })).toBeVisible();
  state.failOverview = true;
  await page.getByRole("button", { name: "Collect now", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("Showing the last successful result");
  await expect(page.getByRole("article").first()).toContainText("100");
  await expect(page.getByRole("button", { name: "Collapse Binance" })).toContainText("100");
  state.failOverview = false;
  await page.getByRole("button", { name: "Retry", exact: true }).click();
  await expect(page.getByRole("article").first()).toContainText("200");
  expect(state.requests.some((path) => path.endsWith("/tree"))).toBe(false);
});

test("Empty, loading, failed and newer account reports are not treated as empty positions", async ({
  page,
}) => {
  const state = await fixture(page);
  let releasePositions!: () => void;
  state.positionsReady = new Promise<void>((resolve) => {
    releasePositions = resolve;
  });
  state.failPositions = true;
  try {
    await page.goto(`/p/${portfolioId}/overview`);
    await openSpot(page);
    await expect(page.getByRole("heading", { name: "Futures", exact: true })).toBeVisible();
    await expect(page.getByText("Loading positions.", { exact: true })).toBeVisible();
  } finally {
    releasePositions();
  }
  await expect(page.getByRole("alert")).toContainText("Positions unavailable");
  await expect(page.getByRole("heading", { name: "Futures", exact: true })).toBeVisible();
  state.failPositions = false;
  await page.getByRole("button", { name: "Retry", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Futures", exact: true })).toHaveCount(0);
  await page.getByRole("button", { name: "Collapse balances and positions for Spot" }).click();
  state.detailReport = "newer-report";
  await page.getByRole("button", { name: "Expand balances and positions for Spot" }).click();
  await expect(page.getByRole("alert").first()).toContainText(
    "does not match the selected valuation point",
  );
  await expect(page.getByRole("table", { name: "Balances of Spot" })).toContainText("100");
  await expect(page.getByRole("alert").first()).toContainText("Showing the last successful result");
});

for (const noPoint of [false, true]) {
  test(`Overview handles ${noPoint ? "no point" : "empty tree"} without duplicate notices`, async ({
    page,
  }) => {
    const state = await fixture(page);
    state.emptyTree = true;
    if (noPoint) state.point = null;
    await page.goto(`/p/${portfolioId}/overview`);
    await expect(page.getByRole("button", { name: "Collect now" })).toBeVisible();
    if (noPoint) {
      await expect(page.getByRole("alert")).toHaveCount(0);
      await expect(page.getByRole("article")).toHaveCount(0);
    } else
      await expect(page.getByRole("region", { name: "Holdings tree", exact: true })).toContainText(
        "No accounts to display.",
      );
  });

  test(`Overview guides a portfolio without identities ${noPoint ? "before" : "after"} an empty collection`, async ({
    page,
  }) => {
    const state = await fixture(page);
    state.hasIdentities = false;
    state.emptyTree = true;
    state.value = "0";
    if (noPoint) state.point = null;
    await page.goto(`/p/${portfolioId}/overview`);
    await expect(page.getByRole("heading", { name: "No identities yet" })).toBeVisible();
    await expect(page.getByText("Add an identity to start tracking account values.")).toBeVisible();
    await expect(page.getByRole("button", { name: "Collect now" })).toHaveCount(0);
    await page.getByRole("link", { name: "Manage identities" }).click();
    await expect(page).toHaveURL(`/p/${portfolioId}/data/identities`);
  });
}

test("Overview guides archived-only setup without hiding retained valuations", async ({ page }) => {
  const state = await fixture(page);
  state.archivedIdentity = true;
  state.emptyTree = true;
  state.point = null;
  await page.goto(`/p/${portfolioId}/overview`);
  await expect(page.getByRole("heading", { name: "No active identities" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Collect now" })).toHaveCount(0);
  await expect(page.getByRole("link", { name: "Manage identities" })).toBeVisible();
  state.point = "retained-point";
  await page.reload();
  await expect(page.getByRole("article").first()).toContainText("100");
  await expect(page.getByRole("heading", { name: "No active identities" })).toHaveCount(0);
});

test("Activity cycles list does not poll", async ({ page }) => {
  const state = await fixture(page);
  await page.clock.install();
  await page.goto(`/p/${portfolioId}/data/activity?tab=cycles`);
  await expect(page.getByRole("table")).toBeVisible();
  const path = "/collection-cycles";
  await expect
    .poll(() => state.requests.filter((request) => request.endsWith(path)).length)
    .toBe(1);
  await page.clock.runFor(16000);
  expect(state.requests.filter((request) => request.endsWith(path))).toHaveLength(1);
});

test("Stale futures and dual investment remain visible and a failed initial balance read stays unknown", async ({
  page,
}) => {
  const state = await fixture(page);
  state.stale = true;
  state.positions = [
    {
      contractSize: null,
      settlementMargin: null,
      entryPrice: null,
      markPrice: null,
      liquidationPrice: null,
      unrealizedPnl: null,
      displayLeverage: null,
      marginUnavailableCode: null,
      reportedLeverage: null,
      reportedMargin: null,
      notionalUsd: null,
      accountId: "spot",
      accountName: "Spot",
      accountType: "spot",
      identityId: "owner",
      identityName: state.ownerName,
      platform: "binance",
      positionKey: "btc",
      futuresContractId: "btc",
      futuresSymbol: "BTCUSDT",
      kind: "linear_perpetual",
      side: "long",
      sourceQuantity: "1",
      sourceQuantityUnit: "base",
      baseAssetSymbol: "BTC",
      quoteAssetSymbol: "USDT",
      size: "1",
      sizeSymbol: "BTC",
      freshness: "stale",
      reportObservedAt: observed,
      issues: [{ code: "stale", detail: "Earlier successful report retained." }],
    },
  ];
  state.dual = [
    {
      targetPrice: null,
      annualizedRate: null,
      accountId: "dual",
      accountName: "Dual account",
      accountType: "dual_investment",
      identityId: "owner",
      identityName: state.ownerName,
      platform: "binance",
      positionKey: "subscription",
      baseAssetSymbol: "BTC",
      quoteAssetSymbol: "USDT",
      principalAssetSymbol: "USDT",
      principal: "100",
      direction: "buy_low",
      status: "running",
      freshness: "stale",
      reportObservedAt: observed,
    },
  ];
  await page.goto(`/p/${portfolioId}/overview`);
  await openSpot(page);
  await expect(page.getByRole("heading", { name: "Futures", exact: true })).toBeVisible();
  await expect(page.getByRole("table", { name: "Positions of Spot" })).toContainText("BTCUSDT");
  await expect(page.getByRole("table", { name: "Positions of Spot" })).toContainText(/stale/i);
  await page
    .getByRole("button", { name: "Expand balances and positions for Dual account" })
    .click();
  await expect(page.getByRole("heading", { name: "Dual investment", exact: true })).toBeVisible();
  await expect(page.getByRole("table", { name: "Dual investment positions" })).toContainText(
    "Stale",
  );
  await page.reload();
  state.detailReport = "newer-report";
  await openSpot(page);
  await expect(page.getByRole("alert").first()).toContainText(
    "does not match the selected valuation point",
  );
  await expect(page.getByRole("table", { name: "Balances of Spot" })).toContainText(
    "Balances could not be loaded. Retry above.",
  );
  await expect(
    page
      .getByRole("table", { name: "Balances of Spot" })
      .getByRole("cell", { name: "USD", exact: true }),
  ).toHaveCount(0);
});

for (const viewport of [
  { width: 1280, height: 800 },
  { width: 375, height: 812 },
]) {
  for (const theme of ["light", "dark"] as const) {
    test(`Nested account details stay contained at ${viewport.width}px in ${theme} mode`, async ({
      page,
    }) => {
      const state = await fixture(page);
      state.ownerName = `Owner-${"x".repeat(60)}`;
      await page.setViewportSize(viewport);
      await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
      await page.goto(`/p/${portfolioId}/overview`);
      const tree = page.getByRole("region", { name: "Holdings tree", exact: true });
      await expect(tree.getByRole("table")).toHaveCount(0);
      await expect(tree.getByRole("button", { name: "Collapse Binance" })).toHaveText(
        /^Binance\$100(\d+(?:\.\d+)?%)?$/,
      );
      await expect(tree).not.toContainText(/\d+ (identities|accounts)|connection enabled/);
      await tree.getByRole("button", { name: `Expand ${state.ownerName}`, exact: true }).click();
      const account = tree.getByRole("button", {
        name: "Expand balances and positions for Spot",
        exact: true,
      });
      // Clicking the visible name exercises the whole native disclosure button.
      await account.getByText("Spot", { exact: true }).click();
      const open = tree.getByRole("button", {
        name: "Collapse balances and positions for Spot",
        exact: true,
      });
      await expect(open).toHaveAttribute("aria-expanded", "true");
      const balances = tree.getByRole("table", { name: "Balances of Spot" });
      await expect(balances).toContainText("USD");
      await expect(tree.getByText("Equity", { exact: true }).first()).toBeVisible();
      await expect(tree.getByText("AdjEq", { exact: true })).toHaveCount(0);
      const source = tree.locator("details");
      await expect(source).not.toHaveAttribute("open");
      await expect(tree.getByText("reported", { exact: true })).toBeHidden();
      await source.locator("summary").focus();
      await page.keyboard.press("Enter");
      await expect(source).toHaveAttribute("open");
      await expect(source).toContainText("Account value uses equity reported by the platform.");
      await expect(source).toContainText("2026-10-04 00:00");
      const bounds = await open.boundingBox();
      const wrapper = balances.locator("..");
      const tableBounds = await wrapper.boundingBox();
      expect(bounds).not.toBeNull();
      expect(tableBounds).not.toBeNull();
      expect(tableBounds!.x).toBeGreaterThan(bounds!.x);
      expect(tableBounds!.x + tableBounds!.width).toBeLessThanOrEqual(bounds!.x + bounds!.width);
      expect(
        await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
      ).toBe(true);
      if (viewport.width < 900) {
        await wrapper.evaluate((element) => {
          element.scrollLeft = 150;
        });
        expect(await wrapper.evaluate((element) => element.scrollLeft)).toBeGreaterThan(0);
        expect(
          await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
        ).toBe(true);
      }
      await open.focus();
      await page.keyboard.press("Space");
      await expect(balances).toHaveCount(0);
      await expect(
        tree.getByRole("button", { name: "Expand balances and positions for Spot" }),
      ).toBeFocused();
    });
  }
}

test("Unknown and stale valuations keep exceptions inside account details", async ({ page }) => {
  const state = await fixture(page);
  state.unknown = true;
  await page.goto(`/p/${portfolioId}/overview`);
  await page.getByRole("button", { name: "Expand Owner", exact: true }).click();
  const spot = page.getByRole("button", {
    name: "Expand balances and positions for Spot",
    exact: true,
  });
  await expect(spot).toHaveText("Spot—");
  await expect(spot).not.toContainText(/failed|stale|incomplete/);
  await spot.click();
  const tree = page.getByRole("region", { name: "Holdings tree", exact: true });
  await expect(tree.getByText("Incomplete", { exact: true })).toBeVisible();
  await expect(
    tree.getByText("No compatible successful account report; holdings and value are unknown."),
  ).toBeVisible();
  await expect(
    page.getByText("Named omission remains visible while accounts are folded.").first(),
  ).toBeVisible();
  state.unknown = false;
  state.stale = true;
  await page.reload();
  await openSpot(page);
  await expect(
    tree.getByText("Earlier successful account report retained", { exact: false }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Collapse balances and positions for Spot" }),
  ).toHaveText("Spot$100");
  await page.getByRole("button", { name: "Collapse Owner", exact: true }).click();
  await expect(tree.getByText("Failed", { exact: true })).toHaveCount(0);
  await expect(
    page.getByText("Named omission remains visible while accounts are folded.").first(),
  ).toBeVisible();
});

test("Overview shares earn columns without ownership and rejects mismatched reports", async ({
  page,
}) => {
  const state = await fixture(page);
  state.includeEarn = true;
  state.earns = [
    {
      accountId: "earn",
      accountName: "Earn",
      accountType: "earn",
      identityId: "owner",
      identityName: "Owner",
      platform: "binance",
      positionKey: "flex:ETH",
      type: "flexible",
      granularity: "product",
      amountBasis: "total",
      assets: [{ assetSymbol: "ETH", amount: "1" }],
      rates: [],
      rewards: [],
      details: [],
      issues: [],
      freshness: "fresh",
      reportObservedAt: observed,
    },
  ];
  await page.setViewportSize({ width: 375, height: 812 });
  await page.goto(`/p/${portfolioId}/overview`);
  await page.getByRole("button", { name: "Expand Owner", exact: true }).click();
  await page
    .getByRole("button", { name: "Expand balances and positions for Earn", exact: true })
    .click();
  const table = page.getByRole("table", { name: "Earn positions", exact: true });
  await expect(table.getByRole("columnheader")).toHaveText([
    "Asset",
    "Type",
    "Maturity",
    "Rate",
    "Amount",
    "Rewards",
    "Subscribed",
    "Status",
    "Issues",
  ]);
  await expect(table.locator("tbody > tr")).toHaveCount(1);
  expect(state.requests.some((path) => path.includes("/accounts/earn/positions"))).toBe(false);
  const wrapper = table.locator("..");
  await wrapper.evaluate((element) => {
    element.scrollLeft = 200;
  });
  expect(await wrapper.evaluate((element) => element.scrollLeft)).toBeGreaterThan(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(
    true,
  );
  state.detailReport = "newer-report";
  await page.reload();
  await page.getByRole("button", { name: "Expand Owner", exact: true }).click();
  await page
    .getByRole("button", { name: "Expand balances and positions for Earn", exact: true })
    .click();
  await expect(
    page
      .getByText("The account response does not match the selected valuation point and report.")
      .first(),
  ).toBeVisible();
  await expect(table).not.toContainText("1 ETH");
});
