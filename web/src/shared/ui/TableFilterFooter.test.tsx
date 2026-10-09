import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { TableFilterFooter } from "./TableFilterFooter";

describe("TableFilterFooter", () => {
  it("renders disabled Clear filters button when hasFilters is false", () => {
    const html = renderToStaticMarkup(<TableFilterFooter hasFilters={false} onReset={() => {}} />);
    expect(html).toContain("Clear filters");
    expect(html).toContain("disabled");
  });

  it("renders enabled Clear filters button when hasFilters is true", () => {
    const html = renderToStaticMarkup(<TableFilterFooter hasFilters={true} onReset={() => {}} />);
    expect(html).toContain("Clear filters");
    expect(html).not.toMatch(/<button[^>]*disabled[^>]*>[^<]*Clear filters/);
  });

  it("renders pager when page is provided", () => {
    const html = renderToStaticMarkup(
      <TableFilterFooter
        page={{ kind: "offset", offset: 0, limit: 50, total: 120 }}
        onPageChange={() => {}}
        hasFilters={true}
        onReset={() => {}}
      />,
    );
    expect(html).toContain("Clear filters");
    expect(html).toContain("1–50 of 120");
  });
});
