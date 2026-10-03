import { describe, expect, it } from "vitest";
import { formatAxisNumber } from "./chartTheme";

describe("numeric chart ticks", () => {
  it.each([
    [0, "0"],
    [-0, "0"],
    [1234, "1.2K"],
    [1234567, "1.2M"],
    [12.34, "12.3"],
    [0.012345, "0.01235"],
    [-0.012345, "-0.01235"],
    [0.000012345, "1.235e-5"],
    [-0.000012345, "-1.235e-5"],
    [0.000099995, "0.0001"],
    [0.99995, "1"],
    [NaN, "—"],
  ])("formats %s without losing its nonzero magnitude", (input, expected) => {
    expect(formatAxisNumber(input)).toBe(expected);
  });
});
