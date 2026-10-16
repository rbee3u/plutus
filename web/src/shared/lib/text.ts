import { formatCount } from "@/shared/lib/amount";
/** "month" -> "Month", for enum values shown as choices. */
export function capitalize(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1);
}

const ISSUE_LABELS: Record<string, string> = {
  stale: "Stale",
  no_price: "Missing price",
  missing: "Missing report",
};

export function issueLabel(code: string): string {
  return ISSUE_LABELS[code] ?? capitalize(reasonText(code));
}

const REASON_TEXT: Record<string, string> = {
  no_start_point: "no point at start",
  no_end_point: "no point at end",
  no_point_in_period: "no point in period",
  point_incomplete: "point incomplete",
  start_point_incomplete: "start point incomplete",
  end_point_incomplete: "end point incomplete",
  days_without_points: "days without points",
  asset_exclusion_changed: "asset treatment changed",
  asset_mapping_changed: "asset mapping changed",
  calculation_version_changed: "calculation version changed",
  manual_balance_changed: "manual balance changed",
  unvalued_flow: "unvalued flow",
  boundary_incomplete: "boundary incomplete",
  unvalued_boundary_event: "unvalued boundary event",
  no_point: "no point",
};

/** Case known reason phrases for their context without recasing source details. */
export function reasonText(code: string, context: "phrase" | "label" = "phrase"): string {
  const [head = "", ...rest] = code.split(": ");
  const known = REASON_TEXT[head];
  const text = known
    ? context === "label"
      ? capitalize(known)
      : known
    : head.replaceAll("_", " ");
  return rest.length > 0 ? `${text}: ${rest.join(": ")}` : text;
}

export function boundaryEventCountText(count: number): string {
  return count === 0
    ? "No boundary events"
    : `${formatCount(count)} boundary ${count === 1 ? "event" : "events"}`;
}

/** "Chrome on macOS" from a browser `User-Agent`; anything else is returned as is. */
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

/** "6", or "6 · 1 stale · 2 incomplete" when something is off. */
export function accountSummary(total: number, stale: number, incomplete: number): string {
  const parts = [formatCount(total)];
  if (stale > 0) parts.push(`${formatCount(stale)} stale`);
  if (incomplete > 0) parts.push(`${formatCount(incomplete)} incomplete`);
  return parts.join(" · ");
}

/** Shown wherever a read needs a valuation point and the portfolio has none yet. */
export const NO_POINT = "No valuation point yet. Run a collection cycle to create the first point.";

export const NO_ACCOUNT_REPORT =
  "No compatible successful account report is available; holdings are unknown.";
