import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { TableEmptyState } from "./TableEmptyState";

describe("TableEmptyState", () => {
  it("renders title, optional help, and action", () => {
    const html = renderToStaticMarkup(
      <TableEmptyState
        title="No items found"
        help="Try clearing your filters."
        action={<button type="button">Reset</button>}
      />,
    );
    expect(html).toContain("No items found");
    expect(html).toContain("Try clearing your filters.");
    expect(html).toContain("<button");
    expect(html).toContain("Reset</button>");
  });

  it("renders title only when help and action are omitted", () => {
    const html = renderToStaticMarkup(<TableEmptyState title="Loading data..." />);
    expect(html).toContain("Loading data...");
    expect(html).not.toContain("<button");
  });
});
