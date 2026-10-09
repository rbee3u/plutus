import type { ReactNode } from "react";
import type { Completeness, Freshness } from "@/api";
import styles from "@/shared/styles/page.module.css";

export type BadgeTone = "neutral" | "good" | "warn" | "bad" | "info";

const TONE_CLASS: Record<BadgeTone, string> = {
  neutral: styles.badgeNeutral,
  good: styles.badgeGood,
  warn: styles.badgeWarn,
  bad: styles.badgeBad,
  info: styles.badgeInfo,
};

export function Badge({
  tone = "neutral",
  title,
  children,
}: {
  tone?: BadgeTone;
  title?: string;
  children: ReactNode;
}) {
  return (
    <span className={`${styles.badge} ${TONE_CLASS[tone]}`} title={title}>
      {children}
    </span>
  );
}

export function CompletenessBadge({
  value,
  silentWhenHealthy = true,
}: {
  value: Completeness | null | undefined;
  silentWhenHealthy?: boolean;
}) {
  if (!value || (silentWhenHealthy && value === "complete")) {
    return null;
  }
  return <Badge tone={value === "complete" ? "good" : "warn"}>{value}</Badge>;
}

export function FreshnessBadge({
  value,
  silentWhenHealthy = true,
}: {
  value: Freshness | null | undefined;
  silentWhenHealthy?: boolean;
}) {
  if (!value || (silentWhenHealthy && value === "fresh")) {
    return null;
  }
  return <Badge tone={value === "fresh" ? "good" : "warn"}>{value}</Badge>;
}

export function DirectionBadge({ value }: { value: string }) {
  const isContribution = value.toLowerCase() === "contribution";
  return (
    <Badge tone={isContribution ? "good" : "bad"}>
      {isContribution ? "+ Contribution" : "- Withdrawal"}
    </Badge>
  );
}

const STATUS_TONES: Record<string, BadgeTone> = {
  ok: "good",
  enabled: "good",
  succeeded: "good",
  complete: "good",
  fresh: "good",
  reported: "good",
  failed: "bad",
  invalid: "bad",
  stale: "warn",
  incomplete: "warn",
  disabled: "warn",
  missing: "warn",
  queued: "info",
  running: "info",
  calculated: "info",
  manual: "info",
  ignored: "warn",
};

/** Badge for a server status word; unknown words render neutrally. */
export function StatusBadge({
  value,
  silentWhenHealthy = false,
}: {
  value: string | null | undefined;
  silentWhenHealthy?: boolean;
}) {
  if (
    !value ||
    (silentWhenHealthy &&
      (value === "ok" || value === "succeeded" || value === "complete" || value === "fresh"))
  ) {
    return null;
  }
  return <Badge tone={STATUS_TONES[value] ?? "neutral"}>{value}</Badge>;
}

/** Renders warning/error badges only when an account has an anomaly (silent when healthy). */
export function AccountHealthBadges({
  status,
  freshness,
  completeness,
  errorClass,
  errorDetail,
}: {
  status?: string | null;
  freshness?: Freshness | null;
  completeness?: Completeness | null;
  errorClass?: string | null;
  errorDetail?: string | null;
}) {
  const isFailed = status === "failed";
  const isStale = freshness === "stale";
  const isIncomplete = completeness === "incomplete";

  if (!isFailed && !isStale && !isIncomplete) {
    return null;
  }

  const errorTitle = [errorClass, errorDetail].filter(Boolean).join(": ");

  return (
    <span className={styles.statusGroup}>
      {isFailed ? (
        <Badge tone="bad" title={errorTitle || "collection failed"}>
          failed
        </Badge>
      ) : null}
      {isStale ? (
        <Badge tone="warn" title={errorTitle || "stale: report carried forward"}>
          stale
        </Badge>
      ) : null}
      {isIncomplete ? (
        <Badge tone="warn" title="incomplete: some assets omitted or unpriced">
          incomplete
        </Badge>
      ) : null}
    </span>
  );
}
