import { describe, expect, it } from "vitest";
import { estimateSeriesStep, formatAxisNumber, withChartTheme } from "./chartTheme";

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

  it.each([
    [191400, 400, "191.4K"],
    [191000, 400, "191.0K"],
    [191050, 50, "191.05K"],
    [191000, 50, "191.00K"],
    [72150, 50, "72.15K"],
    [72100, 50, "72.10K"],
    [72000, 50, "72.00K"],
    [191575, 5, "191,575"],
    [150000, 10000, "150K"],
    [1240000, 20000, "1.24M"],
  ])("adapts precision for %s with step %s to %s", (value, step, expected) => {
    expect(formatAxisNumber(value, step)).toBe(expected);
  });

  it("estimates step correctly from series data", () => {
    expect(estimateSeriesStep([{ data: [191000, 193500] }], 5)).toBe(500);
    expect(estimateSeriesStep([{ data: [100] }])).toBeUndefined();
    expect(estimateSeriesStep([{ data: [100, 100] }])).toBeUndefined();
  });

  it("applies boundaryGap and adaptive formatter to scaled yAxis in withChartTheme", () => {
    const styled = withChartTheme({
      yAxis: { type: "value", scale: true },
      series: [{ type: "line", data: [191000, 193500] }],
    });
    const yAxis = styled.yAxis as Record<string, unknown>;
    expect(yAxis.boundaryGap).toEqual(["5%", "5%"]);
    const axisLabel = yAxis.axisLabel as { align: string; formatter: (v: number) => string };
    expect(axisLabel.align).toBe("right");
    expect(axisLabel.formatter(191400)).toBe("191.4K");
    expect(axisLabel.formatter(191000)).toBe("191.0K");
  });
});
