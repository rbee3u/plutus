import type { HoldingsFilters } from "../hooks/useHoldingsFilters";

/** Translate the settled URL filters for all four Holdings list endpoints. */
export function holdingsListRequest(filters: Pick<HoldingsFilters, "values" | "offset">) {
  const { platform, identityId, symbol } = filters.values;
  return {
    platform: platform || undefined,
    identityId: identityId || undefined,
    q: symbol || undefined,
    offset: filters.offset,
    limit: 50,
  };
}
