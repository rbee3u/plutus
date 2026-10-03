import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { NumericText } from "./NumericText";
import { formatUsd } from "@/shared/lib/amount";

describe("accessible zero-count notation", () => {
  it("reads the rounded value without source precision or an interactive control", () => {
    const html = renderToStaticMarkup(<NumericText>{formatUsd("-0.000000004444567")}</NumericText>);
    expect(html).toContain('aria-hidden="true">-0.0₈4445 USD');
    expect(html).toContain("-0.000000004445 USD");
    expect(html).not.toContain("4444567");
    expect(html).not.toContain("button");
  });
});
