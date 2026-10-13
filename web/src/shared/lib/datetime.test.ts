import { describe, expect, it } from "vitest";
import { formatInstant, formatRelativeTime } from "./datetime";

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
