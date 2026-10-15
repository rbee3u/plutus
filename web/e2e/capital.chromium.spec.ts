import { expect, test, type Page } from "@playwright/test";
import type { BoundaryEvent, CapitalFlow } from "../src/api";
import { mockApi } from "./support/api";
import { sessionFixture } from "./support/session";

test.use({ timezoneId: "UTC" });

async function fixture(page: Page) {
  const session = sessionFixture();
  const portfolioId = session.portfolios.items[0].id;
  const initial: CapitalFlow = {
    id: "00000000-0000-7000-8000-000000000003",
    portfolioId,
    direction: "contribution",
    pricingMethod: "explicit",
    pricingAttemptedAt: "2026-10-10T00:00:00Z",
    calculationVersion: 1,
    canonicalAssetSymbol: "USD",
    quantity: "100",
    valueUsd: "100",
    effectiveAt: "2026-10-10T00:00:00Z",
    version: 1,
    createdAt: "2026-10-10T00:00:00Z",
    updatedAt: "2026-10-10T00:00:00Z",
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
    if (path.endsWith("/history-window")) {
      const url = new URL(request.url());
      const fromLocal = url.searchParams.get("fromLocal");
      const toLocal = url.searchParams.get("toLocal");
      if (fromLocal && toLocal) {
        return route.fulfill({
          json: {
            context: { portfolioId, timezone: "UTC" },
            from: `${fromLocal}Z`,
            to: `${toLocal}Z`,
          },
        });
      }
      const preset = url.searchParams.get("preset") ?? "1W";
      const days =
        {
          "1D": 1,
          "3D": 3,
          "1W": 7,
          "2W": 14,
          "1M": 30,
          "3M": 90,
          "6M": 180,
        }[preset] ?? 7;
      const to = new Date("2026-10-14T00:00:00Z");
      const from = new Date(to.getTime() - days * 86_400_000);
      return route.fulfill({
        json: {
          context: { portfolioId, timezone: "UTC" },
          from: from.toISOString(),
          to: to.toISOString(),
        },
      });
    }
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
  await expect(page.getByRole("button", { name: /^Actions for flow/i })).toBeVisible();
  return state;
}

async function openFlowAction(page: Page, action: "Edit" | "Delete") {
  await page
    .getByRole("button", { name: /^Actions for flow/i })
    .first()
    .click();
  await page.getByRole("menuitem", { name: action, exact: true }).click();
}

test("Capital views default to 1W and expose presets", async ({ page }) => {
  await fixture(page);
  await expect(page.getByRole("button", { name: "1W", exact: true })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await page.getByRole("button", { name: "3D", exact: true }).click();
  await expect(page.getByRole("button", { name: "3D", exact: true })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await expect(page).toHaveURL(/range=3D/);
});

test("Boundary events includes the selected day's final microsecond and excludes the next day", async ({
  page,
}) => {
  const session = sessionFixture();
  const portfolioId = session.portfolios.items[0].id;
  const event: BoundaryEvent = {
    cycleId: "00000000-0000-7000-8000-000000000004",
    evaluatedAt: "2026-10-13T23:59:59.999999Z",
    accountId: "00000000-0000-7000-8000-000000000005",
    accountName: "Final microsecond",
    direction: "contribution",
    valueUsd: "1",
    completeness: "complete",
    method: "reported",
  };
  const nextDay = {
    ...event,
    cycleId: "00000000-0000-7000-8000-000000000006",
    accountName: "Next day",
    evaluatedAt: "2026-10-14T00:00:00.000000Z",
  };
  await mockApi(page, session, async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname.endsWith("/platforms")) return route.fulfill({ json: { items: [] } });
    if (url.pathname.endsWith("/history-window")) {
      return route.fulfill({
        json: {
          context: { portfolioId, timezone: "UTC" },
          from: "2026-10-13T00:00:00.000000Z",
          to: "2026-10-13T23:59:59.999999Z",
        },
      });
    }
    if (url.pathname.endsWith("/boundary-events")) {
      const to = url.searchParams.get("to") ?? "";
      // Six fractional digits retain the database's precision during this replay.
      const upper = to.replace(/\.(\d{3})Z$/, (_match, fraction: string) => `.${fraction}000Z`);
      return route.fulfill({
        json: {
          items: [event, nextDay].filter(
            (item) =>
              item.evaluatedAt <= upper && item.evaluatedAt >= "2026-10-13T00:00:00.000000Z",
          ),
        },
      });
    }
    return route.fallback();
  });
  await page.goto(
    `/p/${portfolioId}/capital?tab=events&from=2026-10-13T00:00:00Z&to=2026-10-13T23:59:59.999999Z`,
  );
  const table = page.getByRole("table", { name: "Boundary events in the window", exact: true });
  await expect(table).toContainText("Final microsecond");
  await expect(table).not.toContainText("Next day");
});

test("Capital flow edits use the JSON baseline and explicitly reload conflicts", async ({
  page,
}) => {
  const state = await fixture(page);
  await openFlowAction(page, "Edit");
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

test("Capital flow deletion cannot silently advance the confirmed version", async ({ page }) => {
  const state = await fixture(page);
  await openFlowAction(page, "Delete");
  const dialog = page.getByRole("dialog", { name: "Delete capital flow" });
  await expect(dialog).toContainText("100 USD");
  state.flow = { ...state.flow!, quantity: "200", version: 2 };
  await dialog.getByRole("button", { name: "Delete flow", exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByRole("alert")).toContainText("Review the updated list");
  expect(state.flow?.quantity).toBe("200");
  await openFlowAction(page, "Delete");
  await expect(dialog).toContainText("200 USD");
  await dialog.getByRole("button", { name: "Delete flow", exact: true }).click();
  await expect(dialog).toBeHidden();
  expect(state.flow).toBeNull();
  expect(state.writes).toEqual(['"capital-flow:1"', '"capital-flow:2"']);
});
