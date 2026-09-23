import { getOverview } from "@/api/generated/sdk.gen";
import type { Overview } from "@/api/generated/types.gen";
import { unwrapOrRedirect, useSessionScope } from "@/shared/lib/session";
import { useQuery } from "@tanstack/react-query";

const OVERVIEW_REFETCH_MS = 5000;

/** The latest Valuation Point summary; polls so a finished Collection Cycle shows up. */
export function useOverview(portfolioId: string) {
  const { userId, redirect } = useSessionScope();
  return useQuery({
    queryKey: ["overview", userId, portfolioId],
    enabled: Boolean(userId && portfolioId),
    refetchInterval: OVERVIEW_REFETCH_MS,
    queryFn: async (): Promise<Overview> =>
      unwrapOrRedirect(await getOverview({ path: { portfolioId } }), redirect),
  });
}
