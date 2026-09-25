import { describe, expect, it } from "vitest";
import { formatAmount, formatBoundaryFlows, formatUsd, isZeroDecimal } from "./amount";

describe("amount display", () => {
  it("keeps unknown values distinct from known zero", () => {
    expect(formatAmount(null)).toBe("—");
    expect(formatUsd(undefined)).toBe("—");
    expect(isZeroDecimal(null)).toBe(false);
    expect(formatAmount("0")).toBe("0.00");
    expect(isZeroDecimal("0")).toBe(true);
  });

  it("preserves tiny nonzero amounts that would round to zero cents", () => {
    expect(formatAmount("0.0000001")).toBe("0.0000001");
    expect(formatAmount("-0.0000001")).toBe("-0.0000001");
  });

  it("formats known boundary flows and rounds normal USD amounts", () => {
    expect(
      formatBoundaryFlows({ boundaryContributionUsd: "12345.678", boundaryWithdrawalUsd: "5.67" }),
    ).toBe("+12,345.68 USD / −5.67 USD");
    expect(formatBoundaryFlows({ boundaryContributionUsd: "0", boundaryWithdrawalUsd: "0" })).toBe(
      "0.00 USD",
    );
  });
});
