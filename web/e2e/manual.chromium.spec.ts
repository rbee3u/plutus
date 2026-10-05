import { expect, test, type Page } from "@playwright/test";
import type {
  AccountValuation,
  ManualBalance,
  ManualBalanceState,
} from "../src/api/generated/types.gen";
const portfolioId = "00000000-0000-7000-8000-000000000001";
const at = "2026-10-05T00:00:00Z";
async function fixture(page: Page, initialPoint = true) {
  const state = {
    point: initialPoint,
    version: 1,
    items: [] as ManualBalance[],
    applied: [] as ManualBalance[],
    privateWrites: 0,
    conflict: false,
  };
  const ledger = (): ManualBalanceState => ({
    accountId: "manual",
    identityId: "owner",
    version: state.version,
    appliedVersion: state.point ? 1 : null,
    items: state.items,
    appliedItems: state.applied,
    pending:
      JSON.stringify(state.items.map((i) => [i.symbol, i.quantity])) !==
      JSON.stringify(state.applied.map((i) => [i.symbol, i.quantity])),
  });
  const account = (): AccountValuation => ({
    accountId: "manual",
    accountName: "Manual",
    accountType: "manual",
    reportId: "report",
    identityId: "owner",
    identityName: "Owner",
    platform: "kraken",
    selected: true,
    status: "ok",
    method: "calculated",
    freshness: "fresh",
    completeness: "complete",
    valueUsd: state.applied[0]?.quantity ?? "0",
    reportObservedAt: at,
    omitted: [],
  });
  const context = () => ({
    portfolioId,
    timezone: "UTC",
    pointId: state.point ? "point" : null,
    evaluatedAt: state.point ? at : null,
  });
  const point = () =>
    state.point
      ? {
          id: "point",
          jobId: "job",
          evaluatedAt: at,
          totalUsd: state.applied[0]?.quantity ?? "0",
          completeness: "complete",
          accountCount: 1,
          staleAccountCount: 0,
          incompleteAccountCount: 0,
          boundaryContributionUsd: "0",
          boundaryWithdrawalUsd: "0",
        }
      : null;
  await page.route("**/api/v1/**", async (route) => {
    const req = route.request();
    const path = new URL(req.url()).pathname;
    const method = req.method();
    const portfolio = {
      id: portfolioId,
      name: "Manual QA",
      timezone: "UTC",
      collectionIntervalSeconds: 3600,
      undeletable: true,
      version: 1,
      createdAt: at,
      updatedAt: at,
    };
    if (path.endsWith("/auth/me"))
      return route.fulfill({
        json: { user: { id: "user", username: "alice", status: "active" }, sessionCount: 1 },
      });
    if (path.endsWith("/auth/csrf")) return route.fulfill({ json: { token: "test" } });
    if (path === "/api/v1/portfolios") return route.fulfill({ json: { items: [portfolio] } });
    if (path === `/api/v1/portfolios/${portfolioId}`) return route.fulfill({ json: portfolio });
    if (path.endsWith("/identities"))
      return route.fulfill({
        json: {
          items: [
            {
              id: "owner",
              portfolioId,
              platform: "kraken",
              displayName: "Owner",
              accountStructure: "kraken_unified",
              status: "enabled",
              version: 1,
              createdAt: at,
              updatedAt: at,
              connection: { status: "unconfigured", credentialsConfigured: false },
              accounts: [
                {
                  id: "manual",
                  identityId: "owner",
                  accountType: "manual",
                  displayName: "Manual",
                  selected: true,
                  version: 1,
                },
              ],
            },
          ],
        },
      });
    if (path.endsWith("/platforms")) return route.fulfill({ json: { items: [] } });
    if (path.endsWith("/asset-listings"))
      return route.fulfill({ json: { items: [], page: { offset: 0, limit: 200, total: 0 } } });
    if (path.includes("/manual-balances")) {
      if (method !== "GET") {
        if (state.conflict || req.headers()["if-match"] !== `"manual:${state.version}"`) {
          state.conflict = false;
          return route.fulfill({ status: 412, json: { detail: "Concurrent update" } });
        }
        const body =
          method === "DELETE"
            ? null
            : (req.postDataJSON() as { symbol?: string; quantity: string });
        const symbol = path.endsWith("/manual-balances")
          ? (body?.symbol ?? "").trim().toUpperCase()
          : path.split("/").at(-1)!;
        const quantity = method === "DELETE" ? "0" : (body?.quantity ?? "0");
        state.items =
          quantity === "0"
            ? []
            : [{ symbol, quantity, assetSymbol: symbol, listingId: "listing", modifiedAt: at }];
        state.version++;
      }
      return route.fulfill({ json: ledger(), headers: { ETag: `"manual:${state.version}"` } });
    }
    if (path.endsWith("/overview"))
      return route.fulfill({
        json: {
          context: context(),
          point: point(),
          totalUsd: point()?.totalUsd ?? null,
          completeness: "complete",
          accountCount: 1,
          staleAccountCount: 0,
          incompleteAccountCount: 0,
          byPlatform: [],
          byIdentity: [],
          issues: [],
          sparkline: [],
        },
      });
    if (path.endsWith("/tree"))
      return route.fulfill({
        json: {
          context: context(),
          totalUsd: point()?.totalUsd ?? null,
          completeness: "complete",
          issues: [],
          platforms: [
            {
              platform: "kraken",
              label: "Kraken",
              valueUsd: point()?.totalUsd ?? "0",
              completeness: "complete",
              identities: [
                {
                  id: "owner",
                  displayName: "Owner",
                  status: "enabled",
                  connectionStatus: "unconfigured",
                  valueUsd: point()?.totalUsd ?? "0",
                  completeness: "complete",
                  accounts: state.applied.length ? [account()] : [],
                },
              ],
            },
          ],
        },
      });
    if (path.endsWith("/balances")) {
      const items = state.applied.map((item) => ({
        ...item,
        ...account(),
        symbol: item.symbol,
        equity: item.quantity,
        equityUsd: item.quantity,
        priceUsd: "1",
        noPrice: false,
      }));
      return route.fulfill({
        json: {
          context: context(),
          account: account(),
          items,
          totalUsd: point()?.totalUsd ?? null,
          completeness: "complete",
          groups: [],
          issues: [],
          page: { offset: 0, limit: 50, total: items.length },
        },
      });
    }
    if (path.includes("/collection-cycles")) {
      if (method === "POST") {
        state.applied = state.items.map((i) => ({ ...i }));
        state.point = true;
        state.privateWrites++;
      }
      return route.fulfill({
        json:
          method === "POST"
            ? {
                id: "cycle",
                portfolioId,
                status: "succeeded",
                trigger: "manual",
                attempts: 1,
                maxAttempts: 3,
                createdAt: at,
              }
            : {
                cycle: {
                  id: "cycle",
                  portfolioId,
                  status: "succeeded",
                  trigger: "manual",
                  attempts: 1,
                  maxAttempts: 3,
                  createdAt: at,
                },
              },
      });
    }
    return route.fulfill({ json: { items: [] } });
  });
  return state;
}

