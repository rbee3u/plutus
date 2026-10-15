import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { TableFilterHeader } from "./TableFilterHeader";

describe("TableFilterHeader", () => {
  it.each([
    ["Platform", "platform"],
    ["API key", "API key"],
  ])("uses sentence case for %s without changing abbreviations", (label, field) => {
    const html = renderToStaticMarkup(
      <TableFilterHeader
        label={label}
        display="All"
        active={false}
        open={false}
        onToggle={() => {}}
        onClose={() => {}}
        onClear={() => {}}
      />,
    );
    expect(html).toContain(`aria-label="Filter ${field}: All"`);
    expect(html).toContain(label);
    expect(html).toContain("All");
  });

  it("renders active state on trigger when filter is active", () => {
    const html = renderToStaticMarkup(
      <TableFilterHeader
        label="Platform"
        display="Binance"
        active={true}
        open={false}
        onToggle={() => {}}
        onClose={() => {}}
        onClear={() => {}}
      />,
    );
    expect(html).toContain('aria-label="Filter platform: Binance"');
    expect(html).toContain("Platform");
    expect(html).toContain("Binance");
  });

  it("renders search type when inactive", () => {
    const html = renderToStaticMarkup(
      <TableFilterHeader
        label="Asset"
        display=""
        active={false}
        open={false}
        type="search"
        onToggle={() => {}}
        onClose={() => {}}
        onClear={() => {}}
      />,
    );
    expect(html).toContain('aria-label="Search asset"');
    expect(html).toContain("Asset");
  });

  it("uses dialog popup for search type when active", () => {
    const html = renderToStaticMarkup(
      <TableFilterHeader
        label="Asset"
        display="BTC"
        active={true}
        open={false}
        type="search"
        onToggle={() => {}}
        onClose={() => {}}
        onClear={() => {}}
      />,
    );
    expect(html).toContain('aria-haspopup="dialog"');
    expect(html).toContain('aria-label="Filter asset: BTC"');
    expect(html).toContain("BTC");
  });
});
