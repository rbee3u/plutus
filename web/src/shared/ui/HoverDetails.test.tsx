import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { computePosition, HoverCard, HoverDetails } from "./HoverDetails";

describe("HoverDetails computePosition", () => {
  const windowWidth = 1024;
  const windowHeight = 768;
  const panelWidth = 300;
  const panelHeight = 150;

  it("places above by default when top space is sufficient", () => {
    const anchor = {
      left: 100,
      top: 300,
      right: 200,
      bottom: 330,
      width: 100,
      height: 30,
    };
    const pos = computePosition(
      anchor,
      panelWidth,
      panelHeight,
      "above",
      windowWidth,
      windowHeight,
    );
    expect(pos.placement).toBe("above");
    expect(pos.top).toBe(300 - panelHeight - 8);
    expect(pos.left).toBe(100);
    // Anchor center is 100 + 50 = 150. arrowOffset = 150 - 100 - 4 = 46.
    expect(pos.arrowOffset).toBe(46);
  });

  it("flips to below when preferred above has insufficient top space", () => {
    const anchor = {
      left: 100,
      top: 50,
      right: 200,
      bottom: 80,
      width: 100,
      height: 30,
    };
    const pos = computePosition(
      anchor,
      panelWidth,
      panelHeight,
      "above",
      windowWidth,
      windowHeight,
    );
    expect(pos.placement).toBe("below");
    expect(pos.top).toBe(80 + 8);
    expect(pos.left).toBe(100);
  });

  it("places right when preferred right has sufficient right space", () => {
    const anchor = {
      left: 100,
      top: 200,
      right: 250,
      bottom: 230,
      width: 150,
      height: 30,
    };
    const pos = computePosition(
      anchor,
      panelWidth,
      panelHeight,
      "right",
      windowWidth,
      windowHeight,
    );
    expect(pos.placement).toBe("right");
    expect(pos.left).toBe(250 + 8);
    // Ideal top is anchor.top - 8 = 192.
    expect(pos.top).toBe(192);
    // Anchor centerY is 200 + 15 = 215. arrowOffsetY = 215 - 192 - 4 = 19.
    expect(pos.arrowOffsetY).toBe(19);
  });

  it("cascades right -> above -> below when space is constrained", () => {
    // 1. Right constrained, top has space -> falls back to above
    const rightConstrained = {
      left: 800,
      top: 300,
      right: 950,
      bottom: 330,
      width: 150,
      height: 30,
    };
    const posAbove = computePosition(
      rightConstrained,
      panelWidth,
      panelHeight,
      "right",
      windowWidth,
      windowHeight,
    );
    expect(posAbove.placement).toBe("above");
    expect(posAbove.top).toBe(300 - panelHeight - 8);

    // 2. Both right and top constrained -> falls back to below
    const bothConstrained = {
      left: 800,
      top: 50,
      right: 950,
      bottom: 80,
      width: 150,
      height: 30,
    };
    const posBelow = computePosition(
      bothConstrained,
      panelWidth,
      panelHeight,
      "right",
      windowWidth,
      windowHeight,
    );
    expect(posBelow.placement).toBe("below");
    expect(posBelow.top).toBe(80 + 8);
  });

  it("clamps panel coordinates within viewport margins", () => {
    const anchorAtBottom = {
      left: 100,
      top: 720,
      right: 200,
      bottom: 750,
      width: 100,
      height: 30,
    };
    const posRight = computePosition(
      anchorAtBottom,
      panelWidth,
      panelHeight,
      "right",
      windowWidth,
      windowHeight,
    );
    expect(posRight.placement).toBe("right");
    // Clamped so top + panelHeight <= windowHeight - 8
    expect(posRight.top + panelHeight).toBeLessThanOrEqual(windowHeight - 8);
    expect(posRight.arrowOffsetY).toBeGreaterThanOrEqual(12);
  });

  it("positions correctly on narrow mobile viewport with actual panel dimensions", () => {
    const mobileWidth = 390;
    const mobileHeight = 844;
    // An anchor near the right side of the screen, e.g. Margin cell in a table.
    const anchor = {
      left: 320,
      top: 380,
      right: 380,
      bottom: 400,
      width: 60,
      height: 20,
    };
    const actualWidth = 120;
    const actualHeight = 36;
    const pos = computePosition(
      anchor,
      actualWidth,
      actualHeight,
      "above",
      mobileWidth,
      mobileHeight,
    );
    expect(pos.placement).toBe("above");
    // Sits 8px directly above anchor
    expect(pos.top).toBe(anchor.top - actualHeight - 8);
    // Panel right edge remains within viewport margin (390 - 8 = 382)
    expect(pos.left + actualWidth).toBeLessThanOrEqual(mobileWidth - 8);
    expect(pos.left).toBe(mobileWidth - actualWidth - 8);
    // Arrow points towards anchor center (320 + 30 = 350)
    const anchorCenterX = anchor.left + anchor.width / 2;
    expect(pos.left + pos.arrowOffset + 4).toBe(anchorCenterX);
  });
});

describe("HoverDetails and HoverCard markup", () => {
  it("renders trigger button with accessible label", () => {
    const html = renderToStaticMarkup(
      <HoverDetails label="BTC-USDT" name="Contract details for BTC-USDT">
        <div>Content</div>
      </HoverDetails>,
    );
    expect(html).toContain('aria-label="Contract details for BTC-USDT"');
    expect(html).toContain("BTC-USDT");
  });

  it("renders HoverCard with title and badges", () => {
    const html = renderToStaticMarkup(
      <HoverCard title="BTCUSDT_261103" tags={["Linear", "Delivery"]}>
        <p>Details</p>
      </HoverCard>,
    );
    expect(html).toContain("BTCUSDT_261103");
    expect(html).toContain("Linear");
    expect(html).toContain("Delivery");
    expect(html).toContain("Details");
  });
});
