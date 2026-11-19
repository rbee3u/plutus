import Decimal from "decimal.js";
import type { AccountValuation, PositionRow } from "@/api";

/** Ratios keep far more digits than the two decimal places the chip displays. */
const Ratio = Decimal.clone({ precision: 100 });

/** Unified account scopes whose account report supplies an adjusted equity. */
const UNIFIED_ACCOUNT_TYPES = new Map<string, ReadonlySet<string>>([
  ["binance", new Set(["portfolio_margin"])],
  ["okx", new Set(["trading"])],
  ["bybit", new Set(["unified"])],
  ["kraken", new Set(["main"])],
]);

type LeverageAccount = Pick<AccountValuation, "platform" | "accountType" | "adjustedEquityUsd">;
type LeveragePosition = Pick<PositionRow, "notionalUsd">;

/** Whether this platform/account scope has the adjusted-equity basis for real leverage. */
export function supportsRealLeverage(
  account: Pick<AccountValuation, "platform" | "accountType">,
): boolean {
  return UNIFIED_ACCOUNT_TYPES.get(account.platform)?.has(account.accountType) ?? false;
}

/**
 * Display-only gross leverage for one supported unified account report: the
 * absolute USD notional of every open position divided by adjusted equity.
 * Binance portfolio margin, OKX unified trading, Kraken unified main, and Bybit
 * unified are supported; Kraken separate-wallet main has no adjusted equity.
 * It returns null when the account is out of scope, positions have not loaded,
 * any position notional is unknown, or adjusted equity is not positive, so the
 * caller can omit the measure.
 */
export function realLeverage(
  account: LeverageAccount,
  positions: ReadonlyArray<LeveragePosition> | undefined,
): string | null {
  if (!supportsRealLeverage(account)) return null;
  if (positions === undefined) return null;
  const equity = parseLeverageRatio(account.adjustedEquityUsd);
  if (equity === null || equity.lte(0)) return null;
  let notional = new Ratio(0);
  for (const position of positions) {
    const value = parseLeverageRatio(position.notionalUsd);
    if (value === null || value.lt(0)) return null;
    notional = notional.plus(value);
  }
  return notional.div(equity).toSignificantDigits(30).toFixed();
}

/** Parse a ratio string without losing digits before the display compares it. */
export function parseLeverageRatio(value: string | null | undefined): Decimal | null {
  if (value == null || value === "") return null;
  try {
    const parsed = new Ratio(value);
    return parsed.isFinite() ? parsed : null;
  } catch {
    return null;
  }
}
