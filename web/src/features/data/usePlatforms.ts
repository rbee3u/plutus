import { listPlatforms } from "@/api/generated/sdk.gen";
import type { AccountStructure, Platform } from "@/api/generated/types.gen";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { useQuery } from "@tanstack/react-query";

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
        okx_spot: "SPOT",
        okx_futures: "FUTURES",
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

/** Supported Platforms with their Account Structures, Account Types, and Margin Modes. */
export function usePlatforms() {
  const { userId, redirect } = useSessionScope();
  return useQuery({
    queryKey: ["platforms", userId],
    enabled: Boolean(userId),
    staleTime: 5 * 60_000,
    queryFn: async () => unwrapOrRedirect(await listPlatforms(), redirect),
  });
}

export function platformLabel(platforms: Platform[] | undefined, id: string): string {
  return platforms?.find((item) => item.id === id)?.name ?? id;
}

function findStructure(
  platforms: Platform[] | undefined,
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
  platforms: Platform[] | undefined,
  platformId: string,
  structureId: string,
): string {
  return (
    findLabels(platformId, structureId)?.name ??
    findStructure(platforms, platformId, structureId)?.name ??
    structureId
  );
}

/**
 * The Margin Mode line of an Identity: the mode the Platform last reported, a notice while a
 * structure that has Margin Modes has not been validated yet, or null for structures without one.
 */
export function marginModeLabel(
  platforms: Platform[] | undefined,
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
    return "UNKNOWN";
  }
  return null;
}