test("all three manual entries share fresh editing and keep pending values separate", async ({
  page,
}) => {
  const state = await fixture(page);
  await page.goto(`/p/${portfolioId}/overview`);
  await page.getByRole("button", { name: "Add manual balance for Owner", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Manual balances" });
  await dialog.getByRole("combobox", { name: "Asset code" }).fill("USD");
  await dialog.getByRole("textbox", { name: /^Quantity/ }).fill("100");
  await dialog.getByRole("button", { name: "Save quantity", exact: true }).click();
  await expect(dialog.getByRole("status").filter({ hasText: "Saved." })).toBeVisible();
  expect(state.privateWrites).toBe(0);
  await dialog.getByRole("button", { name: "Close dialog" }).click();
  await page.getByRole("button", { name: "Expand Owner", exact: true }).click();
  await expect(page.getByRole("table", { name: "Pending manual quantities" })).toContainText("100");
  await expect(
    page.getByRole("button", { name: "Expand balances and positions for Manual" }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "Collect now", exact: true }).click();
  await page
    .getByRole("button", { name: "Expand balances and positions for Manual", exact: true })
    .click();
  await page.getByRole("button", { name: "Add manual balance", exact: true }).click();
  await expect(dialog.getByRole("table", { name: "Current manual records" })).toContainText("100");
  await dialog.getByRole("button", { name: "Close dialog" }).click();
  await page.getByRole("link", { name: "Holdings", exact: true }).click();
  state.items[0].quantity = "101";
  state.version++;
  await page.getByRole("button", { name: "Edit manual USD", exact: true }).click();
  await expect(dialog.getByRole("textbox", { name: /^Quantity/ })).toHaveValue("101");
  state.items[0].quantity = "103";
  state.version++;
  state.conflict = true;
  await dialog.getByRole("textbox", { name: /^Quantity/ }).fill("102");
  await dialog.getByRole("button", { name: "Save quantity", exact: true }).click();
  await expect(dialog.getByRole("alert")).toContainText("changed elsewhere");
  await expect(dialog.getByRole("textbox", { name: /^Quantity/ })).toHaveValue("102");
  await dialog.getByRole("button", { name: "Reload current records" }).click();
  await expect(dialog.getByRole("textbox", { name: /^Quantity/ })).toHaveValue("103");
  await dialog.getByRole("button", { name: "Close dialog" }).click();
  await page.getByRole("button", { name: "Delete manual USD", exact: true }).click();
  await dialog.getByRole("button", { name: "Delete record", exact: true }).click();
  await dialog.getByRole("button", { name: "Close dialog" }).click();
  await expect(page.getByRole("table", { name: "Pending manual quantities" })).toContainText("100");
  await expect(
    page.getByRole("table", { name: "Balances at the latest Valuation Point" }),
  ).toContainText("100");
});

test("global entry works before a point and requires Identity selection on mobile", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const state = await fixture(page, false);
  await page.goto(`/p/${portfolioId}/holdings/balances`);
  await page.getByRole("button", { name: "Add manual balance", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Manual balances" });
  await expect(dialog.getByRole("combobox", { name: "Identity", exact: true })).toHaveValue("");
  await expect(dialog.getByRole("button", { name: "Save quantity", exact: true })).toHaveCount(0);
  await dialog.getByRole("combobox", { name: "Identity", exact: true }).selectOption("owner");
  await dialog.getByRole("combobox", { name: "Asset code" }).fill("USD");
  await dialog.getByRole("textbox", { name: /^Quantity/ }).fill("-100");
  await dialog.getByRole("button", { name: "Save quantity", exact: true }).click();
  await expect(dialog.getByRole("status").filter({ hasText: "Saved." })).toBeVisible();
  await dialog.getByRole("button", { name: "Close dialog" }).click();
  await expect(page.getByRole("table", { name: "Pending manual quantities" })).toContainText(
    "-100",
  );
  expect(state.privateWrites).toBe(0);
});
