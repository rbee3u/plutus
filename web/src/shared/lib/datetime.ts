const DATE_TIME: Intl.DateTimeFormatOptions = {
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  hourCycle: "h23",
};

const formatters = new Map<string, Intl.DateTimeFormat>();

function formatter(timeZone: string | undefined): Intl.DateTimeFormat {
  const key = timeZone ?? "";
  let cached = formatters.get(key);
  if (!cached) {
    try {
      cached = new Intl.DateTimeFormat("en-CA", { ...DATE_TIME, timeZone });
    } catch {
      cached = new Intl.DateTimeFormat("en-CA", DATE_TIME);
    }
    formatters.set(key, cached);
  }
  return cached;
}

/** An instant as `2026-09-22 23:13` in the Portfolio timezone; unknown instants stay an em dash. */
export function formatInstant(iso: string | null | undefined, timeZone?: string): string {
  if (!iso) {
    return "—";
  }
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return iso;
  }
  return formatter(timeZone || undefined)
    .format(date)
    .replace(",", "");
}

const dateOnlyFormatters = new Map<string, Intl.DateTimeFormat>();

function dateFormatter(timeZone: string | undefined): Intl.DateTimeFormat {
  const key = timeZone ?? "";
  let cached = dateOnlyFormatters.get(key);
  if (!cached) {
    try {
      cached = new Intl.DateTimeFormat("en-CA", {
        year: "numeric",
        month: "2-digit",
        day: "2-digit",
        timeZone,
      });
    } catch {
      cached = new Intl.DateTimeFormat("en-CA", {
        year: "numeric",
        month: "2-digit",
        day: "2-digit",
      });
    }
    dateOnlyFormatters.set(key, cached);
  }
  return cached;
}

/** Human-friendly relative elapsed time: `just now`, `5m ago`, `3h ago`, `4d ago`, or short date if > 30d. */
export function formatRelativeTime(
  iso: string | null | undefined,
  now = Date.now(),
  timeZone?: string,
): string {
  if (!iso) {
    return "—";
  }
  const date = new Date(iso);
  const time = date.getTime();
  if (Number.isNaN(time)) {
    return iso;
  }
  const diffMs = now - time;
  if (diffMs < 0 || diffMs < 60_000) {
    return "just now";
  }
  const minutes = Math.floor(diffMs / 60_000);
  if (minutes < 60) {
    return `${minutes}m ago`;
  }
  const hours = Math.floor(diffMs / 3_600_000);
  if (hours < 24) {
    return `${hours}h ago`;
  }
  const days = Math.floor(diffMs / 86_400_000);
  if (days < 30) {
    return `${days}d ago`;
  }
  return dateFormatter(timeZone).format(date);
}

export function toDatetimeLocalValue(iso: string | null | undefined): string {
  if (!iso) {
    return "";
  }
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

export function fromDatetimeLocalValue(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    throw new Error("Enter a valid date and time.");
  }
  return date.toISOString();
}

export function isoFromLocalInput(value: string): string | undefined {
  if (!value) {
    return undefined;
  }
  try {
    return fromDatetimeLocalValue(value);
  } catch {
    return undefined;
  }
}

/** Extracts a valid YYYY-MM-DD date string from a date or datetime-local value. */
export function normalizeDateInput(value: string | null | undefined): string {
  if (!value) {
    return "";
  }
  const dateStr = value.includes("T") ? value.split("T")[0] : value;
  if (!/^\d{4}-\d{2}-\d{2}$/.test(dateStr)) {
    return "";
  }
  const date = new Date(`${dateStr}T00:00:00Z`);
  return !Number.isNaN(date.getTime()) &&
    date.getUTCFullYear() > 0 &&
    date.toISOString().slice(0, 10) === dateStr
    ? dateStr
    : "";
}

export type DateBoundMode = "start" | "inclusiveEnd" | "exclusiveEnd";

/**
 * Parses a date input string (YYYY-MM-DD) into an ISO string in local timezone.
 * - "start" (default): 00:00:00 local time
 * - "inclusiveEnd" (or true): 23:59:59.999999 local time for closed intervals [from, to]
 * - "exclusiveEnd": next day 00:00:00 local time (e.g. for half-open intervals [from, to))
 */
export function isoFromDateInput(
  value: string | null | undefined,
  mode: DateBoundMode | boolean = "start",
): string | undefined {
  const dateStr = normalizeDateInput(value);
  if (!dateStr) {
    return undefined;
  }
  const bound = typeof mode === "boolean" ? (mode ? "inclusiveEnd" : "start") : mode;

  const date = new Date(`${dateStr}T${bound === "inclusiveEnd" ? "23:59:59.999" : "00:00:00"}`);
  if (bound === "exclusiveEnd") {
    date.setDate(date.getDate() + 1);
  }

  if (Number.isNaN(date.getTime())) {
    return undefined;
  }
  const iso = date.toISOString();
  // Date tracks milliseconds; the inclusive query must retain database microseconds.
  return bound === "inclusiveEnd" ? iso.replace(".999Z", ".999999Z") : iso;
}
