import { expect, test, type Page } from "@playwright/test";
import type { CapitalFlow } from "../src/api";
import { mockApi } from "./support/api";
import { sessionFixture } from "./support/session";

async function fixture(page: Page) {
  const session = sessionFixture();
  const portfolioId = session.portfolios.items[0].id;
  const initial: CapitalFlow = {
    id: "00000000-0000-7000-8000-000000000003",
    portfolioId,
    direction: "contribution",
    canonicalAssetSymbol: "USD",
    quantity: "100",
    valueUsd: "100",
    effectiveAt: "2026-10-03T00:00:00Z",
    version: 1,
    createdAt: "2026-10-03T00:00:00Z",
    updatedAt: "2026-10-03T00:00:00Z",
  };
  const state = { flow: initial as CapitalFlow | null, writes: [] as string[], failRead: false };
  await mockApi(page, session, async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (
      path.endsWith("/identities") ||
      path.endsWith("/boundary-events") ||
      path.endsWith("/platforms")
    )
      return route.fulfill({ json: { items: [] } });
    if (path.endsWith("/capital-flows"))
      return route.fulfill({ json: { items: state.flow ? [state.flow] : [] } });
    if (path.endsWith(`/capital-flows/${initial.id}`)) {
      if (request.method() === "GET" && state.failRead)
        return route.fulfill({ status: 503, json: { detail: "Read unavailable" } });
      if (!state.flow) return route.fulfill({ status: 404, json: { detail: "Removed" } });
      if (request.method() !== "GET") {
        state.writes.push(request.headers()["if-match"]);
        if (request.headers()["if-match"] !== `"capital-flow:${state.flow.version}"`)
          return route.fulfill({ status: 412, json: { detail: "Version conflict" } });
        if (request.method() === "DELETE") {
          state.flow = null;
          return route.fulfill({ status: 204 });
        }
        const body = request.postDataJSON() as Partial<CapitalFlow>;
        state.flow = { ...state.flow, ...body, version: state.flow.version + 1 };
      }
      return route.fulfill({
        json: state.flow,
        headers: { ETag: `W/"capital-flow:${state.flow.version}"` },
      });
    }
    return route.fallback();
  });
  await page.goto(`/p/${portfolioId}/capital`);
  await expect(page.getByRole("button", { name: "Edit", exact: true })).toBeVisible();
  return state;
}

test("Capital Flow edits use the JSON baseline and explicitly reload conflicts", async ({
  page,
}) => {
  const state = await fixture(page);
  await page.getByRole("button", { name: "Edit", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Quantity", { exact: true }).fill("150");
  state.flow = { ...state.flow!, quantity: "200", version: 2 };
  await dialog.getByRole("button", { name: "Save flow" }).click();
  await expect(dialog.getByRole("alert")).toContainText("changed in another session");
  await expect(dialog.getByLabel("Quantity", { exact: true })).toHaveValue("150");
  await expect(dialog.getByRole("button", { name: "Save flow" })).toBeDisabled();
  state.failRead = true;
  await dialog.getByRole("button", { name: "Reload latest flow" }).click();
  await expect(dialog.getByRole("alert")).toContainText("Read unavailable");
  await expect(dialog.getByRole("button", { name: "Save flow" })).toBeDisabled();
  await expect(dialog.getByLabel("Quantity", { exact: true })).toHaveValue("150");
  state.failRead = false;
  await dialog.getByRole("button", { name: "Reload latest flow" }).click();
  await expect(dialog.getByLabel("Quantity", { exact: true })).toHaveValue("200");
  await dialog.getByLabel("Quantity", { exact: true }).fill("250");
  await dialog.getByRole("button", { name: "Save flow" }).click();
  await expect(dialog).toBeHidden();
  expect(state.flow?.quantity).toBe("250");
  expect(state.writes).toEqual(['"capital-flow:1"', '"capital-flow:2"']);
});

test("Capital Flow deletion cannot silently advance the confirmed version", async ({ page }) => {
  const state = await fixture(page);
  await page.getByRole("button", { name: "Delete", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Delete Capital Flow" });
  await expect(dialog).toContainText("100 USD");
  state.flow = { ...state.flow!, quantity: "200", version: 2 };
  await dialog.getByRole("button", { name: "Delete", exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByRole("alert")).toContainText("Review the updated list");
  expect(state.flow?.quantity).toBe("200");
  await page.getByRole("button", { name: "Delete", exact: true }).click();
  await expect(dialog).toContainText("200 USD");
  await dialog.getByRole("button", { name: "Delete", exact: true }).click();
  await expect(dialog).toBeHidden();
  expect(state.flow).toBeNull();
  expect(state.writes).toEqual(['"capital-flow:1"', '"capital-flow:2"']);
});
