import { describe, expect, it } from "vitest";
import {
  historyGranularity,
  historyLocalValue,
  presetPeriod,
} from "@/features/history/lib/timeWindow";

describe("History ranges", () => {
  it("matches the curve granularity to the complete window", () => {
    const from = "2026-01-01T00:00:00Z";
    for (const [days, expected] of [
      [1, "raw"],
      [3, "raw"],
      [7, "hour"],
      [31, "hour"],
      [45, "hour"],
      [46, "day"],
      [365, "day"],
    ] as const) {
      expect(
        historyGranularity(from, new Date(Date.parse(from) + days * 86_400_000).toISOString()),
      ).toBe(expected);
    }
  });
  it("matches calendar periods to presets", () => {
    expect(presetPeriod("1W")).toBe("day");
    expect(presetPeriod("2W")).toBe("day");
    expect(presetPeriod("1M")).toBe("week");
    expect(presetPeriod("3M")).toBe("month");
    expect(presetPeriod("6M")).toBe("month");
  });
  it("formats inputs in the portfolio timezone rather than the browser timezone", () => {
    expect(historyLocalValue("2026-10-04T04:16:44Z", "Asia/Shanghai")).toBe("2026-10-04T12:16:44");
    expect(historyLocalValue("2026-03-08T07:30:00Z", "America/New_York")).toBe(
      "2026-03-08T03:30:00",
    );
  });
});
