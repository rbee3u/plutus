import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { NumberedPager, pageWindow } from "./Pager";

describe("pageWindow", () => {
  it("shows every page when seven or fewer pages exist", () => {
    expect(pageWindow(7, 4)).toEqual([1, 2, 3, 4, 5, 6, 7]);
  });

  it("keeps the first, last, and two neighbours around the current page", () => {
    expect(pageWindow(29, 7)).toEqual([1, "ellipsis", 5, 6, 7, 8, 9, "ellipsis", 29]);
  });

  it("fills a single skipped page instead of drawing an ellipsis", () => {
    expect(pageWindow(29, 2)).toEqual([1, 2, 3, 4, "ellipsis", 29]);
    expect(pageWindow(8, 4)).toEqual([1, 2, 3, 4, 5, 6, 7, 8]);
  });

  it("keeps the last window when the current page is out of range", () => {
    expect(pageWindow(29, 46)).toEqual([1, "ellipsis", 27, 28, 29]);
  });

  it("returns no page numbers for an empty result", () => {
    expect(pageWindow(0, 1)).toEqual([]);
  });
});

describe("NumberedPager", () => {
  it("marks the current page and names both arrows", () => {
    const html = renderToStaticMarkup(
      <NumberedPager page={7} totalPages={29} onChange={() => {}} label="Raw valuation points" />,
    );
    expect(html).toContain('aria-label="Raw valuation points pages"');
    expect(html).toContain('aria-current="page"');
    expect(html).toContain('aria-label="Page 7"');
    expect(html).toContain('aria-label="Previous page"');
    expect(html).toContain('aria-label="Next page"');
    expect(html).toContain("…");
  });

  it("renders only disabled arrows when there are no rows", () => {
    const html = renderToStaticMarkup(
      <NumberedPager page={1} totalPages={0} onChange={() => {}} />,
    );
    expect(html).not.toContain('aria-label="Page 1"');
    expect(html).toContain('aria-label="Previous page"');
    expect(html).toContain('aria-label="Next page"');
    expect(html.match(/disabled/g)).toHaveLength(2);
  });
});
