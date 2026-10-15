import type { HoldingsTree, Overview } from "../../src/api/generated/types.gen";

/** A single-platform ownership tree for the existing summary/scroll fixtures. */
export function overviewTreeFixture(overview: Overview): HoldingsTree {
  const group = overview.byPlatform[0];
  return {
    context: overview.context,
    totalUsd: overview.totalUsd ?? null,
    completeness: overview.completeness,
    issues: overview.issues,
    platforms: overview.point
      ? [
          {
            platform: group?.key ?? "binance",
            label: group?.label ?? "Binance",
            valueUsd: overview.totalUsd ?? null,
            completeness: overview.completeness,
            identities: overview.byIdentity.map((identity) => ({
              id: identity.key,
              displayName: identity.label,
              status: "enabled",
              connectionStatus: "enabled",
              valueUsd: identity.valueUsd,
              completeness: identity.completeness,
              accounts: [],
            })),
          },
        ]
      : [],
  };
}
