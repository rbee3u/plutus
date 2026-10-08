import { expect, test } from "@playwright/test";
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
        clientLabel: "Current browser",
      };
      return route.fulfill({
        json: {
          items: revoked
            ? [current]
            : [current, { ...current, id: "other", current: false, clientLabel: "Other browser" }],
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
  await page.getByRole("button", { name: "Revoke", exact: true }).click();
  await expect(page.getByRole("button", { name: "Revoke", exact: true })).toHaveCount(0);
  expect(reads).toBe(2);
  await page.getByLabel(/^Current password/).fill("OldPassword=123");
  await page.getByLabel("New password", { exact: true }).fill("NewPassword=123");
  await page.getByLabel("Confirm new password", { exact: true }).fill("NewPassword=123");
  await page.getByRole("button", { name: "Change password", exact: true }).click();
  await expect(page.getByText("Current password is incorrect.", { exact: true })).toBeVisible();
  await expect(page).toHaveURL("/account/security");
  await page.getByLabel(/^Current password/).fill("CorrectPassword=123");
  await page.getByRole("button", { name: "Change password", exact: true }).click();
  await expect(page).toHaveURL("/login");
  await expect(page.getByText(/Password changed successfully/)).toBeVisible();
});
