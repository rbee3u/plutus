import { mockApi } from "./support/api";
import { sessionFixture } from "./support/session";
import { expect, test, type Page } from "@playwright/test";
import type {
  AccountReport,
  AccountValuation,
  IdentityList,
  Overview,
  PositionList,
  PositionRow,
} from "../src/api/generated/types.gen";

const portfolioId = "00000000-0000-7000-8000-000000000001";
const cycleId = "00000000-0000-7000-8000-000000000003";
const observed = "2026-10-03T00:00:00Z";

type Notionals = {
  binance: string;
  okx: string;
  bybit: string;
  kraken: string;
};

const defaultNotionals: Notionals = {
  binance: "324",
  okx: "350",
  bybit: "300",
  kraken: "400",
};

function account(
  id: string,
  platform: string,
  accountType: string,
  accountName: string,
  identityName: string,
): AccountValuation {
  return {
    sourceReportId: `report-${id}`,
    reportId: `report-${id}`,
    accountId: id,
    accountName,
    accountType,
    identityId: `identity-${platform}`,
    identityName,
    platform,
    selected: true,
    status: "succeeded",
    method: "reported",
    freshness: "fresh",
    completeness: "complete",
    valueUsd: "100",
    reportObservedAt: observed,
    reportedEquityUsd: "100",
    adjustedEquityUsd: "100",
    availableEquityUsd: "80",
    initialMarginRatio: "0.1",
    maintenanceMarginRatio: "0.01",
    marginRatio: "4.2",
    omitted: [],
  };
}

const accounts = {
  binance: account(
    "00000000-0000-7000-8000-000000000011",
    "binance",
    "portfolio_margin",
    "Portfolio margin",
    "binance-main",
  ),
  okx: account("00000000-0000-7000-8000-000000000012", "okx", "trading", "Trading", "okx-main"),
  bybit: account(
    "00000000-0000-7000-8000-000000000013",
    "bybit",
    "unified",
    "Unified",
    "bybit-main",
  ),
  kraken: account("00000000-0000-7000-8000-000000000014", "kraken", "main", "Main", "kraken-main"),
};

function position(item: AccountValuation, notionalUsd: string): PositionRow {
  return {
    displayLeverage: null,
    marginUnavailableCode: null,
    accountId: item.accountId,
    accountName: item.accountName,
    accountType: item.accountType,
    identityId: item.identityId,
    identityName: item.identityName,
    platform: item.platform,
    positionKey: `${item.accountId}:net`,
    futuresContractId: `${item.accountId}-contract`,
    futuresSymbol: "BTCUSDT",
    kind: "linear_perpetual",
    side: "long",
    positionMode: "one_way",
    sourceQuantity: "1",
    sourceQuantityUnit: "base",
    size: "1",
    sizeSymbol: "BTC",
    baseAssetSymbol: "BTC",
    quoteAssetSymbol: "USDT",
    settlementAssetSymbol: "USDT",
    contractSize: null,
    entryPrice: "100",
    markPrice: "100",
    liquidationPrice: null,
    unrealizedPnl: null,
    marginSetting: "cross",
    marginBasis: "initial",
    marginAssetSymbol: "USDT",
    reportedMargin: null,
    marginSource: null,
    reportedLeverage: null,
    settlementMargin: null,
    portfolioMargin: item.accountType === "portfolio_margin",
    notionalUsd,
    freshness: "fresh",
    reportObservedAt: observed,
    issues: [],
  };
}

const overview: Overview = {
  context: {
    portfolioId,
    timezone: "UTC",
    cycleId,
    evaluatedAt: observed,
  },
  point: {
    cycleId,
    calculationVersion: 1,
    detailState: "full",
    evaluatedAt: observed,
    totalUsd: "400",
    completeness: "complete",
    accountCount: 4,
    staleAccountCount: 0,
    incompleteAccountCount: 0,
    boundaryContributionUsd: "0",
    boundaryWithdrawalUsd: "0",
  },
  platforms: [
    {
      platform: "binance",
      label: "Binance",
      valueUsd: "100",
      completeness: "complete",
      identities: [
        {
          id: "identity-binance",
          displayName: "binance-main",
          status: "enabled",
          connectionStatus: "enabled",
          valueUsd: "100",
          completeness: "complete",
          accounts: [accounts.binance],
        },
      ],
    },
    {
      platform: "okx",
      label: "OKX",
      valueUsd: "100",
      completeness: "complete",
      identities: [
        {
          id: "identity-okx",
          displayName: "okx-main",
          status: "enabled",
          connectionStatus: "enabled",
          valueUsd: "100",
          completeness: "complete",
          accounts: [accounts.okx],
        },
      ],
    },
    {
      platform: "bybit",
      label: "Bybit",
      valueUsd: "100",
      completeness: "complete",
      identities: [
        {
          id: "identity-bybit",
          displayName: "bybit-main",
          status: "enabled",
          connectionStatus: "enabled",
          valueUsd: "100",
          completeness: "complete",
          accounts: [accounts.bybit],
        },
      ],
    },
    {
      platform: "kraken",
      label: "Kraken",
      valueUsd: "100",
      completeness: "complete",
      identities: [
        {
          id: "identity-kraken",
          displayName: "kraken-main",
          status: "enabled",
          connectionStatus: "enabled",
          valueUsd: "100",
          completeness: "complete",
          accounts: [accounts.kraken],
        },
      ],
    },
  ],
  issues: [],
};

