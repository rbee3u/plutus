import Decimal from "decimal.js";
import type { NullableDecimal } from "@/api";

const UNKNOWN = "—";
const SUBSCRIPT_DIGITS = "₀₁₂₃₄₅₆₇₈₉";
const ROUNDING = Decimal.ROUND_HALF_UP;
// Keep percentage scaling exact for DECIMAL(65,30), without changing global arithmetic.
const DisplayDecimal = Decimal.clone({ precision: 100, rounding: ROUNDING });

export type NumberKind = "usd" | "quantity" | "price" | "ratio" | "percent" | "leverage" | "count";
type NumberInput = NullableDecimal | number | undefined;

export interface NumberPresentation {
  text: string;
  /** The same rounded value in ordinary decimal notation, for assistive technology. */
  plainText: string;
}

/** Exact decimal for a known value; null for empty, malformed, or non-finite input. */
export function parseDecimal(value: NumberInput): Decimal | null {
  if (value == null || value === "") return null;
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
  const [integer = "", fraction] = (sign ? fixed.slice(1) : fixed).split(".");
  return `${sign}${integer.replace(/\B(?=(\d{3})+(?!\d))/g, ",")}${fraction === undefined ? "" : `.${fraction}`}`;
}

function round(amount: Decimal, kind: NumberKind): Decimal {
  if (kind === "count") return amount;
  const significant = kind === "quantity" || kind === "price" ? 6 : 4;
  if (amount.abs().lt(1)) return amount.toSignificantDigits(significant, ROUNDING);
  const places =
    kind === "quantity" || kind === "price" || kind === "ratio"
      ? Math.max(2, significant - (amount.e + 1))
      : 2;
  return amount.toDecimalPlaces(places, ROUNDING);
}

/** Presentation only: source values, aggregation, input, and ordering stay exact. */
export function presentNumber(value: NumberInput, kind: NumberKind): NumberPresentation {
  const source = parseDecimal(value);
  if (source === null || (kind === "count" && !source.isInteger())) {
    return { text: UNKNOWN, plainText: UNKNOWN };
  }
  const amount = kind === "percent" ? new DisplayDecimal(source).times(100) : source;
  const rounded = round(amount, kind);
  const fixed = rounded.isZero() ? "0" : rounded.toFixed();
  const suffix = kind === "percent" ? "%" : kind === "leverage" ? "x" : "";
  const plainText = `${group(fixed)}${suffix}`;
  const text = plainText.replace(
    /^(-?)0\.(0{4,})([1-9]\d*)/,
    (_match, sign: string, zeros: string, digits: string) => {
      const count = String(zeros.length).replace(/\d/g, (digit) => SUBSCRIPT_DIGITS[Number(digit)]);
      return `${sign}0.0${count}${digits}`;
    },
  );
  return { text, plainText };
}

/** Expands zero-count shorthand in rounded display text, never restoring source precision. */
export function plainNumberText(text: string): string {
  return text.replace(/0\.0([₀₁₂₃₄₅₆₇₈₉]+)([1-9]\d*)/g, (match, count: string, digits: string) => {
    const zeros = Number([...count].map((digit) => SUBSCRIPT_DIGITS.indexOf(digit)).join(""));
    // Presentation supports API decimals; leave unrelated or unreasonable text unchanged.
    return zeros >= 4 && zeros <= 100 ? `0.${"0".repeat(zeros)}${digits}` : match;
  });
}

export function formatDecimal(value: NumberInput): string {
  return presentNumber(value, "quantity").text;
}

export function formatPrice(value: NumberInput): string {
  return presentNumber(value, "price").text;
}

export function formatAmount(value: NumberInput): string {
  return presentNumber(value, "usd").text;
}

export function formatRatio(value: NumberInput): string {
  return presentNumber(value, "ratio").text;
}

/** A stored fractional annualized rate, displayed as a percentage. */
export function formatPercent(value: NumberInput): string {
  return presentNumber(value, "percent").text;
}

export function formatLeverage(value: NumberInput): string {
  return presentNumber(value, "leverage").text;
}

export function formatCount(value: NumberInput): string {
  return presentNumber(value, "count").text;
}

/** A USD amount with its unit, for prose and summaries. */
export function formatUsd(value: NumberInput): string {
  const amount = formatAmount(value);
  if (amount === UNKNOWN) return amount;
  return amount.startsWith("-") ? `-$${amount.slice(1)}` : `$${amount}`;
}

export function chartNumber(value: NullableDecimal | undefined): number | null {
  return parseDecimal(value)?.toNumber() ?? null;
}

/** True only for a known amount that equals zero; unknown values are not zero. */
export function isZeroDecimal(value: NullableDecimal | undefined): boolean {
  return parseDecimal(value)?.isZero() ?? false;
}

/** Boundary contribution and withdrawal at a point as one figure: "$0" or "+$a / -$b". */
export function formatBoundaryFlows(point: {
  boundaryContributionUsd: NullableDecimal | undefined;
  boundaryWithdrawalUsd: NullableDecimal | undefined;
}): string {
  return isZeroDecimal(point.boundaryContributionUsd) && isZeroDecimal(point.boundaryWithdrawalUsd)
    ? formatUsd(point.boundaryContributionUsd)
    : `+${formatUsd(point.boundaryContributionUsd)} / -${formatUsd(point.boundaryWithdrawalUsd)}`;
}
