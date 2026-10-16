import { expect, test, type Page } from "@playwright/test";
import type { BoundaryEvent, CapitalFlow } from "../src/api";
import { mockApi } from "./support/api";
import { sessionFixture } from "./support/session";

test.use({ timezoneId: "UTC" });

async function fixture(page: Page, options: { timezone?: string; effectiveAt?: string } = {}) {
  const timezone = options.timezone ?? "UTC";
  const session = sessionFixture({ portfolio: { timezone } });
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
    effectiveAt: options.effectiveAt ?? "2026-10-10T00:00:00Z",
    version: 1,
    createdAt: "2026-10-10T00:00:00Z",
    updatedAt: "2026-10-10T00:00:00Z",
  };
  const state = {
    flow: initial as CapitalFlow | null,
    writes: [] as string[],
    updates: [] as Partial<CapitalFlow>[],
    creates: [] as Partial<CapitalFlow>[],
    failRead: false,
  };
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
            context: { portfolioId, timezone },
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
          context: { portfolioId, timezone },
          from: from.toISOString(),
          to: to.toISOString(),
        },
      });
    }
    if (path.endsWith("/capital-flows")) {
      if (request.method() === "POST") {
        const body = request.postDataJSON() as Partial<CapitalFlow>;
        state.creates.push(body);
        state.flow = { ...initial, ...body };
        return route.fulfill({ status: 201, json: state.flow });
      }
      return route.fulfill({ json: { items: state.flow ? [state.flow] : [] } });
    }
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
        state.updates.push(body);
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

test("Boundary events show newest batches first, keep account order, and respect microsecond bounds", async ({
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
  const events = [
    {
      ...event,
      cycleId: "00000000-0000-7000-8000-000000000007",
      evaluatedAt: "2026-10-13T23:59:59.999998Z",
      accountName: "Earlier batch first",
    },
    {
      ...event,
      cycleId: "00000000-0000-7000-8000-000000000007",
      evaluatedAt: "2026-10-13T23:59:59.999998Z",
      accountId: "00000000-0000-7000-8000-000000000008",
      accountName: "Earlier batch second",
    },
    event,
    {
      ...event,
      accountId: "00000000-0000-7000-8000-000000000008",
      accountName: "Final batch second",
    },
    nextDay,
  ];
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
          items: events.filter(
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
  await expect(table.locator("tbody tr")).toHaveCount(4);
  const accountNames = await table.locator("tbody tr td:nth-child(3)").allTextContents();
  expect(accountNames).toEqual([
    "Final microsecond",
    "Final batch second",
    "Earlier batch first",
    "Earlier batch second",
  ]);
  await page.reload();
  await expect(table.locator("tbody tr").first()).toContainText("Final microsecond");
});

test("Capital flow edits use the JSON baseline and explicitly reload conflicts", async ({
  page,
}) => {
  const state = await fixture(page, { timezone: "Europe/Berlin" });
  await openFlowAction(page, "Edit");
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Quantity", { exact: true }).fill("150");
  state.flow = {
    ...state.flow!,
    quantity: "200",
    version: 2,
    effectiveAt: "2026-10-11T01:30:00Z",
  };
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
  await expect(dialog.getByLabel("Effective at", { exact: true })).toHaveValue("2026-10-11T03:30");
  await dialog.getByLabel("Quantity", { exact: true }).fill("250");
  await dialog.getByRole("button", { name: "Save flow" }).click();
  await expect(dialog).toBeHidden();
  expect(state.flow?.quantity).toBe("250");
  expect(state.writes).toEqual(['"capital-flow:1"', '"capital-flow:2"']);
  expect(state.updates[0]).not.toHaveProperty("effectiveAt");
});

test("Capital flow create and edit use the portfolio timezone instead of browser UTC", async ({
  page,
}) => {
  const state = await fixture(page, {
    timezone: "Europe/Berlin",
    effectiveAt: "2026-10-11T01:30:12.123456Z",
  });
  await openFlowAction(page, "Edit");
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByLabel("Asset", { exact: true })).toHaveValue("USD");
  await expect(dialog.getByText("Timezone: Europe/Berlin", { exact: true })).toBeVisible();
  await expect(dialog.getByLabel("Effective at", { exact: true })).toHaveValue(
    "2026-10-11T03:30:12",
  );
  await dialog.getByLabel("Note", { exact: true }).fill("Reviewed");
  await dialog.getByRole("button", { name: "Save flow" }).click();
  await expect(dialog).toBeHidden();
  expect(state.updates[0]).not.toHaveProperty("effectiveAt");
  expect(state.flow?.effectiveAt).toBe("2026-10-11T01:30:12.123456Z");
  await openFlowAction(page, "Edit");
  await dialog.getByLabel("Effective at", { exact: true }).fill("2026-10-11T04:30:12");
  await dialog.getByRole("button", { name: "Save flow" }).click();
  await expect(dialog).toBeHidden();
  expect(state.flow?.effectiveAt).toBe("2026-10-11T02:30:12.000Z");
  await page.getByRole("button", { name: "Record flow", exact: true }).click();
  await dialog.getByLabel("Asset", { exact: true }).fill("USDT");
  await dialog.getByLabel("Quantity", { exact: true }).fill("10");
  await dialog.getByLabel("Value (USD, optional)", { exact: true }).fill("10");
  await dialog.getByLabel("Effective at", { exact: true }).fill("2026-10-11T07:30:12");
  await dialog.getByRole("button", { name: "Record flow", exact: true }).click();
  await expect(dialog).toBeHidden();
  expect(state.creates[0]?.effectiveAt).toBe("2026-10-11T05:30:12.000Z");
});

test("Capital flow note edits preserve fold instants and reject ambiguous replacement times", async ({
  page,
}) => {
  const instant = "2026-10-25T01:30:45.123456Z";
  const state = await fixture(page, { timezone: "Europe/Berlin", effectiveAt: instant });
  await openFlowAction(page, "Edit");
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Note", { exact: true }).fill("Keep the recorded instant");
  await dialog.getByRole("button", { name: "Save flow" }).click();
  await expect(dialog).toBeHidden();
  expect(state.updates[0]).not.toHaveProperty("effectiveAt");
  expect(state.flow?.effectiveAt).toBe(instant);
  await openFlowAction(page, "Edit");
  await dialog.getByLabel("Effective at", { exact: true }).fill("2026-10-25T02:45");
  await dialog.getByRole("button", { name: "Save flow" }).click();
  await expect(dialog.getByRole("alert")).toContainText("ambiguous in Europe/Berlin");
  expect(state.updates).toHaveLength(1);
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
