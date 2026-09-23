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
