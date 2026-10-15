import { getHoldingsTree, getOverview, keys, unwrapOrRedirect } from "@/api";
import type { HoldingsTree, Overview } from "@/api";
import { useSessionScope } from "@/shared/lib/session";
import { useQuery } from "@tanstack/react-query";

/** Publish the summary and ownership tree only when they describe the same point. */
export function useOverview(portfolioId: string) {
  const { userId, redirect } = useSessionScope();
  return useQuery({
    queryKey: keys.overview(userId, portfolioId),
    enabled: Boolean(userId && portfolioId),
    refetchOnReconnect: false,
    queryFn: async (): Promise<Overview & { tree: HoldingsTree }> => {
      const [overview, tree] = await Promise.all([
        getOverview({ path: { portfolioId } }).then((result) => unwrapOrRedirect(result, redirect)),
        getHoldingsTree({ path: { portfolioId } }).then((result) =>
          unwrapOrRedirect(result, redirect),
        ),
      ]);
      if (
        (overview.context.cycleId ?? null) !== (tree.context.cycleId ?? null) ||
        (overview.context.valuationRevision ?? null) !== (tree.context.valuationRevision ?? null) ||
        (overview.context.cycleId &&
          (overview.totalUsd !== tree.totalUsd || overview.completeness !== tree.completeness))
      ) {
        throw new Error(
          "Overview and holdings changed while loading. Retry to read a consistent valuation point.",
        );
      }
      return { ...overview, tree };
    },
  });
}
