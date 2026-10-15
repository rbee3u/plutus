import { expect, test } from "@playwright/test";

test("Overlapping dialogs retain the lock until final close and restore existing styles", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/e2e/fixtures/scroll-lock.html");
  const open = page.getByRole("button", { name: "Open first dialog" });
  await expect(open).toBeVisible();
  await page.evaluate(() => {
    document.body.style.setProperty("position", "relative", "important");
    document.body.style.setProperty("padding-right", "3px");
    window.scrollTo(0, 500);
  });
  await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(500);
  const marker = page.locator("#scroll-marker");
  const top = await marker.evaluate((element) => element.getBoundingClientRect().top);
  for (let index = 0; index < 2; index += 1) {
    await open.click();
    const first = page.getByRole("dialog", { name: "First dialog", exact: true });
    await expect(first).toBeVisible();
    await expect
      .poll(() => marker.evaluate((element) => element.getBoundingClientRect().top))
      .toBe(top);
    const body = first.locator("[data-dialog-body]");
    const bounds = await body.boundingBox();
    await page.mouse.move(bounds!.x + bounds!.width / 2, bounds!.y + bounds!.height / 2);
    await page.mouse.wheel(0, 250);
    await expect.poll(() => body.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);
    await expect
      .poll(() => marker.evaluate((element) => element.getBoundingClientRect().top))
      .toBe(top);
    await first.getByRole("button", { name: "Open second dialog" }).click();
    const second = page.getByRole("dialog", { name: "Second dialog", exact: true });
    await expect(second).toBeVisible();
    await second.getByRole("button", { name: "Close dialog" }).click();
    await expect(first).toBeVisible();
    await expect.poll(() => page.evaluate(() => document.body.style.position)).toBe("fixed");
    await first.getByRole("button", { name: "Close dialog" }).click();
    await expect(open).toBeFocused();
    await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(500);
    expect(await page.evaluate(() => document.body.style.getPropertyPriority("position"))).toBe(
      "important",
    );
    expect(await page.evaluate(() => document.body.style.paddingRight)).toBe("3px");
    expect(await page.evaluate(() => document.documentElement.style.overflowY)).toBe("");
  }

  await open.click();
  await page.getByRole("button", { name: "Open second dialog" }).click();
  await page.getByRole("button", { name: "Unmount first dialog" }).click();
  await expect.poll(() => page.evaluate(() => document.body.style.position)).toBe("fixed");
  await page.getByRole("button", { name: "Unmount all dialogs" }).click();
  await expect.poll(() => page.evaluate(() => document.body.style.position)).toBe("relative");
  await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(500);
});

test("An open dialog releases and reacquires its lock across the mobile breakpoint", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/e2e/fixtures/scroll-lock.html");
  await page.getByRole("button", { name: "Open first dialog" }).click();
  await expect.poll(() => page.evaluate(() => document.body.style.position)).toBe("fixed");
  await page.setViewportSize({ width: 1024, height: 768 });
  await expect.poll(() => page.evaluate(() => document.body.style.position)).toBe("");
  await page.setViewportSize({ width: 812, height: 375 });
  await expect.poll(() => page.evaluate(() => document.body.style.position)).toBe("fixed");
  await page.getByRole("button", { name: "Close dialog" }).click();
  await expect.poll(() => page.evaluate(() => document.body.style.position)).toBe("");
  expect(await page.evaluate(() => document.documentElement.style.overflowY)).toBe("");
});
