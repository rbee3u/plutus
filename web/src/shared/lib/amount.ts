import Decimal from "decimal.js";
import type { AmountStatus, NullableDecimal } from "@/api/generated/types.gen";

export function formatDecimal(value: NullableDecimal | undefined): string {
  if (value == null || value === "") {
    return "—";
  }
  try {
    return new Decimal(value).toSignificantDigits(20).toFixed();
  } catch {
    return value;
  }
}

export function chartNumber(value: NullableDecimal | undefined): number | null {
  if (value == null || value === "") {
    return null;
  }
  try {
    const next = new Decimal(value);
    if (!next.isFinite()) {
      return null;
    }
    return next.toNumber();
  } catch {
    return null;
  }
}

export function amountLabel(status: AmountStatus, unit?: string): string {
  const amount = formatDecimal(status.amount);
  if (amount === "—") {
    return "Unavailable";
  }
  return unit ? `${amount} ${unit}` : amount;
}
