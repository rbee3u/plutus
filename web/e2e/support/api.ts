import type { Page, Route } from "@playwright/test";
import type { SessionFixture } from "./session";

/** Scenario routes take precedence and fall back to the common session reads. */
export async function mockApi(
  page: Page,
  session: SessionFixture,
  scenario: (route: Route) => Promise<void>,
) {
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (request.method() === "GET") {
      if (path === "/api/v1/auth/me") return route.fulfill({ json: session.currentUser });
      if (path === "/api/v1/auth/csrf") return route.fulfill({ json: session.csrf });
      if (path === "/api/v1/portfolios") return route.fulfill({ json: session.portfolios });
      const portfolio = session.portfolios.items.find(
        (item) => path === `/api/v1/portfolios/${item.id}`,
      );
      if (portfolio) return route.fulfill({ json: portfolio });
    }
    throw new Error(`Unexpected API request: ${request.method()} ${path}`);
  });
  // Playwright tries the most recently registered matching route first.
  await page.route("**/api/v1/**", scenario);
}
