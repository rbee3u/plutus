import { describe, expect, it } from "vitest";
import { formatLeverage } from "@/shared/lib/amount";
import { realLeverage } from "./realLeverage";

const account = {
  platform: "binance",
  accountType: "portfolio_margin",
  adjustedEquityUsd: "39349.44",
};

const screenshotNotionals = [
  "4030",
  "10460",
  "10440",
  "7499.06586",
  "9329.3704",
  "5070.6756959",
  "3978.37485",
];

function display(adjustedEquityUsd: string | null, notionals: Array<string | null> | undefined) {
  const value = realLeverage(
    { ...account, adjustedEquityUsd },
    notionals?.map((notionalUsd) => ({ notionalUsd })),
  );
  return value === null ? null : formatLeverage(value);
}

describe("realLeverage", () => {
  it("divides the screenshot positions by adjusted equity", () => {
    expect(display("39349.44", screenshotNotionals)).toBe("1.29x");
  });

  it("keeps full precision until display", () => {
    const value = realLeverage(
      account,
      screenshotNotionals.map((notionalUsd) => ({ notionalUsd })),
    );
    expect(value).toMatch(/^1\.2911870361/);
  });

  it("sums absolute notional without netting long and short positions", () => {
    expect(display("100", ["100", "100"])).toBe("2x");
  });

  it("treats a zero notional as a known value", () => {
    expect(display("100", ["0"])).toBe("0x");
  });

  it("shows zero leverage for an empty position set", () => {
    expect(display("100", [])).toBe("0x");
  });

  it("omits a report that has not loaded positions", () => {
    expect(realLeverage(account, undefined)).toBeNull();
  });

  it.each(["0", "-1", "abc", "Infinity", null])(
    "requires a positive adjusted equity: %s",
    (equity) => {
      expect(display(equity, ["100"])).toBeNull();
    },
  );

  it.each(["abc", "-1", "Infinity", null])(
    "rejects an invalid or unknown position notional: %s",
    (notional) => {
      expect(display("100", ["100", notional])).toBeNull();
    },
  );

  it("rejects an empty notional instead of treating it as zero", () => {
    expect(display("100", [""])).toBeNull();
  });

  it("uses the shared leverage precision and unit", () => {
    expect(display("100", ["129.5"])).toBe("1.3x");
    expect(display("100", ["129.49"])).toBe("1.29x");
    expect(display("1", ["12.345"])).toBe("12.35x");
    expect(display("1", ["0.85321"])).toBe("0.8532x");
    expect(display("1", ["1.30"])).toBe("1.3x");
    expect(display("1", ["0.99999"])).toBe("1x");
    expect(display("1", ["0.00001"])).toBe("0.0₄1x");
  });

  it("keeps a tiny known ratio nonzero", () => {
    expect(display("1", ["0.0000000000000000000000000000001"])).toBe("0.0₃₀1x");
  });

  it.each([
    ["binance", "portfolio_margin"],
    ["okx", "trading"],
    ["kraken", "main"],
    ["bybit", "unified"],
  ])("supports the %s %s unified scope", (platform, accountType) => {
    expect(
      realLeverage({ platform, accountType, adjustedEquityUsd: "100" }, [{ notionalUsd: "100" }]),
    ).not.toBeNull();
  });

  it.each([
    ["binance", "spot"],
    ["binance", "cross_margin"],
    ["okx", "portfolio_margin"],
    ["okx", "funding"],
    ["kraken", "coin_m"],
    ["kraken", "multi_m"],
    ["bybit", "funding"],
    ["bybit", "earn"],
  ])("rejects the %s %s scope", (platform, accountType) => {
    expect(
      realLeverage({ platform, accountType, adjustedEquityUsd: "100" }, [{ notionalUsd: "100" }]),
    ).toBeNull();
  });

  it("keeps Kraken separate-wallet main unknown", () => {
    expect(
      realLeverage({ platform: "kraken", accountType: "main", adjustedEquityUsd: null }, [
        { notionalUsd: "100" },
      ]),
    ).toBeNull();
  });
});