const identities: IdentityList = {
  items: [
    {
      id: "identity-binance",
      portfolioId,
      platform: "binance",
      displayName: "binance-main",
      accountStructure: "binance_pm",
      status: "enabled",
      connection: { status: "enabled", credentialsConfigured: true },
      accounts: [],
      version: 1,
      createdAt: observed,
      updatedAt: observed,
    },
  ],
};

type FixtureOptions = {
  notionals?: Partial<Notionals>;
  failPositions?: boolean;
  mismatchCycle?: boolean;
};

async function fixture(page: Page, options: FixtureOptions = {}) {
  const notionals = { ...defaultNotionals, ...options.notionals };
  const state = {
    accountRequests: 0,
    positionsRequests: 0,
  };
  const allPositions = [
    position(accounts.binance, notionals.binance),
    position(accounts.okx, notionals.okx),
    position(accounts.bybit, notionals.bybit),
    position(accounts.kraken, notionals.kraken),
  ];
  const byId = new Map(Object.values(accounts).map((item) => [item.accountId, item]));

  await mockApi(
    page,
    sessionFixture({
      user: { username: "leverage-risk-check" },
      portfolio: { id: portfolioId, name: "Leverage risk check", timezone: "UTC" },
    }),
    async (route) => {
      const request = route.request();
      const path = new URL(request.url()).pathname;
      if (path === "/api/v1/platforms") {
        return route.fulfill({ json: { items: [] } });
      }
      if (path === `/api/v1/portfolios/${portfolioId}/overview`) {
        return route.fulfill({ json: overview });
      }
      if (path === `/api/v1/portfolios/${portfolioId}/identities`) {
        return route.fulfill({ json: identities });
      }
      if (path.endsWith("/history-window")) {
        return route.fulfill({
          json: { context: overview.context, from: observed, to: observed },
        });
      }
      if (path.endsWith("/valuation-points/series")) {
        return route.fulfill({
          json: {
            context: overview.context,
            from: observed,
            to: observed,
            granularity: "raw",
            items: [],
          },
        });
      }
      if (path.endsWith("/positions")) {
        state.positionsRequests += 1;
        if (options.failPositions) {
          return route.fulfill({ status: 503, json: { detail: "Positions unavailable" } });
        }
        return route.fulfill({
          json: {
            context: options.mismatchCycle
              ? { ...overview.context, cycleId: "00000000-0000-7000-8000-000000000099" }
              : overview.context,
            coverage: { complete: true, missingAccountIds: [] },
            items: allPositions,
            page: { kind: "offset", offset: 0, limit: 500, total: allPositions.length },
            filteredTotalUsd: "1374",
            filteredLinearNotionalUsd: "1374",
            filteredInverseNotionalUsd: "0",
            filteredTotalCompleteness: "complete",
            filteredTotalFreshness: "fresh",
            filteredTotalIssues: [],
            issues: [],
          } satisfies PositionList,
        });
      }
      if (path.endsWith("/report") && path.includes("/accounts/")) {
        state.accountRequests += 1;
        const accountId = path.split("/").at(-2) ?? "";
        const item = byId.get(accountId);
        if (item) {
          const report: AccountReport = {
            manualOverlay: null,
            context: overview.context,
            account: item,
            balances: [],
            positions: [position(item, notionals[item.platform as keyof Notionals])],
            dualPositions: [],
            earnPositions: [],
            issues: [],
          };
          return route.fulfill({ json: report });
        }
      }
      return route.fallback();
    },
  );
  return state;
}

