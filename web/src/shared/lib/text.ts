import { formatCount } from "@/shared/lib/amount";
/** "month" -> "Month", for enum values shown as choices. */
export function capitalize(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1);
}

const REASON_TEXT: Record<string, string> = {
  no_start_point: "no point at start",
  no_end_point: "no point at end",
  no_point_in_period: "no point in period",
  point_incomplete: "point incomplete",
  asset_exclusion_changed: "asset treatment changed",
  no_point: "no point",
};

/** A server reason code as a short phrase; unknown codes read as words. */
export function reasonText(code: string): string {
  const [head = "", ...rest] = code.split(": ");
  const text = REASON_TEXT[head] ?? head.replaceAll("_", " ");
  return rest.length > 0 ? `${text}: ${rest.join(": ")}` : text;
}

/** "Chrome on macOS" from a browser User-Agent; anything else is returned as is. */
export function clientSummary(userAgent: string): string {
  const os = /iPhone|iPad/.test(userAgent)
    ? "iOS"
    : /Android/.test(userAgent)
      ? "Android"
      : /Mac OS X/.test(userAgent)
        ? "macOS"
        : /Windows/.test(userAgent)
          ? "Windows"
          : /CrOS/.test(userAgent)
            ? "ChromeOS"
            : /Linux/.test(userAgent)
              ? "Linux"
              : "";
  const browser = /Edg\//.test(userAgent)
    ? "Edge"
    : /OPR\//.test(userAgent)
      ? "Opera"
      : /Firefox\//.test(userAgent)
        ? "Firefox"
        : /Chrome\//.test(userAgent)
          ? "Chrome"
          : /Safari\//.test(userAgent)
            ? "Safari"
            : "";
  if (!browser && !os) {
    return userAgent;
  }
  return [browser, os].filter(Boolean).join(" on ");
}

/** "6", or "6 · 1 Stale · 2 Incomplete" when something is off. */
export function accountSummary(total: number, stale: number, incomplete: number): string {
  const parts = [formatCount(total)];
  if (stale > 0) parts.push(`${formatCount(stale)} Stale`);
  if (incomplete > 0) parts.push(`${formatCount(incomplete)} Incomplete`);
  return parts.join(" · ");
}

/** Shown wherever a read needs a Valuation Point and the Portfolio has none yet. */
export const NO_POINT = "No Valuation Point yet. Run a Collection Cycle to create the first point.";
