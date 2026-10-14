import { expect, test, type Page } from "@playwright/test";
import type { Portfolio } from "../src/api";
import { mockApi } from "./support/api";
import { sessionFixture } from "./support/session";

test("Portfolio creation retains its idempotency key through retry and returns navigation to the shell", async ({
  page,
}) => {
  const session = sessionFixture();
  const original = session.portfolios.items[0];
  const created = {
    ...original,
    id: "00000000-0000-7000-8000-000000000002",
    name: "Created portfolio",
    undeletable: false,
  };
  const attemptKeys: string[] = [];
  await mockApi(page, session, async (route) => {
    const request = route.request();
    if (new URL(request.url()).pathname === "/api/v1/portfolios" && request.method() === "POST") {
      attemptKeys.push(request.headers()["idempotency-key"]);
      if (attemptKeys.length === 1)
        return route.fulfill({ status: 503, json: { detail: "Try creation again" } });
      session.portfolios.items.push(created);
      return route.fulfill({ status: 201, json: created });
    }
    return route.fallback();
  });
  await page.goto(`/p/${original.id}/settings/portfolio`);
  await page.getByTitle("Switch portfolio", { exact: true }).click();
  await page.getByRole("menuitem", { name: "New portfolio" }).click();
  const dialog = page.getByRole("dialog", { name: "New portfolio" });
  await dialog.getByLabel("Name", { exact: true }).fill(created.name);
  await dialog.getByRole("button", { name: "Create", exact: true }).click();
  await expect(dialog.getByRole("alert")).toHaveText("Try creation again");
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await page.getByTitle("Switch portfolio", { exact: true }).click();
  await page.getByRole("menuitem", { name: "New portfolio" }).click();
  await expect(dialog.getByLabel("Name", { exact: true })).toHaveValue(created.name);
  await dialog.getByRole("button", { name: "Create", exact: true }).click();
  await expect(page).toHaveURL(`/p/${created.id}/settings/portfolio`);
  await expect(page.getByTitle("Switch portfolio", { exact: true })).toContainText(created.name);
  expect(attemptKeys).toHaveLength(2);
  expect(attemptKeys[0]).toBeTruthy();
  expect(attemptKeys[1]).toBe(attemptKeys[0]);
});

test("Security preserves revoke refresh, password rejection and successful sign-out", async ({
  page,
}) => {
  const session = sessionFixture();
  let revoked = false;
  let reads = 0;
  let passwords = 0;
  await mockApi(page, session, async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (path === "/api/v1/auth/sessions" && request.method() === "GET") {
      reads++;
      const current = {
        id: "current",
        current: true,
        createdAt: "2026-10-08T00:00:00Z",
        lastSeenAt: "2026-10-08T00:00:00Z",
        idleExpiresAt: "2026-10-09T00:00:00Z",
        absoluteExpiresAt: "2026-10-10T00:00:00Z",
        userAgent: "Current browser",
      };
      return route.fulfill({
        json: {
          items: revoked
            ? [current]
            : [current, { ...current, id: "other", current: false, userAgent: "Other browser" }],
        },
      });
    }
    if (path === "/api/v1/auth/sessions/other" && request.method() === "DELETE") {
      revoked = true;
      return route.fulfill({ status: 204 });
    }
    if (path === "/api/v1/auth/password" && request.method() === "PUT") {
      passwords++;
      return passwords === 1
        ? route.fulfill({
            status: 401,
            json: {
              type: "about:blank",
              title: "Unauthorized",
              status: 401,
              code: "invalid_credentials",
              detail: "Current password is incorrect.",
            },
          })
        : route.fulfill({ status: 204 });
    }
    return route.fallback();
  });
  await page.goto("/account/security");
  const newPassword = page.getByLabel("New password", { exact: true });
  await expect(newPassword).toHaveAccessibleDescription("8+ chars A–Z a–z 0–9 / symbol");
  expect(await page.getByRole("main").ariaSnapshot()).toContain("8+ chars A–Z a–z 0–9 / symbol");
  await page.getByRole("button", { name: "Revoke", exact: true }).click();
  await expect(page.getByRole("button", { name: "Revoke", exact: true })).toHaveCount(0);
  expect(reads).toBe(2);
  await page.getByLabel(/^Current password/).fill("OldPassword=123");
  await page.getByLabel("New password", { exact: true }).fill("NewPassword=123");
  await page.getByLabel(/^Current password/).fill("NewPassword=123");
  await expect(newPassword).toHaveAccessibleDescription(
    "8+ chars A–Z a–z 0–9 / symbol New password must differ from current password",
  );
  await page.getByLabel(/^Current password/).fill("OldPassword=123");
  await page.getByLabel("Confirm new password", { exact: true }).fill("NewPassword=123");
  await page.getByRole("button", { name: "Change password", exact: true }).click();
  await expect(page.getByText("Current password is incorrect.", { exact: true })).toBeVisible();
  await expect(page).toHaveURL("/account/security");
  await page.getByLabel(/^Current password/).fill("CorrectPassword=123");
  await page.getByRole("button", { name: "Change password", exact: true }).click();
  await expect(page).toHaveURL("/login");
  await expect(page.getByText(/Password changed successfully/)).toBeVisible();
});