test("Overview warns at every level and reddens the expanded Real leverage chip", async ({
  page,
}) => {
  const state = await fixture(page);
  await page.goto(`/p/${portfolioId}/overview`);

  await expect(page.getByRole("heading", { name: "Holdings tree", exact: true })).toBeVisible();
  await expect(page.locator('[data-leverage-warning="true"]')).toHaveCount(6);
  expect(state.positionsRequests).toBe(1);
  expect(state.accountRequests).toBe(0);

  await expect(page.getByRole("button", { name: "Collapse Binance" })).toContainText(
    "High leverage",
  );
  await expect(page.getByRole("button", { name: "Collapse Bybit" })).not.toContainText(
    "High leverage",
  );
  await expect(page.getByRole("button", { name: "Collapse Binance" })).toHaveAccessibleDescription(
    /binance-main \/ Portfolio margin \(3\.24x\)/,
  );
  await expect(
    page.getByRole("button", { name: "Expand binance-main" }),
  ).toHaveAccessibleDescription(/High leverage in Portfolio margin \(3\.24x\)/);

  await page.getByRole("button", { name: "Expand binance-main" }).click();
  const account = page.getByRole("button", {
    name: "Expand balances and positions for Portfolio margin",
  });
  await expect(account).toContainText("High leverage");
  await expect(account).toHaveAccessibleDescription(/Real leverage 3\.24x is above 3x/);
  expect(state.accountRequests).toBe(0);

  await account.click();
  const chip = page.locator('[data-leverage-risk="true"]');
  await expect(chip).toHaveCount(1);
  await expect(chip).toContainText("Real leverage");
  await expect(chip).toContainText("3.24x");
  await expect(page.locator('[data-leverage-warning="true"]')).toHaveCount(7);
  expect(state.accountRequests).toBe(1);
});

test("Exactly 3x stays neutral and makes no warning claim", async ({ page }) => {
  await fixture(page, {
    notionals: { binance: "300", okx: "300", bybit: "300", kraken: "300" },
  });
  await page.goto(`/p/${portfolioId}/overview`);

  await expect(page.getByRole("heading", { name: "Holdings tree", exact: true })).toBeVisible();
  await expect(page.locator('[data-leverage-warning="true"]')).toHaveCount(0);
  await expect(page.getByText("Leverage warnings could not be checked.")).toHaveCount(0);

  await page.getByRole("button", { name: "Expand binance-main" }).click();
  await page
    .getByRole("button", { name: "Expand balances and positions for Portfolio margin" })
    .click();
  const neutralChip = page.getByText("Real leverage", { exact: true }).locator("..");
  await expect(neutralChip).toContainText("3x");
  await expect(page.locator('[data-leverage-risk="true"]')).toHaveCount(0);
});

test("A raw value above 3x warns even when the chip displays 3x", async ({ page }) => {
  await fixture(page, {
    notionals: { binance: "300.01", okx: "300", bybit: "300", kraken: "300" },
  });
  await page.goto(`/p/${portfolioId}/overview`);

  await expect(page.locator('[data-leverage-warning="true"]')).toHaveCount(2);
  await expect(page.getByRole("button", { name: "Collapse Binance" })).toHaveAccessibleDescription(
    /binance-main \/ Portfolio margin \(above 3x\)/,
  );
  await page.getByRole("button", { name: "Expand binance-main" }).click();
  await page
    .getByRole("button", { name: "Expand balances and positions for Portfolio margin" })
    .click();
  const chip = page.locator('[data-leverage-risk="true"]');
  await expect(chip).toHaveCount(1);
  await expect(chip).toContainText("3x");
});

test("A failed positions read keeps the tree and shows the neutral check status", async ({
  page,
}) => {
  await fixture(page, { failPositions: true });
  await page.goto(`/p/${portfolioId}/overview`);

  await expect(page.getByRole("heading", { name: "Holdings tree", exact: true })).toBeVisible();
  await expect(page.getByText("Leverage warnings could not be checked.")).toBeVisible();
  await expect(page.locator('[data-leverage-warning="true"]')).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Collapse Binance" })).toBeVisible();
});

test("A mismatched positions cycle is not used", async ({ page }) => {
  await fixture(page, { mismatchCycle: true });
  await page.goto(`/p/${portfolioId}/overview`);

  await expect(page.getByRole("heading", { name: "Holdings tree", exact: true })).toBeVisible();
  await expect(page.getByText("Leverage warnings could not be checked.")).toBeVisible();
  await expect(page.locator('[data-leverage-warning="true"]')).toHaveCount(0);
});

test("The warning holds at 375px dark and 1280px light without overflow", async ({ page }) => {
  await fixture(page);
  for (const view of [
    { width: 375, height: 812, scheme: "dark" as const },
    { width: 1280, height: 800, scheme: "light" as const },
  ]) {
    await page.setViewportSize({ width: view.width, height: view.height });
    await page.emulateMedia({ colorScheme: view.scheme });
    await page.goto(`/p/${portfolioId}/overview`);

    await expect(page.locator('[data-leverage-warning="true"]')).toHaveCount(6);
    await expect
      .poll(() => page.evaluate(() => document.documentElement.dataset.resolvedTheme))
      .toBe(view.scheme);
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
    );
    expect(overflow).toBeLessThanOrEqual(0);
  }
});
