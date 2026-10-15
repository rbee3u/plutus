import type { ReactNode } from "react";
import type { Completeness, Freshness } from "@/api";
import { capitalize } from "@/shared/lib/text";
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
  const label = value === "complete" ? "Complete" : "Incomplete";
  return <Badge tone={value === "complete" ? "good" : "warn"}>{label}</Badge>;
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
  const label = value === "fresh" ? "Fresh" : "Stale";
  return <Badge tone={value === "fresh" ? "good" : "warn"}>{label}</Badge>;
}

export function DirectionBadge({ value }: { value: string }) {
  const isContribution = value.toLowerCase() === "contribution";
  return (
    <Badge tone={isContribution ? "good" : "bad"}>
      {isContribution ? "+ Contribution" : "- Withdrawal"}
    </Badge>
  );
}

const STATUS_CUSTOM_LABELS: Record<string, string> = {
  ok: "OK",
};

const STATUS_TONES: Record<string, BadgeTone> = {
  enabled: "good",
  succeeded: "good",
  complete: "good",
  fresh: "good",
  reported: "good",
  holding: "good",
  active: "neutral",
  settled: "neutral",
  failed: "bad",
  invalid: "bad",
  stale: "warn",
  incomplete: "warn",
  disabled: "warn",
  missing: "warn",
  excluded: "warn",
  redeeming: "warn",
  unbonding: "warn",
  queued: "info",
  running: "info",
  calculated: "info",
  manual: "info",
  bonding: "info",
  pending: "info",
};

/** Badge for a server status word; capitalizes its first letter; unknown words render neutrally. */
export function StatusBadge({
  value,
  title,
  silentWhenHealthy = false,
}: {
  value: string | null | undefined;
  title?: string;
  silentWhenHealthy?: boolean;
}) {
  if (
    !value ||
    (silentWhenHealthy &&
      (value === "succeeded" || value === "complete" || value === "fresh" || value === "active"))
  ) {
    return null;
  }
  const normalized = value.toLowerCase();
  const label = STATUS_CUSTOM_LABELS[normalized] ?? capitalize(value);
  return (
    <Badge tone={STATUS_TONES[normalized] ?? "neutral"} title={title}>
      {label}
    </Badge>
  );
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
          Failed
        </Badge>
      ) : null}
      {isStale ? (
        <Badge tone="warn" title={errorTitle || "stale: report carried forward"}>
          Stale
        </Badge>
      ) : null}
      {isIncomplete ? (
        <Badge tone="warn" title="Some or all of the account value is unknown.">
          Incomplete
        </Badge>
      ) : null}
    </span>
  );
}
