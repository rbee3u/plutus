import Decimal from "decimal.js";
import type { NullableDecimal } from "@/api/generated/types.gen";

const UNKNOWN = "—";

/** Exact decimal for a known value; null for empty, malformed, or non-finite input. */
export function parseDecimal(value: NullableDecimal | undefined): Decimal | null {
  if (value == null || value === "") {
    return null;
  }
  try {
    const next = new Decimal(value);
    return next.isFinite() ? next : null;
  } catch {
    return null;
  }
}

/** Inserts thousands separators into the integer part of a plain decimal string. */
function group(fixed: string): string {
  const sign = fixed.startsWith("-") ? "-" : "";
  const digits = sign ? fixed.slice(1) : fixed;
  const [integer = "", fraction] = digits.split(".");
  const grouped = integer.replace(/\B(?=(\d{3})+(?!\d))/g, ",");
  return `${sign}${grouped}${fraction === undefined ? "" : `.${fraction}`}`;
}

/** Exact decimal for quantities and prices; unknown values render as an em dash, never as zero. */
export function formatDecimal(value: NullableDecimal | undefined): string {
  const amount = parseDecimal(value);
  if (amount === null) {
    return value ? String(value) : UNKNOWN;
  }
  return group(amount.toSignificantDigits(20).toFixed());
}

/**
 * A USD amount rounded to cents, as `12,345.68`. An amount too small to round
 * to a cent keeps its exact digits rather than reading as zero.
 */
export function formatAmount(value: NullableDecimal | undefined): string {
  const amount = parseDecimal(value);
  if (amount === null) {
    return value ? String(value) : UNKNOWN;
  }
  const rounded = amount.toDecimalPlaces(2);
  if (rounded.isZero() && !amount.isZero()) {
    return formatDecimal(value);
  }
  return group(rounded.toFixed(2));
}

/** A USD amount with its unit, for prose and summaries. */
export function formatUsd(value: NullableDecimal | undefined): string {
  const amount = formatAmount(value);
  return amount === UNKNOWN ? amount : `${amount} USD`;
}

export function chartNumber(value: NullableDecimal | undefined): number | null {
  return parseDecimal(value)?.toNumber() ?? null;
}

/** True only for a known amount that equals zero; unknown values are not zero. */
export function isZeroDecimal(value: NullableDecimal | undefined): boolean {
  return parseDecimal(value)?.isZero() ?? false;
}

/** Boundary contribution and withdrawal at a point as one figure: "0.00 USD" or "+a / −b". */
export function formatBoundaryFlows(point: {
  boundaryContributionUsd: NullableDecimal | undefined;
  boundaryWithdrawalUsd: NullableDecimal | undefined;
}): string {
  return isZeroDecimal(point.boundaryContributionUsd) && isZeroDecimal(point.boundaryWithdrawalUsd)
    ? formatUsd(point.boundaryContributionUsd)
    : `+${formatUsd(point.boundaryContributionUsd)} / −${formatUsd(point.boundaryWithdrawalUsd)}`;
}
