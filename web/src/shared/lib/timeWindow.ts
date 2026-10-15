import type { PeriodKind } from "@/api";

export const TIME_WINDOW_PRESETS = ["1D", "3D", "1W", "2W", "1M", "3M", "6M"] as const;
export type TimeWindowPreset = (typeof TIME_WINDOW_PRESETS)[number];
export const DEFAULT_TIME_WINDOW_PRESET: TimeWindowPreset = "1W";

export const HISTORY_PRESETS = TIME_WINDOW_PRESETS;
export type HistoryPreset = TimeWindowPreset;
export const HISTORY_PAGE_SIZES = [10, 20, 50] as const;
export const PERIOD_KINDS: readonly PeriodKind[] = ["day", "week", "month", "quarter", "year"];

export function presetPeriod(preset: string): PeriodKind {
  return preset === "6M" || preset === "3M" || preset === "1Year"
    ? "month"
    : preset === "1M" || preset === "1Month"
      ? "week"
      : "day";
}

export function timeWindowGranularity(from: string, to: string) {
  const days = (Date.parse(to) - Date.parse(from)) / 86_400_000;
  return days <= 3 ? "raw" : days <= 45 ? "hour" : "day";
}
export const historyGranularity = timeWindowGranularity;

export function normalizeTimeWindowPreset(val: string | null): TimeWindowPreset | undefined {
  if (!val) return undefined;
  const map: Record<string, TimeWindowPreset> = {
    "1d": "1D",
    "1D": "1D",
    "3d": "3D",
    "3D": "3D",
    "1w": "1W",
    "1W": "1W",
    "2w": "2W",
    "2W": "2W",
    "1Month": "1M",
    "1m": "1M",
    "1M": "1M",
    "3m": "3M",
    "3M": "3M",
    "6m": "6M",
    "6M": "6M",
  };
  return map[val];
}
export const normalizePreset = normalizeTimeWindowPreset;

/** Wall-clock text only; interpretation happens on the server in the portfolio timezone. */
export function timeWindowLocalValue(iso: string, timezone: string): string {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: timezone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hourCycle: "h23",
  }).formatToParts(new Date(iso));
  const value = (type: string) => parts.find((part) => part.type === type)?.value ?? "";
  return `${value("year")}-${value("month")}-${value("day")}T${value("hour")}:${value("minute")}:${value("second")}`;
}
export const historyLocalValue = timeWindowLocalValue;
