import { describe, expect, it } from "vitest";
import {
  formatInstant,
  formatRelativeTime,
  isoFromDateInput,
  normalizeDateInput,
} from "./datetime";

describe("formatRelativeTime", () => {
  const baseTime = new Date("2026-10-13T12:00:00Z").getTime();

  it("returns dash for empty/null values", () => {
    expect(formatRelativeTime(null)).toBe("—");
    expect(formatRelativeTime(undefined)).toBe("—");
    expect(formatRelativeTime("")).toBe("—");
  });

  it("handles just now for recent seconds under 60s", () => {
    expect(formatRelativeTime("2026-10-13T11:59:30Z", baseTime)).toBe("just now");
    expect(formatRelativeTime("2026-10-13T11:59:10Z", baseTime)).toBe("just now");
    expect(formatRelativeTime("2026-10-13T12:00:05Z", baseTime)).toBe("just now");
  });

  it("formats minutes ago", () => {
    expect(formatRelativeTime("2026-10-13T11:59:00Z", baseTime)).toBe("1m ago");
    expect(formatRelativeTime("2026-10-13T11:55:00Z", baseTime)).toBe("5m ago");
    expect(formatRelativeTime("2026-10-13T11:01:00Z", baseTime)).toBe("59m ago");
  });

  it("formats hours ago", () => {
    expect(formatRelativeTime("2026-10-13T10:00:00Z", baseTime)).toBe("2h ago");
    expect(formatRelativeTime("2026-10-12T13:00:00Z", baseTime)).toBe("23h ago");
  });

  it("formats days ago", () => {
    expect(formatRelativeTime("2026-10-11T12:00:00Z", baseTime)).toBe("2d ago");
    expect(formatRelativeTime("2026-09-20T12:00:00Z", baseTime)).toBe("23d ago");
  });

  it("falls back to date in specified timezone for > 30 days", () => {
    expect(formatRelativeTime("2026-08-01T12:00:00Z", baseTime, "UTC")).toBe("2026-08-01");
    // 2026-08-01T23:00:00Z is 2026-08-02 in Asia/Tokyo
    expect(formatRelativeTime("2026-08-01T23:00:00Z", baseTime, "Asia/Tokyo")).toBe("2026-08-02");
  });

  it("returns invalid date input directly", () => {
    expect(formatRelativeTime("not-a-date")).toBe("not-a-date");
  });
});

describe("formatInstant", () => {
  it("formats valid ISO timestamp", () => {
    const formatted = formatInstant("2026-10-13T12:34:00Z", "UTC");
    expect(formatted).toBe("2026-10-13 12:34");
  });

  it("returns dash for empty ISO", () => {
    expect(formatInstant(null)).toBe("—");
  });
});

describe("normalizeDateInput", () => {
  it("normalizes date and datetime-local strings", () => {
    expect(normalizeDateInput("2026-10-13")).toBe("2026-10-13");
    expect(normalizeDateInput("2026-10-13T12:30")).toBe("2026-10-13");
    expect(normalizeDateInput("2026-10-13T00:00:00.000Z")).toBe("2026-10-13");
  });

  it("returns empty string for invalid or empty inputs", () => {
    expect(normalizeDateInput("")).toBe("");
    expect(normalizeDateInput(null)).toBe("");
    expect(normalizeDateInput(undefined)).toBe("");
    expect(normalizeDateInput("invalid-date")).toBe("");
  });

  it.each([
    "2026-02-30",
    "2025-02-29",
    "1900-02-29",
    "2026-04-31",
    "2026-00-13",
    "2026-13-01",
    "2026-10-00",
    "0000-01-01",
  ])("rejects the nonexistent calendar date %s", (value) => {
    expect(normalizeDateInput(value)).toBe("");
    expect(normalizeDateInput(`${value}T12:30`)).toBe("");
    for (const mode of ["start", "inclusiveEnd", "exclusiveEnd"] as const) {
      expect(isoFromDateInput(value, mode)).toBeUndefined();
    }
  });

  it.each(["2024-02-29", "2000-02-29", "2026-04-30"])("accepts the calendar date %s", (value) => {
    expect(normalizeDateInput(value)).toBe(value);
  });
});

describe("isoFromDateInput", () => {
  it("includes the last database microsecond of the selected day", () => {
    const lastMillisecond = new Date("2026-10-13T23:59:59.999").toISOString();
    expect(isoFromDateInput("2026-10-13", "inclusiveEnd")).toBe(
      lastMillisecond.replace(".999Z", ".999999Z"),
    );
  });
  it("converts date string to ISO start and end of day", () => {
    const start = isoFromDateInput("2026-10-13");
    expect(start).toBeDefined();
    const end = isoFromDateInput("2026-10-13", true);
    expect(end).toBeDefined();
    expect(new Date(start!).getTime()).toBeLessThan(new Date(end!).getTime());
  });

  it("handles exclusiveEnd for half-open intervals", () => {
    const inclusive = isoFromDateInput("2026-10-13", "inclusiveEnd");
    const exclusive = isoFromDateInput("2026-10-13", "exclusiveEnd");
    expect(inclusive).toBeDefined();
    expect(exclusive).toBeDefined();
    expect(new Date(exclusive!).getTime()).toBeGreaterThan(new Date(inclusive!).getTime());
    // Exclusive end of 2026-10-13 equals start of 2026-10-14
    const nextStart = isoFromDateInput("2026-10-14", "start");
    expect(exclusive).toBe(nextStart);
  });

  it("handles legacy datetime-local string by extracting date", () => {
    const iso = isoFromDateInput("2026-10-13T12:30");
    const expected = isoFromDateInput("2026-10-13");
    expect(iso).toBe(expected);
  });

  it("returns undefined for empty input", () => {
    expect(isoFromDateInput("")).toBeUndefined();
  });
});