for (const responseEtag of ['"portfolio:99"', 'W/"portfolio:1"', undefined]) {
  test(`Portfolio writes use JSON versions with response ETag ${responseEtag ?? "absent"}`, async ({
    page,
  }) => {
    const session = sessionFixture({ portfolio: { undeletable: false } });
    const original = session.portfolios.items[0];
    const fallback = {
      ...original,
      id: "00000000-0000-7000-8000-000000000002",
      name: "default",
      undeletable: true,
    };
    session.portfolios.items.push(fallback);
    const path = `/api/v1/portfolios/${original.id}`;
    const writes: string[] = [];
    await mockApi(page, session, async (route) => {
      const request = route.request();
      const url = new URL(request.url()).pathname;
      if (url === path) {
        const current = session.portfolios.items[0];
        if (request.method() !== "GET") {
          writes.push(request.headers()["if-match"]);
          if (request.headers()["if-match"] !== `"portfolio:${current.version}"`)
            return route.fulfill({ status: 412, json: { detail: "Version conflict" } });
          if (request.method() === "DELETE") {
            session.portfolios.items.shift();
            return route.fulfill({ status: 202 });
          }
          session.portfolios.items[0] = {
            ...current,
            ...(request.postDataJSON() as Partial<Portfolio>),
            version: current.version + 1,
          };
        }
        return route.fulfill({
          json: session.portfolios.items[0],
          headers: responseEtag ? { ETag: responseEtag } : {},
        });
      }
      if (
        url.endsWith("/overview") ||
        url.endsWith("/tree") ||
        url.endsWith("/valuation-points") ||
        url.endsWith("/history-window")
      )
        return route.fulfill({ status: 503, json: { detail: "No valuation in this fixture" } });
      return route.fallback();
    });
    await page.goto(`/p/${original.id}/settings/portfolio`);
    await page.getByRole("textbox", { name: "Name", exact: true }).fill("Renamed");
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await expect(page.getByRole("status")).toContainText("Portfolio settings saved.");
    await page.getByLabel("Confirm name", { exact: true }).fill("Renamed");
    await page.getByRole("button", { name: "Delete", exact: true }).click();
    await expect(page).toHaveURL(`/p/${fallback.id}/overview`);
    expect(writes).toEqual(['"portfolio:1"', '"portfolio:2"']);
  });
}

test("Portfolio drafts keep their baseline through background reads and require reload after conflicts", async ({
  page,
  context,
}) => {
  const session = sessionFixture();
  const id = session.portfolios.items[0].id;
  const path = `/api/v1/portfolios/${id}`;
  const writes: string[] = [];
  let reads = 0;
  let failRead = false;
  async function install(target: Page) {
    await mockApi(target, session, async (route) => {
      const request = route.request();
      if (new URL(request.url()).pathname !== path) return route.fallback();
      if (request.method() === "GET") {
        reads++;
        if (failRead) return route.fulfill({ status: 503, json: { detail: "Read unavailable" } });
      } else {
        const current = session.portfolios.items[0];
        writes.push(request.headers()["if-match"]);
        if (request.headers()["if-match"] !== `"portfolio:${current.version}"`)
          return route.fulfill({ status: 412, json: { detail: "Version conflict" } });
        session.portfolios.items[0] = {
          ...current,
          ...(request.postDataJSON() as Partial<Portfolio>),
          version: current.version + 1,
        };
      }
      return route.fulfill({
        json: session.portfolios.items[0],
        headers: { ETag: `W/"portfolio:${session.portfolios.items[0].version}"` },
      });
    });
  }
  await install(page);
  const other = await context.newPage();
  await install(other);
  await page.goto(`/p/${id}/settings/portfolio`);
  await other.goto(`/p/${id}/settings/portfolio`);
  await page.getByRole("textbox", { name: "Name", exact: true }).fill("My draft");
  await other.getByRole("textbox", { name: "Name", exact: true }).fill("Other editor");
  await other.getByRole("button", { name: "Save", exact: true }).click();
  await expect(other.getByRole("status")).toContainText("Portfolio settings saved.");
  const before = reads;
  const background = page.waitForResponse(
    (response) => response.url().endsWith(path) && response.request().method() === "GET",
  );
  await page.evaluate(() => {
    window.dispatchEvent(new Event("offline"));
    window.dispatchEvent(new Event("online"));
  });
  await background;
  expect(reads).toBeGreaterThan(before);
  await expect(page.getByRole("textbox", { name: "Name", exact: true })).toHaveValue("My draft");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("changed in another session");
  await expect(page.getByRole("button", { name: "Save", exact: true })).toBeDisabled();
  failRead = true;
  await page.getByRole("button", { name: "Reload latest settings" }).click();
  await expect(page.getByRole("button", { name: "Reload latest settings" })).toBeEnabled();
  await expect(page.getByRole("textbox", { name: "Name", exact: true })).toHaveValue("My draft");
  await expect(page.getByRole("button", { name: "Save", exact: true })).toBeDisabled();
  failRead = false;
  await page.getByRole("button", { name: "Reload latest settings" }).click();
  await expect(page.getByRole("textbox", { name: "Name", exact: true })).toHaveValue(
    "Other editor",
  );
  await page.getByRole("textbox", { name: "Name", exact: true }).fill("Reviewed draft");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Portfolio settings saved.");
  expect(writes).toEqual(['"portfolio:1"', '"portfolio:1"', '"portfolio:2"']);
  expect(session.portfolios.items[0].name).toBe("Reviewed draft");
  await other.close();
});
