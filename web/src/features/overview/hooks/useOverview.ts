import { getOverview, keys } from "@/api";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";

/** Read the summary and ownership tree as one server snapshot. */
export function useOverview(portfolioId: string) {
  return usePortfolioQuery({
    portfolioId,
    refetchOnReconnect: false,
    key: (userId, portfolioId) => keys.overview(userId, portfolioId),
    load: (portfolioId) => getOverview({ path: { portfolioId } }),
  });
}
