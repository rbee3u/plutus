import type { ReactNode } from "react";
import type { Completeness, Freshness } from "@/api/generated/types.gen";
import styles from "@/features/pages.module.css";

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

export function CompletenessBadge({ value }: { value: Completeness | null | undefined }) {
  if (!value) {
    return <span>—</span>;
  }
  return <Badge tone={value === "complete" ? "good" : "warn"}>{value}</Badge>;
}

export function FreshnessBadge({ value }: { value: Freshness | null | undefined }) {
  if (!value) {
    return <span>—</span>;
  }
  return <Badge tone={value === "fresh" ? "good" : "warn"}>{value}</Badge>;
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
};

/** Badge for a server status word; unknown words render neutrally. */
export function StatusBadge({ value }: { value: string | null | undefined }) {
  if (!value) {
    return <span>—</span>;
  }
  return <Badge tone={STATUS_TONES[value] ?? "neutral"}>{value}</Badge>;
}
