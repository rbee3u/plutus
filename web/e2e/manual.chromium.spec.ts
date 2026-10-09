import { mockApi } from "./support/api";
import { sessionFixture } from "./support/session";
import { expect, test, type Page } from "@playwright/test";
import type {
  AccountValuation,
  BalanceRow,
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
      JSON.stringify(state.items.map((i) => [i.platformAssetSymbol, i.quantity])) !==
      JSON.stringify(state.applied.map((i) => [i.platformAssetSymbol, i.quantity])),
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
  await mockApi(
    page,
    sessionFixture({
      user: { username: "alice" },
      portfolio: { id: portfolioId, name: "Manual QA", createdAt: at, updatedAt: at },
    }),
    async (route) => {
      const req = route.request();
      const path = new URL(req.url()).pathname;
      const method = req.method();
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
      if (path.endsWith("/platform-assets"))
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
              : (req.postDataJSON() as { platformAssetSymbol?: string; quantity: string });
          const symbol = path.endsWith("/manual-balances")
            ? (body?.platformAssetSymbol ?? "").trim().toUpperCase()
            : path.split("/").at(-1)!;
          const quantity = method === "DELETE" ? "0" : (body?.quantity ?? "0");
          state.items =
            quantity === "0"
              ? []
              : [
                  {
                    platformAssetSymbol: symbol,
                    quantity,
                    assetSymbol: symbol,
                    platformAssetId: "platform-asset",
                    modifiedAt: at,
                  },
                ];
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
                    accounts: state.applied.length || state.items.length ? [account()] : [],
                  },
                ],
              },
            ],
          },
        });
      if (path.endsWith("/balances")) {
        const items: BalanceRow[] = state.applied.map((item) => ({
          ...item,
          ...account(),
          platformAssetSymbol: item.platformAssetSymbol,
          equity: item.quantity,
          equityUsd: item.quantity,
          priceUsd: "1",
          noPrice: false,
          manualChange: !state.items.some(
            (current) => current.platformAssetSymbol === item.platformAssetSymbol,
          )
            ? "deleted"
            : state.items.find(
                  (current) => current.platformAssetSymbol === item.platformAssetSymbol,
                )?.quantity !== item.quantity
              ? "updated"
              : undefined,
          manualQuantity: state.items.find(
            (current) => current.platformAssetSymbol === item.platformAssetSymbol,
          )?.quantity,
        }));
        for (const item of state.items) {
          if (
            !state.applied.some(
              (applied) => applied.platformAssetSymbol === item.platformAssetSymbol,
            )
          ) {
            items.push({
              ...item,
              ...account(),
              platformAssetSymbol: item.platformAssetSymbol,
              equity: item.quantity,
              priceUsd: null,
              equityUsd: null,
              noPrice: false,
              freshness: "fresh",
              reportObservedAt: null,
              manualChange: "added",
              manualQuantity: item.quantity,
            });
          }
        }
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
      if (path.endsWith("/history-window"))
        return route.fulfill({ json: { context: context(), from: at, to: at } });
      if (path.endsWith("/valuation-points"))
        return route.fulfill({
          json: { context: context(), from: at, to: at, granularity: "raw", items: [] },
        });
      return route.fallback();
    },
  );
  return state;
}

test("all three manual entries share fresh editing and keep pending values separate", async ({
  page,
}) => {
  const state = await fixture(page);
  await page.goto(`/p/${portfolioId}/overview`);
  await page.getByRole("button", { name: "Add manual balance for Owner", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Manual balances" });
  await dialog.getByRole("textbox", { name: "Asset code" }).fill("USD");
  await dialog.getByRole("textbox", { name: /^Quantity/ }).fill("100");
  await dialog.getByRole("button", { name: "Save quantity", exact: true }).click();
  await expect(dialog).toBeHidden();
  expect(state.privateWrites).toBe(0);
  await page.getByRole("button", { name: "Expand Owner", exact: true }).click();
  await page
    .getByRole("button", { name: "Expand balances and positions for Manual", exact: true })
    .click();
  await page.getByRole("button", { name: "Manual USD quantity 100", exact: true }).hover();
  await expect(page.getByRole("tooltip")).toHaveText("Pending collection");
  await page.getByRole("button", { name: "Collect now", exact: true }).click();
  await page.getByRole("button", { name: "Add manual balance", exact: true }).click();
  await expect(dialog.getByRole("heading", { name: "Add asset" })).toBeVisible();
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
  await dialog.getByRole("textbox", { name: /^Quantity/ }).fill("104");
  await dialog.getByRole("button", { name: "Save quantity", exact: true }).click();
  await expect(dialog).toBeHidden();
  await page.getByRole("button", { name: "Manual USD quantity 100", exact: true }).hover();
  await expect(page.getByRole("tooltip")).toHaveText("Pending: 104 USD");
  await expect(page.getByRole("button", { name: "Edit manual USD", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Delete manual USD", exact: true }).click();
  await dialog.getByRole("button", { name: "Delete record", exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByRole("table", { name: "Balances", exact: true })).toContainText("100");
  await expect(page.getByRole("button", { name: "Edit manual USD", exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Delete manual USD", exact: true })).toHaveCount(0);
  await page.getByRole("button", { name: "Manual USD quantity 100", exact: true }).focus();
  await expect(page.getByRole("tooltip")).toHaveText("Pending removal");
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
  await dialog.getByRole("textbox", { name: "Asset code" }).fill("USD");
  await dialog.getByRole("textbox", { name: /^Quantity/ }).fill("-100");
  await dialog.getByRole("button", { name: "Save quantity", exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByRole("table", { name: "Balances", exact: true })).toContainText("-100");
  await page.getByRole("button", { name: "Manual USD quantity -100", exact: true }).click();
  await expect(page.getByRole("tooltip")).toHaveText("Pending collection");
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "Edit manual USD", exact: true }).click();
  await dialog.getByRole("textbox", { name: /^Quantity/ }).fill("-200");
  await dialog.getByRole("button", { name: "Save quantity", exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByRole("table", { name: "Balances", exact: true })).toContainText("-200");
  await page.getByRole("button", { name: "Delete manual USD", exact: true }).click();
  await dialog.getByRole("button", { name: "Delete record", exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByRole("button", { name: /^Manual USD quantity/ })).toHaveCount(0);
  expect(state.privateWrites).toBe(0);
});

test("adding recovers from a version conflict without losing the draft", async ({ page }) => {
  const state = await fixture(page, false);
  await page.goto(`/p/${portfolioId}/holdings/balances`);
  await page.getByRole("button", { name: "Add manual balance", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Manual balances" });
  await dialog.getByRole("combobox", { name: "Identity", exact: true }).selectOption("owner");
  await dialog.getByRole("textbox", { name: "Asset code" }).fill("USD");
  await dialog.getByRole("textbox", { name: /^Quantity/ }).fill("100");
  state.version++;
  await dialog.getByRole("button", { name: "Save quantity", exact: true }).click();
  await expect(dialog.getByRole("alert")).toContainText("changed elsewhere");
  await expect(dialog.getByRole("button", { name: "Save quantity", exact: true })).toBeDisabled();
  await dialog.getByRole("button", { name: "Reload current records" }).click();
  await expect(dialog.getByRole("textbox", { name: "Asset code" })).toHaveValue("USD");
  await expect(dialog.getByRole("textbox", { name: /^Quantity/ })).toHaveValue("100");
  await dialog.getByRole("button", { name: "Save quantity", exact: true }).click();
  await expect(dialog).toBeHidden();
});
