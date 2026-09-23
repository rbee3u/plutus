import { listPlatforms } from "@/api/generated/sdk.gen";
import type { Platform } from "@/api/generated/types.gen";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { useQuery } from "@tanstack/react-query";

/** Supported Platforms with their Account Modes and Account Types. */
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

export function accountModeLabel(
  platforms: Platform[] | undefined,
  platformId: string,
  modeId: string,
): string {
  const platform = platforms?.find((item) => item.id === platformId);
  return platform?.accountModes.find((mode) => mode.id === modeId)?.name ?? modeId;
}
