import type { AccountValuation, PositionRow } from "@/api";
import { formatLeverage } from "@/shared/lib/amount";
import { parseLeverageRatio, realLeverage, supportsRealLeverage } from "./realLeverage";

/** The warning is strictly above this display ratio. */
export const HIGH_LEVERAGE_THRESHOLD = 3;

export type LeverageRisk = {
  /** Unrounded derived ratio, retained for comparisons. */
  value: string;
  /** Existing leverage display, for descriptions and tests. */
  display: string;
};

export type LeverageRiskMap = ReadonlyMap<string, LeverageRisk>;

export type LeverageRiskAccount = Pick<
  AccountValuation,
  "accountId" | "platform" | "accountType" | "adjustedEquityUsd" | "sourceReportId"
>;

export type LeverageRiskEntry = {
  identityName?: string;
  accountName: string;
  display: string;
};

/**
 * True only for a known, unrounded ratio strictly above three. Display
 * rounding never decides the warning: a raw 3.0001 can display as `3x` and
 * still warn.
 */
export function isHighRealLeverage(value: string | null | undefined): boolean {
  const parsed = parseLeverageRatio(value);
  return parsed !== null && parsed.gt(HIGH_LEVERAGE_THRESHOLD);
}

/**
 * Warning map for visible accounts at one overview point. Positions are only
 * interpreted after a successful bulk read; an account with no source report
 * stays unknown rather than becoming a zero-leverage account.
 */
export function buildLeverageRiskMap(
  accounts: ReadonlyArray<LeverageRiskAccount>,
  positionsByAccount: ReadonlyMap<string, ReadonlyArray<Pick<PositionRow, "notionalUsd">>>,
  positionsLoaded: boolean,
): LeverageRiskMap {
  const risks = new Map<string, LeverageRisk>();
  if (!positionsLoaded) return risks;
  for (const account of accounts) {
    if (!supportsRealLeverage(account)) {
      continue;
    }
    if (account.sourceReportId == null) {
      continue;
    }
    const positions = positionsByAccount.get(account.accountId) ?? [];
    const value = realLeverage(account, positions);
    if (value !== null && isHighRealLeverage(value)) {
      risks.set(account.accountId, { value, display: formatLeverage(value) });
    }
  }
  return risks;
}

/** Parent descriptions list affected accounts rather than inventing a parent ratio. */
export function highLeverageDescription(
  level: "account" | "identity" | "platform",
  entries: ReadonlyArray<LeverageRiskEntry>,
): string {
  const first = entries[0];
  if (!first) return "";
  if (level === "account") {
    return first.display === "3x"
      ? "Real leverage is above 3x. Add margin to reduce risk."
      : `Real leverage ${first.display} is above 3x. Add margin to reduce risk.`;
  }
  const labels = entries.map((entry) => {
    const account =
      level === "platform" && entry.identityName
        ? `${entry.identityName} / ${entry.accountName}`
        : entry.accountName;
    return `${account} (${entry.display === "3x" ? "above 3x" : entry.display})`;
  });
  return `High leverage in ${labels.join(", ")}. Add margin to reduce risk.`;
}
