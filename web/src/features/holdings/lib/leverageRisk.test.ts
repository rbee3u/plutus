import { describe, expect, it } from "vitest";
import {
  buildLeverageRiskMap,
  highLeverageDescription,
  isHighRealLeverage,
  type LeverageRiskAccount,
} from "./leverageRisk";

function account(overrides: Partial<LeverageRiskAccount> = {}): LeverageRiskAccount {
  return {
    accountId: "account",
    platform: "binance",
    accountType: "portfolio_margin",
    adjustedEquityUsd: "100",
    sourceReportId: "report",
    ...overrides,
  };
}

function positions(notionalUsd: Array<string | null>) {
  return notionalUsd.map((value) => ({ notionalUsd: value }));
}

describe("isHighRealLeverage", () => {
  it.each([
    ["3", false],
    ["3.0", false],
    ["2.999999999999999999999999999999", false],
    ["3.000000000000000000000000000001", true],
    ["3.0001", true],
    ["3.004", true],
    ["3.005", true],
    ["12.345", true],
    ["0", false],
    ["-1", false],
    ["abc", false],
    ["", false],
    [null, false],
    ["Infinity", false],
  ])("classifies %s as %s", (value, expected) => {
    expect(isHighRealLeverage(value)).toBe(expected);
  });

  it("compares the raw value rather than the rounded display", () => {
    expect(isHighRealLeverage("3.0001")).toBe(true);
    expect(isHighRealLeverage("2.9999")).toBe(false);
  });
});

describe("buildLeverageRiskMap", () => {
  it("keeps only known supported unified accounts above 3x", () => {
    const risk = buildLeverageRiskMap(
      [
        account({ accountId: "high", adjustedEquityUsd: "100" }),
        account({ accountId: "at-three", adjustedEquityUsd: "100" }),
        account({
          accountId: "spot",
          platform: "binance",
          accountType: "spot",
        }),
        account({ accountId: "no-source", sourceReportId: null }),
      ],
      new Map([
        ["high", positions(["324"])],
        ["at-three", positions(["300"])],
        ["spot", positions(["1000"])],
        ["no-source", positions(["1000"])],
      ]),
      true,
    );

    expect(risk.get("high")).toEqual({ value: "3.24", display: "3.24x" });
    expect(risk.has("at-three")).toBe(false);
    expect(risk.has("spot")).toBe(false);
    expect(risk.has("no-source")).toBe(false);
  });

  it("treats a loaded empty position set as zero, not unknown", () => {
    const risk = buildLeverageRiskMap(
      [account({ adjustedEquityUsd: "100" })],
      new Map([["account", []]]),
      true,
    );
    expect(risk.size).toBe(0);
  });

  it("returns no map until positions are loaded", () => {
    const risk = buildLeverageRiskMap(
      [account()],
      new Map([["account", positions(["324"])]]),
      false,
    );
    expect(risk.size).toBe(0);
  });

  it("rejects unknown notionals and non-positive adjusted equity", () => {
    const risk = buildLeverageRiskMap(
      [
        account({ accountId: "unknown-notional" }),
        account({ accountId: "zero-equity", adjustedEquityUsd: "0" }),
        account({ accountId: "null-equity", adjustedEquityUsd: null }),
        account({ accountId: "invalid-equity", adjustedEquityUsd: "abc" }),
      ],
      new Map([
        ["unknown-notional", positions(["100", null])],
        ["zero-equity", positions(["400"])],
        ["null-equity", positions(["400"])],
        ["invalid-equity", positions(["400"])],
      ]),
      true,
    );
    expect(risk.size).toBe(0);
  });

  it("retains full precision at the threshold", () => {
    const risk = buildLeverageRiskMap(
      [account({ adjustedEquityUsd: "100" })],
      new Map([["account", positions(["300.01"])]]),
      true,
    );
    expect(risk.get("account")?.value).toMatch(/^3\.000/);
    expect(risk.get("account")?.display).toBe("3x");
  });
});

describe("highLeverageDescription", () => {
  it("leaves no ambiguity when the display rounds to 3x", () => {
    expect(
      highLeverageDescription("account", [{ accountName: "Portfolio margin", display: "3x" }]),
    ).toBe("Real leverage is above 3x. Add margin to reduce risk.");
  });

  it("uses above-3x wording when the display rounds to 3x", () => {
    expect(
      highLeverageDescription("identity", [{ accountName: "Portfolio margin", display: "3x" }]),
    ).toBe("High leverage in Portfolio margin (above 3x). Add margin to reduce risk.");
  });

  it("lists every identity account without inventing a parent ratio", () => {
    expect(
      highLeverageDescription("identity", [
        { accountName: "Portfolio margin", display: "3.24x" },
        { accountName: "Futures", display: "4.2x" },
      ]),
    ).toBe("High leverage in Portfolio margin (3.24x), Futures (4.2x). Add margin to reduce risk.");
  });

  it("qualifies platform accounts with their identity", () => {
    expect(
      highLeverageDescription("platform", [
        {
          identityName: "main",
          accountName: "Portfolio margin",
          display: "3.24x",
        },
        { identityName: "sub", accountName: "Unified", display: "4.2x" },
      ]),
    ).toBe(
      "High leverage in main / Portfolio margin (3.24x), sub / Unified (4.2x). Add margin to reduce risk.",
    );
  });
});
