import type {
  TreeIdentity,
  TreePlatform,
  ValuationPointSummary,
} from "../../src/api/generated/types.gen";

/** A single-platform ownership tree for summary and scrolling fixtures. */
export function overviewTreeFixture(
  point: ValuationPointSummary | null,
  identities: TreeIdentity[] = [],
  platform = "binance",
): TreePlatform[] {
  return point
    ? [
        {
          platform,
          label: platform === "okx" ? "OKX" : "Binance",
          valueUsd: point.totalUsd,
          completeness: point.completeness,
          identities,
        },
      ]
    : [];
}
