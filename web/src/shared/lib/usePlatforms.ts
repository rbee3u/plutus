import { keys, listPlatforms, unwrapOrRedirect } from "@/api";
import type { AccountStructure, Platform } from "@/api";
import { useSessionScope } from "@/shared/lib/session";
import { useQuery } from "@tanstack/react-query";

type PlatformLabels = Pick<Platform, "id" | "name" | "accountStructures">;

// Display aliases only; API names and identifiers remain canonical.
const identityLabels: Record<
  string,
  Record<string, { name: string; marginModes?: Record<string, string> }>
> = {
  binance: {
    binance_classic: { name: "Classic" },
    binance_pm: { name: "Unified" },
  },
  okx: {
    okx_unified: {
      name: "Unified",
      marginModes: {
        okx_spot: "Spot",
        okx_futures: "Futures",
        okx_mcm: "MCM",
        okx_pm: "PM",
      },
    },
  },
  bybit: {
    bybit_uta_2_0: {
      name: "Unified",
      marginModes: {
        bybit_isolated: "ISO",
        bybit_cross: "MCM",
        bybit_pm: "PM",
      },
    },
  },
  kraken: {
    kraken_separate: { name: "Classic" },
    kraken_unified: { name: "Unified" },
  },
};

/** Supported platforms with their account structures, account types, and margin modes. */
export function usePlatforms() {
  const { userId, redirect } = useSessionScope();
  return useQuery({
    queryKey: keys.platforms(userId),
    enabled: Boolean(userId),
    staleTime: 5 * 60_000,
    queryFn: async () => unwrapOrRedirect(await listPlatforms(), redirect),
  });
}

export function platformLabel(platforms: PlatformLabels[] | undefined, id: string): string {
  return platforms?.find((item) => item.id === id)?.name ?? id;
}

function findStructure(
  platforms: PlatformLabels[] | undefined,
  platformId: string,
  structureId: string,
): AccountStructure | undefined {
  const platform = platforms?.find((item) => item.id === platformId);
  return platform?.accountStructures.find((structure) => structure.id === structureId);
}

function findLabels(platformId: string, structureId: string) {
  const structures = Object.hasOwn(identityLabels, platformId)
    ? identityLabels[platformId]
    : undefined;
  return structures && Object.hasOwn(structures, structureId) ? structures[structureId] : undefined;
}

export function structureLabel(
  platforms: PlatformLabels[] | undefined,
  platformId: string,
  structureId: string,
): string {
  return (
    findLabels(platformId, structureId)?.name ??
    findStructure(platforms, platformId, structureId)?.name ??
    structureId
  );
}

/** `Unknown` when a required mode is unavailable; null when no mode applies. */
export function marginModeLabel(
  platforms: PlatformLabels[] | undefined,
  platformId: string,
  structureId: string,
  marginMode: string | null | undefined,
): string | null {
  const labels = findLabels(platformId, structureId);
  if (labels && !labels.marginModes) return null;
  const structure = findStructure(platforms, platformId, structureId);
  if (marginMode) {
    if (labels?.marginModes && Object.hasOwn(labels.marginModes, marginMode)) {
      return labels.marginModes[marginMode];
    }
    return structure?.marginModes.find((mode) => mode.id === marginMode)?.name ?? marginMode;
  }
  if (labels?.marginModes || (structure && structure.marginModes.length > 0)) {
    return "Unknown";
  }
  return null;
}
