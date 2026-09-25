import { listPlatforms } from "@/api/generated/sdk.gen";
import type { AccountStructure, Platform } from "@/api/generated/types.gen";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { useQuery } from "@tanstack/react-query";

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

export function structureLabel(
  platforms: Platform[] | undefined,
  platformId: string,
  structureId: string,
): string {
  return findStructure(platforms, platformId, structureId)?.name ?? structureId;
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
  const structure = findStructure(platforms, platformId, structureId);
  if (marginMode) {
    return structure?.marginModes.find((mode) => mode.id === marginMode)?.name ?? marginMode;
  }
  if (structure && structure.marginModes.length > 0) {
    return "margin mode unknown until validated";
  }
  return null;
}
