import { describe, expect, it } from "vitest";
import {
  formatAmount,
  formatBoundaryFlows,
  formatCount,
  formatDecimal,
  formatLeverage,
  formatPercent,
  formatPrice,
  formatRatio,
  formatUsd,
  isZeroDecimal,
  parseDecimal,
  plainNumberText,
  presentNumber,
  type NumberKind,
} from "./amount";

const kinds: NumberKind[] = ["usd", "quantity", "price", "ratio", "percent", "leverage", "count"];

describe("numeric presentation", () => {
  it.each(kinds)("keeps unknown %s distinct from zero", (kind) => {
    for (const input of [null, undefined, "", "invalid", "NaN", "Infinity"])
      expect(presentNumber(input, kind)).toEqual({ text: "—", plainText: "—" });
    const suffix = kind === "percent" ? "%" : kind === "leverage" ? "x" : "";
    expect(presentNumber("-0.000", kind).text).toBe(`0${suffix}`);
    expect(isZeroDecimal(null)).toBe(false);
    expect(isZeroDecimal("0")).toBe(true);
  });

  it.each([
    ["1.23456789", "1.23", "1.23457", "1.235"],
    ["9.999995", "10.00", "10", "10"],
    ["10.23456789", "10.23", "10.2346", "10.23"],
    ["99.99995", "100.00", "100", "100"],
    ["100.23456789", "100.23", "100.235", "100.23"],
    ["999.9995", "1,000.00", "1,000", "1,000"],
    ["1000.23456789", "1,000.23", "1,000.23", "1,000.23"],
    [
      "12345678901234567890123456789012345.678901234567890123456789012345",
      "12,345,678,901,234,567,890,123,456,789,012,345.68",
      "12,345,678,901,234,567,890,123,456,789,012,345.68",
      "12,345,678,901,234,567,890,123,456,789,012,345.68",
    ],
    ["0.123456789", "0.1235", "0.123457", "0.1235"],
    ["0.99995", "1.00", "0.99995", "1"],
    ["0.20", "0.2", "0.2", "0.2"],
  ])("selects magnitude and type for %s, preserving the sign", (input, usd, quantity, ratio) => {
    for (const sign of ["", "-"]) {
      expect(formatAmount(sign + input)).toBe(sign + usd);
      expect(formatDecimal(sign + input)).toBe(sign + quantity);
      expect(formatPrice(sign + input)).toBe(sign + quantity);
      expect(formatRatio(sign + input)).toBe(sign + ratio);
    }
  });

  it("rounds symmetric midpoints, including long inputs near the cutoff", () => {
    expect(formatAmount("1.235")).toBe("1.24");
    expect(formatAmount("-1.235")).toBe("-1.24");
    expect(formatAmount("1.234999999999999999999999999999")).toBe("1.23");
    expect(formatDecimal("0.9999995")).toBe("1");
    expect(formatDecimal("0.999999499999999999999999999999")).toBe("0.999999");
    expect(formatAmount("0.000099999")).toBe("0.0001");
    expect(formatAmount("0.0000099999")).toBe("0.0₄1");
    expect(formatDecimal("0.00009999995")).toBe("0.0001");
  });

  it.each([
    ["0.0001234567", "0.0001235", "0.000123457"],
    ["0.00001234567", "0.0₄1235", "0.0₄123457"],
    ["0.00000444", "0.0₅444", "0.0₅444"],
    ["0.000000004444567", "0.0₈4445", "0.0₈444457"],
    ["0.0000000000123456789", "0.0₁₀1235", "0.0₁₀123457"],
    ["0.000000000000000000000000000001", "0.0₂₉1", "0.0₂₉1"],
  ])("compresses all fractional leading zeros after rounding %s", (input, usd, quantity) => {
    for (const sign of ["", "-"]) {
      expect(formatAmount(sign + input)).toBe(sign + usd);
      expect(formatDecimal(sign + input)).toBe(sign + quantity);
      for (const kind of ["usd", "quantity"] as const) {
        const result = presentNumber(sign + input, kind);
        expect(plainNumberText(result.text)).toBe(result.plainText);
        expect(result.plainText).not.toBe("0");
      }
    }
  });

  it("scales percentages before precision selection, with no binary or global precision loss", () => {
    expect(formatPercent("0.123456789")).toBe("12.35%");
    expect(formatPercent("0.000123456789")).toBe("0.01235%");
    expect(formatPercent("0.00000000123456789")).toBe("0.0₆1235%");
    expect(formatPercent("0.012349999999999999999999999999")).toBe("1.23%");
    expect(formatPercent("-0.01235")).toBe("-1.24%");
    expect(formatLeverage("12.345")).toBe("12.35x");
    expect(formatLeverage("0.123456")).toBe("0.1235x");
    expect(formatRatio("0.123456")).toBe("0.1235");
  });

  it("groups counts without shortening them and leaves source decimals exact", () => {
    expect(formatCount(12345)).toBe("12,345");
    expect(formatCount("123456789012345678901234567890")).toBe(
      "123,456,789,012,345,678,901,234,567,890",
    );
    expect(formatCount("1.2")).toBe("—");
    expect(parseDecimal("0.12345678901234567890123456789")?.toFixed()).toBe(
      "0.12345678901234567890123456789",
    );
    expect(plainNumberText("+0.0₄1235 USD / −0.0₁₀1235 BTC")).toBe(
      "+0.00001235 USD / −0.00000000001235 BTC",
    );
    expect(formatUsd(null)).toBe("—");
    expect(formatBoundaryFlows({ boundaryContributionUsd: "0", boundaryWithdrawalUsd: "0" })).toBe(
      "0 USD",
    );
    expect(
      formatBoundaryFlows({ boundaryContributionUsd: "12345.678", boundaryWithdrawalUsd: "0.2" }),
    ).toBe("+12,345.68 USD / −0.2 USD");
  });
});
