import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { NumericText } from "./NumericText";
import { formatDecimal, formatUsd } from "@/shared/lib/amount";

describe("accessible zero-count notation", () => {
  it("reads the rounded value without source precision or an interactive control", () => {
    const html = renderToStaticMarkup(<NumericText>{formatUsd("-0.000000004444567")}</NumericText>);
    expect(html).toContain('aria-hidden="true">-$0.0<sub class="');
    expect(html).toContain(">8</sub>4445");
    expect(html).toContain("-$0.000000004445");
    expect(html).not.toContain("4444567");
    expect(html).not.toContain("button");
  });

  it("renders multi-digit zero counts using ASCII digits inside a subscript", () => {
    const html = renderToStaticMarkup(
      <NumericText>{formatDecimal("0.0000000000000000003537")}</NumericText>,
    );
    expect(html).toContain('aria-hidden="true">0.0<sub class="');
    expect(html).toContain(">18</sub>3537");
    expect(html).toContain("0.0000000000000000003537");
  });
});
